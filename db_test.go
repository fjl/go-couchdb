package couchdb_test

import (
	"context"
	"testing"
)

func TestDBContext(t *testing.T) {
	c := newTestClient(t)
	db := c.DB("fooo")
	ndb := db.WithContext(context.TODO())
	if ndb == db {
		t.Errorf("database object not replaced")
	}
	if ndb.Context() == db.Context() {
		t.Errorf("expected contexts to change")
	}
	if ndb.Context() != context.TODO() {
		t.Errorf("expected new context to be %v", context.TODO())
	}
}
