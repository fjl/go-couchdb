package couchdb_test

import (
	"encoding/json"
	"io"
	. "net/http"
	"testing"

	"github.com/fjl/go-couchdb"
)

func TestDBUpdatesFeed(t *testing.T) {
	c := newTestClient(t)
	c.Handle("GET /_db_updates", func(resp ResponseWriter, req *Request) {
		check(t, "request query string", "feed=continuous", req.URL.RawQuery)
		io.WriteString(resp, `{
			"db_name": "db",
			"seq": "1-...",
			"type": "created"
		}`+"\n")
		io.WriteString(resp, `{
			"db_name": "db2",
			"seq": "4-...",
			"type": "deleted"
		}`+"\n")
	})

	feed, err := c.DBUpdates(nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("-- first event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err", error(nil), feed.Err())
	check(t, "feed.DB", "db", feed.DB)
	check(t, "feed.Event", "created", feed.Event)
	check(t, "feed.Seq", "1-...", feed.Seq)

	t.Log("-- second event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err", error(nil), feed.Err())
	check(t, "feed.DB", "db2", feed.DB)
	check(t, "feed.Event", "deleted", feed.Event)
	check(t, "feed.Seq", "4-...", feed.Seq)

	t.Log("-- end of feed")
	check(t, "feed.Next()", false, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())
	check(t, "feed.DB", "", feed.DB)
	check(t, "feed.Event", "", feed.Event)
	check(t, "feed.Seq", nil, feed.Seq)
	check(t, "feed.OK", false, feed.OK)

	if err := feed.Close(); err != nil {
		t.Fatalf("feed.Close err: %v", err)
	}
}

// This test checks that the poll parser skips over unexpected object
// keys at the end of feed data.
func TestChangesFeedPoll_UnexpectedKeys(t *testing.T) {
	c := newTestClient(t)
	c.Handle("GET /db/_changes", func(resp ResponseWriter, req *Request) {
		check(t, "request query string", "", req.URL.RawQuery)
		io.WriteString(resp, `{
			"results": [
			],
			"last_seq": "99-...", "foobar": {"x": [1, "y"]}, "pending": 1
		}`)
	})
	feed, err := c.DB("db").Changes(nil)
	if err != nil {
		t.Fatalf("client.Changes error: %v", err)
	}

	t.Log("-- end of feed")
	check(t, "feed.Next()", false, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())
	check(t, "feed.Seq", "99-...", feed.Seq)
	check(t, "feed.Pending", int64(1), feed.Pending)
}

func TestChangesFeedPoll_Doc(t *testing.T) {
	c := newTestClient(t)
	c.Handle("GET /db/_changes", func(resp ResponseWriter, req *Request) {
		check(t, "request query string", "include_docs=true", req.URL.RawQuery)
		io.WriteString(resp, `{
			"results": [
				{
					"seq": "1-...",
					"id": "doc",
                    "doc": {"x": "y"},
					"deleted": true,
					"changes": [{"rev":"1-619db7ba8551c0de3f3a178775509611"}]
				}
			],
			"last_seq": "99-..."
		}`)
	})
	opt := couchdb.Options{"include_docs": true}
	feed, err := c.DB("db").Changes(opt)
	if err != nil {
		t.Fatalf("client.Changes error: %v", err)
	}

	t.Log("-- first event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())
	check(t, "feed.ID", "doc", feed.ID)
	check(t, "feed.Seq", "1-...", feed.Seq)
	check(t, "feed.Deleted", true, feed.Deleted)
	check(t, "feed.Doc", json.RawMessage(`{"x": "y"}`), feed.Doc)
	check(t, "feed.ChangesRevs", []string{"1-619db7ba8551c0de3f3a178775509611"}, feed.ChangesRevs())

	t.Log("-- end of feed")
	check(t, "feed.Next()", false, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())
	check(t, "feed.ID", "", feed.ID)
	check(t, "feed.Seq", "99-...", feed.Seq)
	check(t, "feed.Deleted", false, feed.Deleted)

	if err := feed.Close(); err != nil {
		t.Fatalf("feed.Close error: %v", err)
	}
}

func TestChangesFeedPoll_SeqInteger(t *testing.T) {
	c := newTestClient(t)
	c.Handle("GET /db/_changes", func(resp ResponseWriter, req *Request) {
		check(t, "request query string", "", req.URL.RawQuery)
		io.WriteString(resp, `{
			"results": [
				{
					"seq": 1,
					"id": "doc",
					"deleted": true,
					"changes": [{"rev":"1-619db7ba8551c0de3f3a178775509611"}]
				},
				{
					"seq": 2,
					"id": "doc",
					"changes": [{"rev":"1-619db7ba8551c0de3f3a178775509611"}]
				}
			],
			"last_seq": 99
		}`)
	})

	feed, err := c.DB("db").Changes(nil)
	if err != nil {
		t.Fatalf("client.Changes error: %v", err)
	}

	t.Log("-- first event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())
	check(t, "feed.ID", "doc", feed.ID)
	check(t, "feed.Seq", float64(1), feed.Seq)
	check(t, "feed.Deleted", true, feed.Deleted)

	t.Log("-- second event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())
	check(t, "feed.ID", "doc", feed.ID)
	check(t, "feed.Seq", float64(2), feed.Seq)
	check(t, "feed.Deleted", false, feed.Deleted)

	t.Log("-- end of feed")
	check(t, "feed.Next()", false, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())
	check(t, "feed.ID", "", feed.ID)
	check(t, "feed.Seq", float64(99), feed.Seq)
	check(t, "feed.Deleted", false, feed.Deleted)

	if err := feed.Close(); err != nil {
		t.Fatalf("feed.Close error: %v", err)
	}
}

func TestChangesFeedPoll_SeqString(t *testing.T) {
	c := newTestClient(t)
	c.Handle("GET /db/_changes", func(resp ResponseWriter, req *Request) {
		check(t, "request query string", "", req.URL.RawQuery)
		io.WriteString(resp, `{
			"results": [
				{
					"seq": "1-...",
					"id": "doc",
					"deleted": true,
					"changes": [{"rev":"1-619db7ba8551c0de3f3a178775509611"}]
				},
				{
					"seq": "2-...",
					"id": "doc",
					"changes": [{"rev":"1-619db7ba8551c0de3f3a178775509611"}]
				}
			],
			"last_seq": "99-..."
		}`)
	})

	feed, err := c.DB("db").Changes(nil)
	if err != nil {
		t.Fatalf("client.Changes error: %v", err)
	}

	t.Log("-- first event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())
	check(t, "feed.ID", "doc", feed.ID)
	check(t, "feed.Seq", "1-...", feed.Seq)
	check(t, "feed.Deleted", true, feed.Deleted)

	t.Log("-- second event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())
	check(t, "feed.ID", "doc", feed.ID)
	check(t, "feed.Seq", "2-...", feed.Seq)
	check(t, "feed.Deleted", false, feed.Deleted)

	t.Log("-- end of feed")
	check(t, "feed.Next()", false, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())
	check(t, "feed.ID", "", feed.ID)
	check(t, "feed.Seq", "99-...", feed.Seq)
	check(t, "feed.Deleted", false, feed.Deleted)

	if err := feed.Close(); err != nil {
		t.Fatalf("feed.Close error: %v", err)
	}
}

func TestChangesFeedCont_SeqInteger(t *testing.T) {
	c := newTestClient(t)
	c.Handle("GET /db/_changes", func(resp ResponseWriter, req *Request) {
		check(t, "request query string", "feed=continuous", req.URL.RawQuery)
		io.WriteString(resp, `{
			"seq": 1,
			"id": "doc",
			"deleted": true,
			"changes": [{"rev":"1-619db7ba8551c0de3f3a178775509611"}]
		}`+"\n")
		io.WriteString(resp, `{
			"seq": 2,
			"id": "doc",
			"changes": [{"rev":"1-619db7ba8551c0de3f3a178775509611"}]
		}`+"\n")
		io.WriteString(resp, `{
			"seq": 99,
			"last_seq": true
		}`+"\n")
	})

	feed, err := c.DB("db").Changes(couchdb.Options{"feed": "continuous"})
	if err != nil {
		t.Fatalf("client.Changes error: %v", err)
	}

	t.Log("-- first event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())
	check(t, "feed.ID", "doc", feed.ID)
	check(t, "feed.Seq", float64(1), feed.Seq)
	check(t, "feed.Deleted", true, feed.Deleted)

	t.Log("-- second event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())
	check(t, "feed.ID", "doc", feed.ID)
	check(t, "feed.Seq", float64(2), feed.Seq)
	check(t, "feed.Deleted", false, feed.Deleted)

	t.Log("-- end of feed")
	check(t, "feed.Next()", false, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())
	check(t, "feed.ID", "", feed.ID)
	check(t, "feed.Seq", float64(99), feed.Seq)
	check(t, "feed.Deleted", false, feed.Deleted)

	if err := feed.Close(); err != nil {
		t.Fatalf("feed.Close error: %v", err)
	}
}

func TestChangesFeedCont_Doc(t *testing.T) {
	c := newTestClient(t)
	c.Handle("GET /db/_changes", func(resp ResponseWriter, req *Request) {
		check(t, "request query string", "feed=continuous&include_docs=true", req.URL.RawQuery)
		io.WriteString(resp, `{
			"seq": "1-...",
			"id": "doc",
            "doc": {"x": "y"},
			"deleted": true,
			"changes": [{"rev":"1-619db7ba8551c0de3f3a178775509611"}]
		}`+"\n")
		io.WriteString(resp, `{
			"seq": "99-...",
			"last_seq": true
		}`+"\n")
	})

	opt := couchdb.Options{"include_docs": true, "feed": "continuous"}
	feed, err := c.DB("db").Changes(opt)
	if err != nil {
		t.Fatalf("client.Changes error: %v", err)
	}

	t.Log("-- first event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())
	check(t, "feed.ID", "doc", feed.ID)
	check(t, "feed.Seq", "1-...", feed.Seq)
	check(t, "feed.Deleted", true, feed.Deleted)
	check(t, "feed.Doc", json.RawMessage(`{"x": "y"}`), feed.Doc)
	check(t, "feed.ChangesRevs", []string{"1-619db7ba8551c0de3f3a178775509611"}, feed.ChangesRevs())

	t.Log("-- end of feed")
	check(t, "feed.Next()", false, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())
	check(t, "feed.ID", "", feed.ID)
	check(t, "feed.Seq", "99-...", feed.Seq)
	check(t, "feed.Deleted", false, feed.Deleted)

	if err := feed.Close(); err != nil {
		t.Fatalf("feed.Close error: %v", err)
	}
}

func TestChangesFeedCont_SeqString(t *testing.T) {
	c := newTestClient(t)
	c.Handle("GET /db/_changes", func(resp ResponseWriter, req *Request) {
		check(t, "request query string", "feed=continuous", req.URL.RawQuery)
		io.WriteString(resp, `{
			"seq": "1-...",
			"id": "doc",
			"deleted": true,
			"changes": [{"rev":"1-619db7ba8551c0de3f3a178775509611"}]
		}`+"\n")
		io.WriteString(resp, `{
			"seq": "2-...",
			"id": "doc",
			"changes": [{"rev":"1-619db7ba8551c0de3f3a178775509611"}]
		}`+"\n")
		io.WriteString(resp, `{
			"seq": "99-...",
			"last_seq": true
		}`+"\n")
	})

	feed, err := c.DB("db").Changes(couchdb.Options{"feed": "continuous"})
	if err != nil {
		t.Fatalf("client.Changes error: %v", err)
	}

	t.Log("-- first event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())
	check(t, "feed.ID", "doc", feed.ID)
	check(t, "feed.Seq", "1-...", feed.Seq)
	check(t, "feed.Deleted", true, feed.Deleted)

	t.Log("-- second event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())
	check(t, "feed.ID", "doc", feed.ID)
	check(t, "feed.Seq", "2-...", feed.Seq)
	check(t, "feed.Deleted", false, feed.Deleted)

	t.Log("-- end of feed")
	check(t, "feed.Next()", false, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())
	check(t, "feed.ID", "", feed.ID)
	check(t, "feed.Seq", "99-...", feed.Seq)
	check(t, "feed.Deleted", false, feed.Deleted)

	if err := feed.Close(); err != nil {
		t.Fatalf("feed.Close error: %v", err)
	}
}
