package couchdb_test

import (
	. "net/http"
	"testing"
)

type testauth struct{ called bool }

func (a *testauth) AddAuth(*Request) {
	a.called = true
}

func TestClientSetAuth(t *testing.T) {
	c := newTestClient(t)
	c.Handle("HEAD /", func(resp ResponseWriter, req *Request) {})

	auth := new(testauth)
	c.SetAuth(auth)
	if err := c.Ping(); err != nil {
		t.Fatal(err)
	}
	if !auth.called {
		t.Error("AddAuth was not called")
	}

	auth.called = false
	c.SetAuth(nil)
	if err := c.Ping(); err != nil {
		t.Fatal(err)
	}
	if auth.called {
		t.Error("AddAuth was called after removing Auth instance")
	}
}
