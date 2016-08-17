package couchdb

// Tests for the CouchDB2/Cloudant specific extensions
import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

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
