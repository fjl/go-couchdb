package couchdb_test

import (
	"context"
	"testing"
)

func TestDBContext(t *testing.T) {
	c := newTestClient(t)
	db := c.DB("fooo")
	ndb := db.Context(context.TODO())
	if ndb == db {
		t.Errorf("database object not replaced")
	}
	if ndb.GetContext() == db.GetContext() {
		t.Errorf("expected contexts to change")
	}
}
