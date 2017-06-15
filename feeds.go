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
	Event string `json:"type"`    // "created" | "updated" | "deleted"
	OK    bool   `json:"ok"`      // Event operation status
	DB    string `json:"db_name"` // Event database name

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
	f.Event, f.DB, f.OK = "", "", false
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
	// After all items have been processed, set to the last_seq value sent
	// by CouchDB.
	Seq interface{} `json:"seq"`

	// LastSeq last change sequence number
	LastSeq interface{} `json:"last_seq"`

	// Changes is the list of the document's leaf revisions.
	/*
		Changes []struct {
			Rev string `json:"rev"`
		} `json:"changes"`
	*/

	// The document. This is populated only if the feed option
	// "include_docs" is true.
	Doc json.RawMessage `json:"doc"`

	end     bool
	err     error
	conn    io.Closer
	decoder *json.Decoder
	parser  func() error
}

// ContinuousChanges opens the _changes feed of a database for continuous feed updates.
// This feed receives an event whenever a document is created, updated or deleted.
//
// This implementation only continuous feeds.

// There are many other options that allow you to customize what the
// feed returns. For information on all of them, see the official CouchDB
// documentation:
//
// http://docs.couchdb.org/en/latest/api/database/changes.html#db-changes
func (db *DB) ContinuousChanges(options Options) (*ChangesFeed, error) {
	options["feed"] = "continuous"
	path, err := optpath(options, nil, db.name, "_changes")
	if err != nil {
		return nil, err
	}

	resp, err := db.request("GET", path, nil)
	if err != nil {
		return nil, err
	}

	feed := &ChangesFeed{
		DB:      db,
		conn:    resp.Body,
		decoder: json.NewDecoder(resp.Body),
	}

	return feed, nil
}

// Next decodes the next event. It returns false when the feeds end has been
// reached or an error has occurred.
func (f *ChangesFeed) Next() (bool, error) {
	// the json doesn't include the 'deleted' attr unless it's deleted,
	// so we need to set this to false before parsing the next row so that
	// it's not maintained from the previous row
	f.Deleted = false

	if f.end {
		return false, nil
	}
	if f.err = f.parse(); f.err != nil || f.end {
		f.Close()
	}
	return !f.end, f.err
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

func (f *ChangesFeed) parse() error {
	if err := f.decoder.Decode(f); err != nil {
		return err
	}

	var err error
	f.end, err = f.isEnd()
	return err
}

func (f *ChangesFeed) isEnd() (bool, error) {
	if f.LastSeq == nil {
		return false, nil
	}

	switch f.LastSeq.(type) {
	case string:
		return f.LastSeq.(string) != "", nil
	case int:
		return f.LastSeq.(int) > 0, nil
	case int64:
		return f.LastSeq.(int64) > 0, nil
	case float32:
		return f.LastSeq.(float32) > 0, nil
	case float64:
		return f.LastSeq.(float64) > 0, nil
	default:
		err := fmt.Errorf("LastSeq of type %T is not supported, assuming feed end", f.LastSeq)
		return true, err
	}

}
