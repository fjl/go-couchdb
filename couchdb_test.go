package couchdb_test

import (
	"bytes"
	"github.com/fjl/go-couchdb"
	"io"
	"io/ioutil"
	. "net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"testing"
)

// testClient is a very special couchdb.Client that also implements
// the http.RoundTripper interface. The tests can register HTTP
// handlers on the testClient. Any requests made through the client are
// dispatched to a matching handler. This allows us to test what the
// HTTP client in the couchdb package does without actually using the network.
//
// If no handler matches the requests method/path combination, the test
// fails with a descriptive error.
type testClient struct {
	*couchdb.Client
	t        *testing.T
	handlers map[string]Handler
}

func (s *testClient) Handle(pat string, f func(ResponseWriter, *Request)) {
	s.handlers[pat] = HandlerFunc(f)
}

func (s *testClient) ClearHandlers() {
	s.handlers = make(map[string]Handler)
}

func (s *testClient) RoundTrip(req *Request) (*Response, error) {
	handler, ok := s.handlers[req.Method+" "+req.URL.Path]
	if !ok {
		s.t.Fatalf("unhandled request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	}
	recorder := httptest.NewRecorder()
	recorder.Body = new(bytes.Buffer)
	handler.ServeHTTP(recorder, req)
	resp := &Response{
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		StatusCode:    recorder.Code,
		Status:        StatusText(recorder.Code),
		Header:        recorder.HeaderMap,
		ContentLength: int64(recorder.Body.Len()),
		Body:          ioutil.NopCloser(recorder.Body),
		Request:       req,
	}
	return resp, nil
}

func newTestClient(t *testing.T) *testClient {
	c := &testClient{t: t, handlers: make(map[string]Handler)}
	c.Client = couchdb.NewClient("http://testClient:5984/", c)
	return c
}

func TestServerURL(t *testing.T) {
	c := newTestClient(t)
	check(t, "c.URL()", "http://testClient:5984", c.URL())
}

func TestPing(t *testing.T) {
	c := newTestClient(t)
	c.Handle("HEAD /", func(resp ResponseWriter, req *Request) {})

	if err := c.Ping(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateDB(t *testing.T) {
	c := newTestClient(t)
	c.Handle("PUT /db", func(resp ResponseWriter, req *Request) {})

	db, err := c.CreateDB("db")
	if err != nil {
		t.Fatal(err)
	}

	check(t, "db.Name()", "db", db.Name())
}

func TestDeleteDB(t *testing.T) {
	c := newTestClient(t)
	c.Handle("DELETE /db", func(resp ResponseWriter, req *Request) {})
	if err := c.DeleteDB("db"); err != nil {
		t.Fatal(err)
	}
}

func TestAllDBs(t *testing.T) {
	c := newTestClient(t)
	c.Handle("GET /_all_dbs", func(resp ResponseWriter, req *Request) {
		io.WriteString(resp, `["a","b","c"]`)
	})

	names, err := c.AllDBs()
	if err != nil {
		t.Fatal(err)
	}
	check(t, "returned names", []string{"a", "b", "c"}, names)
}

// those are re-used across several tests
var securityObjectJSON = regexp.MustCompile("\\s").ReplaceAllString(
	`{
		"admins": {
			"names": ["adminName1", "adminName2"]
		},
		"members": {
			"names": ["memberName1"],
			"roles": ["memberRole1"]
		}
	}`, "")
var securityObject = &couchdb.Security{
	Admins: couchdb.Members{
		Names: []string{"adminName1", "adminName2"},
		Roles: nil,
	},
	Members: couchdb.Members{
		Names: []string{"memberName1"},
		Roles: []string{"memberRole1"},
	},
}

func TestSecurity(t *testing.T) {
	c := newTestClient(t)
	c.Handle("GET /db/_security", func(resp ResponseWriter, req *Request) {
		io.WriteString(resp, securityObjectJSON)
	})

	secobj, err := c.DB("db").Security()
	if err != nil {
		t.Fatal(err)
	}
	check(t, "secobj", securityObject, secobj)
}

func TestEmptySecurity(t *testing.T) {
	c := newTestClient(t)
	c.Handle("GET /db/_security", func(resp ResponseWriter, req *Request) {
		// CouchDB returns an empty reply if no security object has been set
		resp.WriteHeader(200)
	})

	secobj, err := c.DB("db").Security()
	if err != nil {
		t.Fatal(err)
	}
	check(t, "secobj", &couchdb.Security{}, secobj)
}

func TestPutSecurity(t *testing.T) {
	c := newTestClient(t)
	c.Handle("PUT /db/_security", func(resp ResponseWriter, req *Request) {
		body, _ := ioutil.ReadAll(req.Body)
		check(t, "request body", securityObjectJSON, string(body))
		resp.WriteHeader(200)
	})

	err := c.DB("db").PutSecurity(securityObject)
	if err != nil {
		t.Fatal(err)
	}
}

type testDocument struct {
	Rev   string `json:"_rev,omitempty"`
	Field int64  `json:"field"`
}

func TestGetExistingDoc(t *testing.T) {
	c := newTestClient(t)
	c.Handle("GET /db/doc", func(resp ResponseWriter, req *Request) {
		io.WriteString(resp, `{
			"_id": "doc",
			"_rev": "1-619db7ba8551c0de3f3a178775509611",
			"field": 999
		}`)
	})

	var doc testDocument
	if err := c.DB("db").Get("doc", &doc, nil); err != nil {
		t.Fatal(err)
	}
	check(t, "doc.Rev", "1-619db7ba8551c0de3f3a178775509611", doc.Rev)
	check(t, "doc.Field", int64(999), doc.Field)
}

func TestGetNonexistingDoc(t *testing.T) {
	c := newTestClient(t)
	c.Handle("GET /db/doc", func(resp ResponseWriter, req *Request) {
		resp.WriteHeader(404)
		io.WriteString(resp, `{"error":"not_found","reason":"error reason"}`)
	})

	var doc testDocument
	err := c.DB("db").Get("doc", doc, nil)
	check(t, "couchdb.NotFound(err)", true, couchdb.NotFound(err))
}

func TestRev(t *testing.T) {
	c := newTestClient(t)
	db := c.DB("db")
	c.Handle("HEAD /db/ok", func(resp ResponseWriter, req *Request) {
		resp.Header().Set("ETag", `"1-619db7ba8551c0de3f3a178775509611"`)
	})
	c.Handle("HEAD /db/404", func(resp ResponseWriter, req *Request) {
		NotFound(resp, req)
	})

	rev, err := db.Rev("ok")
	if err != nil {
		t.Fatal(err)
	}
	check(t, "rev", "1-619db7ba8551c0de3f3a178775509611", rev)

	errorRev, err := db.Rev("404")
	check(t, "errorRev", "", errorRev)
	check(t, "couchdb.NotFound(err)", true, couchdb.NotFound(err))
	if _, ok := err.(*couchdb.Error); !ok {
		t.Errorf("expected couchdb.Error, got %#+v", err)
	}
}

func TestPut(t *testing.T) {
	c := newTestClient(t)
	c.Handle("PUT /db/doc", func(resp ResponseWriter, req *Request) {
		body, _ := ioutil.ReadAll(req.Body)
		check(t, "request body", `{"field":999}`, string(body))

		resp.Header().Set("ETag", `"1-619db7ba8551c0de3f3a178775509611"`)
		resp.WriteHeader(StatusCreated)
		io.WriteString(resp, `{
			"id": "doc",
			"ok": true,
			"rev": "1-619db7ba8551c0de3f3a178775509611"
		}`)
	})

	doc := &testDocument{Field: 999}
	rev, err := c.DB("db").Put("doc", doc, "")
	if err != nil {
		t.Fatal(err)
	}
	check(t, "returned rev", "1-619db7ba8551c0de3f3a178775509611", rev)
}

func TestDelete(t *testing.T) {
	c := newTestClient(t)
	c.Handle("DELETE /db/doc", func(resp ResponseWriter, req *Request) {
		check(t, "request query string",
			"rev=1-619db7ba8551c0de3f3a178775509611",
			req.URL.RawQuery)

		resp.Header().Set("ETag", `"2-619db7ba8551c0de3f3a178775509611"`)
		resp.WriteHeader(StatusOK)
		io.WriteString(resp, `{
			"id": "doc",
			"ok": true,
			"rev": "2-619db7ba8551c0de3f3a178775509611"
		}`)
	})

	delrev := "1-619db7ba8551c0de3f3a178775509611"
	if rev, err := c.DB("db").Delete("doc", delrev); err != nil {
		t.Fatal(err)
	} else {
		check(t, "returned rev", "2-619db7ba8551c0de3f3a178775509611", rev)
	}
}

func check(t *testing.T, field string, expected, actual interface{}) {
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("%s mismatch: want %#v, got %#v", field, expected, actual)
	}
}
