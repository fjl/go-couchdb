package couchdb

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestDBUpdatesFeed(t *testing.T) {
	c := newTestClient(t)
	c.Handle("GET /_db_updates", func(resp http.ResponseWriter, req *http.Request) {
		check(t, "request query string", "feed=continuous", req.URL.RawQuery)
		io.WriteString(resp, `{
			"db_name": "db",
			"ok": true,
			"type": "created"
		}`+"\n")
		io.WriteString(resp, `{
			"db_name": "db2",
			"ok": false,
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
	check(t, "feed.OK", true, feed.OK)

	t.Log("-- second event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err", error(nil), feed.Err())

	check(t, "feed.DB", "db2", feed.DB)
	check(t, "feed.Event", "deleted", feed.Event)
	check(t, "feed.OK", false, feed.OK)

	t.Log("-- end of feed")
	check(t, "feed.Next()", false, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())

	check(t, "feed.DB", "", feed.DB)
	check(t, "feed.Event", "", feed.Event)
	check(t, "feed.OK", false, feed.OK)

	if err := feed.Close(); err != nil {
		t.Fatalf("feed.Close err: %v", err)
	}
}

func TestChangesFeedPoll(t *testing.T) {
	c := newTestClient(t)
	c.Handle("GET /db/_changes", func(resp http.ResponseWriter, req *http.Request) {
		check(t, "request query string", "", req.URL.RawQuery)
		io.WriteString(resp, `{
			"results": [
				{
					"seq": 1,
					"id": "doc",
					"deleted": true,
					"changes": [{"rev":"1-619db7ba8551c0de3f3a178775509611"}],
					"doc": {
		        		"_id":"doc",
		        		"_rev":"1-619db7ba8551c0de3f3a178775509611",
		        		"user":"Random J. User",
		        		"email":"random2@domain.com",
		        		"highscore":52
		    		}
				},
				{
					"seq": "2-hdhff",
					"id": "doc",
					"changes": [{"rev":"1-619db7ba8551c0de3f3a178775509611"}],
					"doc": {
		        		"_id":"doc",
		        		"_rev":"1-619db7ba8551c0de3f3a178775509611",
		        		"user":"Random J. User",
		        		"email":"random2@domain.com",
		        		"highscore":53
		    		}
				}
			],
			"last_seq": "99-kjashdkf"
		}`)
	})

	feed, err := c.DB("db").Changes(nil)
	if err != nil {
		t.Fatalf("client.Changes error: %v", err)
	}

	// TODO: check "changes"

	t.Log("-- first event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())

	check(t, "feed.ID", "doc", feed.ID)
	//check(t, "feed.Seq", int64(1), feed.Seq)
	if seq, ok := feed.Seq.(int64); ok {
		check(t, "feed.Seq", int64(1), seq)
	}
	check(t, "feed.Deleted", true, feed.Deleted)

	t.Log("-- second event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())

	check(t, "feed.ID", "doc", feed.ID)
	if seq, ok := feed.Seq.(string); ok {
		check(t, "feed.Seq", "2-hdhff", seq)
	}
	check(t, "feed.Deleted", false, feed.Deleted)

	t.Log("-- end of feed")
	check(t, "feed.Next()", false, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())

	check(t, "feed.ID", "", feed.ID)
	if seq, ok := feed.Seq.(string); ok {
		check(t, "feed.Seq", "99-kjashdkf", seq)
	}
	check(t, "feed.Deleted", false, feed.Deleted)

	if err := feed.Close(); err != nil {
		t.Fatalf("feed.Close error: %v", err)
	}
}

func TestChangesFeedCont(t *testing.T) {
	c := newTestClient(t)
	c.Handle("GET /db/_changes", func(resp http.ResponseWriter, req *http.Request) {
		check(t, "request query string", "feed=continuous", req.URL.RawQuery)
		io.WriteString(resp, `{
			"seq": 1,
			"id": "doc",
			"deleted": true,
			"changes": [{"rev":"1-619db7ba8551c0de3f3a178775509611"}],
			"doc": {
        		"_id":"doc",
        		"_rev":"1-619db7ba8551c0de3f3a178775509611",
        		"user":"Random J. User",
        		"email":"random2@domain.com",
        		"highscore":52
    		}
		}`+"\n")
		io.WriteString(resp, `{
			"seq": "2-lisdfg",
			"id": "doc",
			"changes": [{"rev":"1-619db7ba8551c0de3f3a178775509611"}],
			"doc": {
        		"_id":"doc",
        		"_rev":"1-619db7ba8551c0de3f3a178775509611",
        		"user":"Random J. User",
        		"email":"random2@domain.com",
        		"highscore":53
    		}
		}`+"\n")
		io.WriteString(resp, `{
			"seq": "99-987234982734hjk",
			"last_seq": true
		}`+"\n")
	})

	feed, err := c.DB("db").Changes(Options{"feed": "continuous"}) // add include_docs
	if err != nil {
		t.Fatalf("client.Changes error: %v", err)
	}

	// TODO: check "changes"

	t.Log("-- first event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())

	check(t, "feed.ID", "doc", feed.ID)
	if seq, ok := feed.Seq.(int64); ok {
		check(t, "feed.Seq", int64(1), seq)
	}
	check(t, "feed.Deleted", true, feed.Deleted)

	t.Log("-- second event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())

	check(t, "feed.ID", "doc", feed.ID)
	if seq, ok := feed.Seq.(string); ok {
		check(t, "feed.Seq", "2-lisdfg", seq)
	}

	check(t, "feed.Deleted", false, feed.Deleted)

	t.Log("-- end of feed")
	check(t, "feed.Next()", false, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())

	check(t, "feed.ID", "", feed.ID)
	if seq, ok := feed.Seq.(string); ok {
		check(t, "feed.Seq", "99-987234982734hjk", seq)
	}
	check(t, "feed.Deleted", false, feed.Deleted)

	if err := feed.Close(); err != nil {
		t.Fatalf("feed.Close error: %v", err)
	}
}

// TestChangesFeedPollSeqString tests the polled changes feed with a CouchDB2-style
// seq id - a string.
func TestChangesFeedPollSeqString(t *testing.T) {
	c := newTestClient(t)
	c.Handle("GET /db/_changes", func(resp http.ResponseWriter, req *http.Request) {
		check(t, "request query string", "", req.URL.RawQuery)
		io.WriteString(resp, `{
			"results": [
				{
					"seq": "1-ba445ec888551c",
					"id": "doc",
					"changes": [{"rev":"1-619db7ba8551c0de3f3a178775509611"}]
				},
				{
					"seq": "2-8551c0db7ba8551c0",
					"id": "doc",
					"changes": [{"rev":"1-619db7ba8551c0de3f3a178775509611"}]
				}
			],
			"last_seq": "99-7877550961db7b"
		}`)
	})

	feed, err := c.DB("db").Changes(nil)
	if err != nil {
		t.Fatalf("client.Changes error: %v", err)
	}

	// TODO: check "changes"

	t.Log("-- first event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())

	check(t, "feed.ID", "doc", feed.ID)
	if seq, ok := feed.Seq.(string); ok {
		check(t, "feed.Seq", "1-ba445ec888551c", seq)
	}

	t.Log("-- second event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())

	check(t, "feed.ID", "doc", feed.ID)
	if seq, ok := feed.Seq.(string); ok {
		check(t, "feed.Seq", "2-8551c0db7ba8551c0", seq)
	}

	t.Log("-- end of feed")
	check(t, "feed.Next()", false, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())

	check(t, "feed.ID", "", feed.ID)
	if seq, ok := feed.Seq.(string); ok {
		check(t, "feed.Seq", "99-7877550961db7b", seq)
	}

	if err := feed.Close(); err != nil {
		t.Fatalf("feed.Close error: %v", err)
	}
}

// TestChangesFeedPollIncludeDocs tests the 'include_docs' option.
func TestChangesFeedPollIncludeDocs(t *testing.T) {
	c := newTestClient(t)
	c.Handle("GET /db/_changes", func(resp http.ResponseWriter, req *http.Request) {
		check(t, "request query string", "include_docs=true", req.URL.RawQuery)
		io.WriteString(resp, `{
			"results": [
				{
					"seq": "1-ba445ec888551c",
					"id": "doc1",
					"changes": [{"rev":"1-619db7ba8551c0de3f3a178775509611"}],
					"doc": {
		        		"_id":"doc1",
		        		"_rev":"1-619db7ba8551c0de3f3a178775509611",
		        		"user":"Random J. User1",
		        		"email":"random1@domain.com",
		        		"highscore":58
		    		}
				},
				{
					"seq": "2-8551c0db7ba8551c0",
					"id": "doc2",
					"changes": [{"rev":"2-619db7ba8551c0de3f3a178775509611"}],
					"doc": {
		        		"_id":"doc2",
		        		"_rev":"2-619db7ba8551c0de3f3a178775509611",
		        		"user":"Random J. User2",
		        		"email":"random2@domain.com",
		        		"highscore":53
		    		}
				}
			],
			"last_seq": "99-7877550961db7b"
		}`)
	})

	feed, err := c.DB("db").Changes(Options{"include_docs": true})
	if err != nil {
		t.Fatalf("client.Changes error: %v", err)
	}

	var doc map[string]interface{}

	t.Log("-- first event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())
	err = json.Unmarshal([]byte(feed.Doc), &doc)
	check(t, "Unmarshal doc", true, err == nil)
	if email, ok := doc["email"].(string); ok {
		check(t, "doc['email']", "random1@domain.com", email)
	} else {
		t.Fatalf("type assertion error")
	}

	t.Log("-- second event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())

	err = json.Unmarshal([]byte(feed.Doc), &doc)
	check(t, "Unmarshal doc", true, err == nil)
	if email, ok := doc["email"].(string); ok {
		check(t, "doc['email']", "random2@domain.com", email)
	} else {
		t.Fatalf("type assertion error")
	}

	t.Log("-- end of feed")
	check(t, "feed.Next()", false, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())

	if err := feed.Close(); err != nil {
		t.Fatalf("feed.Close error: %v", err)
	}
}

// TestChangesFeedContSeqString tests the continuous changes feed with a CouchDB2-style
// seq id - a string.
func TestChangesFeedContSeqString(t *testing.T) {
	c := newTestClient(t)
	c.Handle("GET /db/_changes", func(resp http.ResponseWriter, req *http.Request) {
		check(t, "request query string", "feed=continuous", req.URL.RawQuery)
		io.WriteString(resp, `{
			"seq": "1-ba445ec888551c",
			"id": "doc",
			"changes": [{"rev":"1-619db7ba8551c0de3f3a178775509611"}]
		}`+"\n")
		io.WriteString(resp, `{
			"seq": "2-8551c0db7ba8551c0",
			"id": "doc",
			"changes": [{"rev":"1-619db7ba8551c0de3f3a178775509611"}]
		}`+"\n")
		io.WriteString(resp, `{
			"seq": "99-7877550961db7b",
			"last_seq": true
		}`+"\n")
	})

	feed, err := c.DB("db").Changes(Options{"feed": "continuous"})
	if err != nil {
		t.Fatalf("client.Changes error: %v", err)
	}

	// TODO: check "changes"

	t.Log("-- first event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())

	check(t, "feed.ID", "doc", feed.ID)
	if seq, ok := feed.Seq.(string); ok {
		check(t, "feed.Seq", "1-ba445ec888551c", seq)
	}

	t.Log("-- second event")
	check(t, "feed.Next()", true, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())

	check(t, "feed.ID", "doc", feed.ID)
	if seq, ok := feed.Seq.(string); ok {
		check(t, "feed.Seq", "2-8551c0db7ba8551c0", seq)
	}

	t.Log("-- end of feed")
	check(t, "feed.Next()", false, feed.Next())
	check(t, "feed.Err()", error(nil), feed.Err())

	check(t, "feed.ID", "", feed.ID)
	if seq, ok := feed.Seq.(string); ok {
		check(t, "feed.Seq", "99-7877550961db7b", seq)
	}

	if err := feed.Close(); err != nil {
		t.Fatalf("feed.Close error: %v", err)
	}
}
