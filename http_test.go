package couchdb

import (
	"net/http"
	"testing"
)

type testauth struct{ called bool }

func (a *testauth) AddAuth(*http.Request) {
	a.called = true
}

func TestClientSetAuth(t *testing.T) {
	c := newTestClient(t)
	c.Handle("HEAD /", func(resp http.ResponseWriter, req *http.Request) {})

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

func TestPath(t *testing.T) {
	data := map[string][]string {
		// Expected output						Input
		"/foo/bar/baz":							[]string{"foo","bar","baz"},
		"/foo/_design/bar/_view/baz":			[]string{"foo","_design/bar","_view","baz"},
		"/foo%2Fbar/baz%2Fquz":					[]string{"foo/bar","baz/quz"},
	}
	for expected,segs := range data {
		result := path(segs...)
		if result != expected {
			t.Fatalf("path() produced '%s', expected '%s'", result, expected)
		}
	}
}
