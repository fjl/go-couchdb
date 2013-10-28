// This file contains methods to access CouchDB changes feeds.

package couchdb

import (
	"encoding/json"
	"github.com/bitly/go-simplejson"
	"io"
)

// This is the representation of database update notifications
// in the _db_updates feed.
type DbEvent struct {
	Name string `json:"db_name"` // The database that was affected
	Type string `json:"type"`    // "created" | "updated" | "deleted"
}

type DbEventFeed struct {
	io.Closer
	dec *json.Decoder
}

// Open the _db_updates feed. This feed receives an event whenever
// a database is created, updated or deleted.
//
// Don't forget to Close() the feed.
func (srv *Server) Updates(options Options) (*DbEventFeed, error) {
	newopts := options.clone()
	newopts["feed"] = "continuous"
	resp, err := srv.request("GET", nil, newopts, "_db_updates")
	if err != nil {
		return nil, err
	} else {
		feed := &DbEventFeed{
			Closer: resp.Body,
			dec:    json.NewDecoder(resp.Body),
		}
		return feed, nil
	}
}

// Next decodes the next event in a _db_updates feed.
//
// It is not safe to call this method from more than one
// goroutine at the same time.
func (f *DbEventFeed) Next() (event DbEvent, err error) {
	err = f.dec.Decode(&event)
	return
}

// This is the representation for the events returned
// in a database _changes feed.
type DocEvent struct {
	Last     bool      // true if this is a last_seq event
	Seq      int64     // Event sequence number
	Database *Database // The database that the change occurred in
	Deleted  bool      // Whether the document was deleted
	Id       string    // Document id
}

type ChangesFeed struct {
	io.Closer
	db  *Database
	dec *json.Decoder
}

// Open the _changes feed of a database.
// This feed receives an event whenever a document is created,
// updated or deleted.
//
// Don't forget to Close() the feed.
func (db *Database) Changes(options Options) (*ChangesFeed, error) {
	newopts := options.clone()
	newopts["feed"] = "continuous"
	resp, err := db.srv.request("GET", nil, newopts, db.name, "_changes")
	if err != nil {
		return nil, err
	} else {
		feed := &ChangesFeed{
			Closer: resp.Body,
			db:     db,
			dec:    json.NewDecoder(resp.Body),
		}
		return feed, nil
	}
}

// Next decodes an event from a _changes feed.
//
// CouchDB usually sends a special "last_seq" event before closing
// the connection on the server side. The field DocEvent.Last
// can be used to detect those.
//
// It is not safe to call this method from more than one goroutine
// at the same time.
func (f *ChangesFeed) Next() (event DocEvent, err error) {
	event.Database = f.db

	var json simplejson.Json
	err = f.dec.Decode(&json)
	if err != nil {
		return
	}

	// TODO: handle invalid documents better
	if last, ok := json.CheckGet("last_seq"); ok {
		event.Last = true
		event.Seq = last.MustInt64()
	} else {
		event.Id = json.Get("id").MustString()
		event.Deleted = json.Get("deleted").MustBool()
		event.Seq = json.Get("seq").MustInt64(0)
	}

	return
}
