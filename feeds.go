package couchdb

import (
	"encoding/json"
	"fmt"
	"io"
)

// DBUpdatesFeed is an iterator for the _db_updates feed.
// This feed receives an event whenever any database is created, updated
// or deleted. On each call to the Next method, the event fields are updated
// for the current event.
//
//     feed, err := client.DbUpdates(nil)
//     ...
//     for feed.Next() {
//	       fmt.Printf("changed: %s %s", feed.Event, feed.Db)
//     }
//     err = feed.Err()
//     ...
type DBUpdatesFeed struct {
	Event string      `json:"type"`    // "created" | "updated" | "deleted"
	DB    string      `json:"db_name"` // Event database name
	Seq   interface{} `json:"seq"`     // DB update sequence of the event.
	OK    bool        `json:"ok"`      // Event operation status (deprecated)

	end  bool
	err  error
	conn io.Closer
	dec  *json.Decoder
}

// DBUpdates opens the _db_updates feed.
// For the possible options, please see the CouchDB documentation.
// Pleas note that the "feed" option is currently always set to "continuous".
//
// http://docs.couchdb.org/en/latest/api/server/common.html#db-updates
func (c *Client) DBUpdates(options Options) (*DBUpdatesFeed, error) {
	newopts := options.clone()
	newopts["feed"] = "continuous"
	path, err := optpath(newopts, nil, "_db_updates")
	if err != nil {
		return nil, err
	}
	resp, err := c.request("GET", path, nil)
	if err != nil {
		return nil, err
	}
	feed := &DBUpdatesFeed{
		conn: resp.Body,
		dec:  json.NewDecoder(resp.Body),
	}
	return feed, nil
}

// Next decodes the next event in a _db_updates feed. It returns false when
// the feeds end has been reached or an error has occurred.
func (f *DBUpdatesFeed) Next() bool {
	if f.end {
		return false
	}
	f.Event, f.DB, f.Seq, f.OK = "", "", nil, false
	if f.err = f.dec.Decode(f); f.err != nil {
		if f.err == io.EOF {
			f.err = nil
		}
		f.Close()
	}
	return !f.end
}

// Err returns the last error that occurred during iteration.
func (f *DBUpdatesFeed) Err() error {
	return f.err
}

// Close terminates the connection of a feed.
func (f *DBUpdatesFeed) Close() error {
	f.end = true
	return f.conn.Close()
}

// ChangesFeed is an iterator for the _changes feed of a database.
// On each call to the Next method, the event fields are updated
// for the current event. Next is designed to be used in a for loop:
//
//     feed, err := client.Changes("db", nil)
//     ...
//     for feed.Next() {
//	       fmt.Printf("changed: %s", feed.ID)
//     }
//     err = feed.Err()
//     ...
type ChangesFeed struct {
	// DB is the database. Since all events in a _changes feed
	// belong to the same database, this field is always equivalent to the
	// database from the DB.Changes call that created the feed object
	DB *DB `json:"-"`

	// ID is the document ID of the current event.
	ID string `json:"id"`

	// Deleted is true when the event represents a deleted document.
	Deleted bool `json:"deleted"`

	// Seq is the database update sequence number of the current event.
	// This is usually a string, but may also be a number for couchdb 0.x servers.
	//
	// For poll-style feeds (feed modes "normal", "longpoll"), this is set to the
	// last_seq value sent by CouchDB after all feed rows have been read.
	Seq interface{} `json:"seq"`

	// Pending is the count of remaining items in the feed. This is set for poll-style
	// feeds (feed modes "normal", "longpoll") after the last element has been
	// processed.
	Pending int64 `json:"pending"`

	// Changes is the list of the document's leaf revisions.
	Changes []struct {
		Rev string `json:"rev"`
	} `json:"changes"`

	// The document. This is populated only if the feed option
	// "include_docs" is true.
	Doc json.RawMessage `json:"doc"`

	end    bool
	err    error
	conn   io.Closer
	parser func() error
}

// changesRow is the JSON structure of a changes feed row.
type changesRow struct {
	ID      string      `json:"id"`
	Deleted bool        `json:"deleted"`
	Seq     interface{} `json:"seq"`
	Changes []struct {
		Rev string `json:"rev"`
	} `json:"changes"`
	Doc     json.RawMessage `json:"doc"`
	LastSeq bool            `json:"last_seq"`
}

// apply sets the row as the current event of the feed.
func (d *changesRow) apply(f *ChangesFeed) error {
	f.Seq = d.Seq
	f.ID = d.ID
	f.Deleted = d.Deleted
	f.Doc = d.Doc
	f.Changes = d.Changes
	return nil
}

// reset resets the iterator outputs to zero.
func (f *ChangesFeed) reset() {
	f.ID, f.Deleted, f.Changes, f.Doc = "", false, nil, nil
}

