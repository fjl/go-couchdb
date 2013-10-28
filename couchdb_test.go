package couchdb

import (
	"math/rand"
	"testing"
	"time"
)

var (
	srv = NewServer("http://127.0.0.1:5984")
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestPing(t *testing.T) {
	if err := srv.Ping(); err != nil {
		t.Fatal(err)
	}
}

func TestDatabaseAPI(t *testing.T) {
	dbname := "go-couch-test-db-api"

	db := srv.Db(dbname)
	if db.Name() != dbname {
		t.Errorf("db name %#v did not match %#v", db.Name(), dbname)
	}

	db, err := srv.CreateDb(dbname)
	if err != nil {
		t.Fatal(err)
	}
	if db.Name() != dbname {
		t.Errorf("db name %#v did not match %#v", db.Name(), dbname)
	}

	names, err := srv.AllDbs()
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(dbname, names) {
		t.Errorf("_all_dbs didn't contain %v", dbname)
	}

	db, err = srv.OpenDb(dbname)
	if err != nil {
		t.Fatal(err)
	}
	if db.Name() != dbname {
		t.Errorf("db name %#v did not match %#v", db.Name(), dbname)
	}

	err = srv.DeleteDb(dbname)
	if err != nil {
		t.Fatal(err)
	}
}

type TestDocument struct {
	Rev          string `json:"_rev,omitempty"`
	RandomNumber int
}

func newTestdoc() TestDocument {
	return TestDocument{
		RandomNumber: rand.Intn(100000) + 1,
	}
}

func TestGetNonexisting(t *testing.T) {
	db := resetDb(t, "go-couch-test-doc-api")

	var doc TestDocument
	err := db.Get("non-existing-test-doc", nil, doc)
	if !NotFound(err) {
		t.Errorf("db.Get returned unexpected error: %+v", err)
	}
}

func TestPut(t *testing.T) {
	db := resetDb(t, "go-couch-test-doc-api")

	docid := "test-put-doc"
	doc := newTestdoc()
	rev, err := db.Put(docid, doc)
	if err != nil {
		t.Fatal(err)
	}

	var fromdb TestDocument
	if err := db.Get(docid, nil, &fromdb); err != nil {
		t.Fatal(err)
	}
	if fromdb.Rev != rev {
		t.Errorf("rev did not match, %q != %q", fromdb.Rev, rev)
	}
	if fromdb.RandomNumber != doc.RandomNumber {
		t.Errorf("doc.RandomNumber did not match, %v != %v",
			fromdb.RandomNumber, doc.RandomNumber)
	}
}

func TestDelete(t *testing.T) {
	dbname := "go-couch-test-doc-api"
	db := resetDb(t, dbname)

	docid := "test-delete-doc"
	rev, err := db.Put(docid, newTestdoc())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.Delete(docid, rev); err != nil {
		t.Fatal(err)
	}
}

func TestDbUpdatesFeed(t *testing.T) {
	db := resetDb(t, "go-couch-test-db-events")

	feed, err := srv.Updates(nil)
	if err != nil {
		t.Fatal(err)
	}

	docid := "test-doc-1"
	_, err = db.Put(docid, newTestdoc())
	if err != nil {
		t.Fatal(err)
	}

	event, err := feed.Next()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("event: %+v", event)
	if event.Name != db.Name() {
		t.Errorf("database name didn't match, %q != %q", event.Name, db.Name())
	}
	if event.Type != "updated" {
		t.Errorf("type didn't match, %q != %q", event.Type, "updated")
	}

	feed.Close()
}

func TestChangesFeed(t *testing.T) {
	db := resetDb(t, "go-couch-test-doc-events")

	feed, err := db.Changes(nil)
	if err != nil {
		t.Fatal(err)
	}

	docid := "test-doc-1"
	_, err = db.Put(docid, newTestdoc())
	if err != nil {
		t.Fatal(err)
	}

	event, err := feed.Next()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("event: %+v", event)
	if event.Id != docid {
		t.Fatal("id didn't match")
	}
	if event.Database != db {
		t.Fatal("database didn't match")
	}

	feed.Close()
}

// Helper function that deletes and recreates a database.
func resetDb(t *testing.T, name string) *Database {
	srv.DeleteDb(name) // ignore error

	// wait a bit until deletion is complete
	// if this is not done, the CreateDb will
	// mysteriously fail sometimes
	time.Sleep(10 * time.Millisecond)

	if db, err := srv.CreateDb(name); err != nil {
		t.Fatal(err)
		return nil
	} else {
		return db
	}
}

func containsStr(needle string, slice []string) bool {
	for _, s := range slice {
		if s == needle {
			return true
		}
	}
	return false
}