// Changes opens the _changes feed of a database. This feed receives an event
// whenever a document is created, updated or deleted.
//
// The implementation supports both poll-style and continuous feeds.
// The default feed mode is "normal", which retrieves changes up to some point
// and then closes the feed. If you want a never-ending feed, set the "feed"
// option to "continuous":
//
//     feed, err := client.Changes("db", couchdb.Options{"feed": "continuous"})
//
// There are many other options that allow you to customize what the
// feed returns. For information on all of them, see the official CouchDB
// documentation:
//
// http://docs.couchdb.org/en/latest/api/database/changes.html#db-changes
func (db *DB) Changes(options Options) (*ChangesFeed, error) {
	path, err := optpath(options, nil, db.name, "_changes")
	if err != nil {
		return nil, err
	}
	resp, err := db.request("GET", path, nil)
	if err != nil {
		return nil, err
	}
	feed := &ChangesFeed{DB: db, conn: resp.Body}

	switch options["feed"] {
	case nil, "normal", "longpoll":
		feed.parser, err = feed.pollParser(resp.Body)
		if err != nil {
			feed.Close()
			return nil, err
		}
	case "continuous":
		feed.parser = feed.contParser(resp.Body)
	default:
		err := fmt.Errorf(`couchdb: unsupported value for option "feed": %#v`, options["feed"])
		feed.Close()
		return nil, err
	}

	return feed, nil
}

// Next decodes the next event. It returns false when the feeds end has been
// reached or an error has occurred.
func (f *ChangesFeed) Next() bool {
	if f.end {
		return false
	}
	if f.err = f.parser(); f.err != nil || f.end {
		f.Close()
	}
	return !f.end
}

// Err returns the last error that occurred during iteration.
func (f *ChangesFeed) Err() error {
	return f.err
}

// Close terminates the connection of the feed.
// If Next returns false, the feed has already been closed.
func (f *ChangesFeed) Close() error {
	f.end = true
	return f.conn.Close()
}

// ChangesRevs returns the rev list of the current result row.
func (f *ChangesFeed) ChangesRevs() []string {
	revs := make([]string, len(f.Changes))
	for i, x := range f.Changes {
		revs[i] = x.Rev
	}
	return revs
}

func (f *ChangesFeed) contParser(r io.Reader) func() error {
	dec := json.NewDecoder(r)
	return func() error {
		var row changesRow
		if err := dec.Decode(&row); err != nil {
			return err
		}
		if err := row.apply(f); err != nil {
			return err
		}
		if row.LastSeq {
			f.end = true
			return nil
		}
		return nil
	}
}

func (f *ChangesFeed) pollParser(r io.Reader) (func() error, error) {
	dec := json.NewDecoder(r)
	if err := expectTokens(dec, json.Delim('{'), "results", json.Delim('[')); err != nil {
		return nil, err
	}

	next := func() error {
		f.reset()

		// Decode next row.
		if dec.More() {
			var row changesRow
			if err := dec.Decode(&row); err != nil {
				return err
			}
			return row.apply(f)
		}

		// End of results reached, decode trailing object keys.
		if err := expectTokens(dec, json.Delim(']')); err != nil {
			return err
		}
		f.end = true
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return err
			}
			switch key {
			case "last_seq":
				if err := dec.Decode(&f.Seq); err != nil {
					return fmt.Errorf(`can't decode "last_seq" feed key: %v`, err)
				}
			case "pending":
				if err := dec.Decode(&f.Pending); err != nil {
					return fmt.Errorf(`can't decode "pending" feed key: %v`, err)
				}
			default:
				if err := skipValue(dec); err != nil {
					return fmt.Errorf(`can't skip over %q feed key: %v`, key, err)
				}
			}
		}
		return nil
	}
	return next, nil
}

// tokens verifies that the given tokens are present in the
// input stream. Whitespace between tokens is skipped.
func expectTokens(dec *json.Decoder, toks ...json.Token) error {
	for _, tok := range toks {
		tokin, err := dec.Token()
		if err != nil {
			return err
		}
		if tokin != tok {
			return fmt.Errorf("unexpected token: found %v, want %v", tokin, tok)
		}
	}
	return nil
}

// skipValue skips over the next JSON value in the decoder.
func skipValue(dec *json.Decoder) error {
	firstDelim, err := nextDelim(dec)
	if err != nil || firstDelim == 0 {
		// If the value is not an object or array, we're done skipping it.
		return err
	}
	var nesting = 1
	for nesting > 0 {
		d, err := nextDelim(dec)
		if err != nil {
			return err
		}
		switch d {
		case '{', '[':
			nesting++
		case '}', ']':
			nesting--
		default:
			// just skip
		}
	}
	return nil
}

// nextDelim decodes the next token and returns it as a delimiter.
// If the token is not a delimiter, it returns zero.
func nextDelim(dec *json.Decoder) (json.Delim, error) {
	tok, err := dec.Token()
	if err != nil {
		return 0, err
	}
	d, _ := tok.(json.Delim)
	return d, nil
}
