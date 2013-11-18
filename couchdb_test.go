package couchdb

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type testServer struct {
	*Server
	t        *testing.T
	handlers map[string]http.Handler
}

func (s *testServer) Handle(pat string, f func(http.ResponseWriter, *http.Request)) {
	s.handlers[pat] = http.HandlerFunc(f)
}

func (s *testServer) RoundTrip(req *http.Request) (*http.Response, error) {
	handler, ok := s.handlers[req.Method+" "+req.URL.Path]
	if !ok {
		s.t.Fatalf("unhandled request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	}
	recorder := httptest.NewRecorder()
	recorder.Body = new(bytes.Buffer)
	handler.ServeHTTP(recorder, req)
	resp := &http.Response{
		StatusCode:    recorder.Code, // TODO: other status fields
		Header:        recorder.HeaderMap,
		ContentLength: int64(recorder.Body.Len()),
		Body:          ioutil.NopCloser(recorder.Body),
		Request:       req,
	}
	return resp, nil
}

func newTestServer(t *testing.T) *testServer {
	srv := &testServer{t: t, handlers: make(map[string]http.Handler)}
	srv.Server = NewServer("http://testserver:5984/", srv)
	return srv
}

func TestPing(t *testing.T) {
	srv := newTestServer(t)
	srv.Handle("HEAD /", func(resp http.ResponseWriter, req *http.Request) {})

	if err := srv.Ping(); err != nil {
		t.Fatal(err)
	}
}

func TestDb(t *testing.T) {
	srv := newTestServer(t)
	db := srv.Db("db")
	check(t, "db.Name", "db", db.Name())
}

func TestCreateDb(t *testing.T) {
	srv := newTestServer(t)
	srv.Handle("PUT /db", func(resp http.ResponseWriter, req *http.Request) {})

	db, err := srv.CreateDb("db")
	if err != nil {
		t.Fatal(err)
	}
	check(t, "db.Name", "db", db.Name())
}

func TestOpenDb(t *testing.T) {
	srv := newTestServer(t)
	srv.Handle("GET /db", func(resp http.ResponseWriter, req *http.Request) {
		io.WriteString(resp, `
{"db_name":"db","doc_count":1,"doc_del_count":0,"update_seq":1,"purge_seq":0,"compact_running":false,"disk_size":8290,"data_size":2024,"instance_start_time":"1384798820687932","disk_format_version":6,"committed_update_seq":1}`)
	})

	db, err := srv.OpenDb("db")
	if err != nil {
		t.Fatal(err)
	}
	check(t, "db.Name", "db", db.Name())
}

func TestDeleteDb(t *testing.T) {
	srv := newTestServer(t)
	srv.Handle("DELETE /db", func(resp http.ResponseWriter, req *http.Request) {})
	if err := srv.DeleteDb("db"); err != nil {
		t.Fatal(err)
	}
}

func TestAllDbs(t *testing.T) {
	srv := newTestServer(t)
	srv.Handle("GET /_all_dbs", func(resp http.ResponseWriter, req *http.Request) {
		io.WriteString(resp, `["a","b","c"]`)
	})

	names, err := srv.AllDbs()
	if err != nil {
		t.Fatal(err)
	}
	check(t, "returned names", []string{"a", "b", "c"}, names)
}

type testDocument struct {
	Rev   string `json:"_rev,omitempty"`
	Field int64  `json:"field"`
}

func TestGetExistingDoc(t *testing.T) {
	srv := newTestServer(t)
	srv.Handle("GET /db/doc", func(resp http.ResponseWriter, req *http.Request) {
		io.WriteString(resp, `{"_id":"doc","_rev":"1-619db7ba8551c0de3f3a178775509611","field":999}`)
	})

	var doc testDocument
	if err := srv.Db("db").Get("doc", nil, &doc); err != nil {
		t.Fatal(err)
	}
	check(t, "doc.Rev", "1-619db7ba8551c0de3f3a178775509611", doc.Rev)
	check(t, "doc.Field", int64(999), doc.Field)
}

func TestGetNonexistingDoc(t *testing.T) {
	srv := newTestServer(t)
	srv.Handle("GET /db/doc", func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(404)
		io.WriteString(resp, `{"error":"not_found","reason":"error reason"}`)
	})

	var doc testDocument
	err := srv.Db("db").Get("doc", nil, doc)
	check(t, "NotFound(err)", true, NotFound(err))
}

func TestPut(t *testing.T) {
	srv := newTestServer(t)
	srv.Handle("PUT /db/doc", func(resp http.ResponseWriter, req *http.Request) {
		body, _ := ioutil.ReadAll(req.Body)
		check(t, "request body", `{"field":999}`, string(body))

		resp.Header().Set("ETag", `"1-619db7ba8551c0de3f3a178775509611"`)
		resp.WriteHeader(http.StatusCreated)
		io.WriteString(resp, `{"id":"doc","ok":true,"rev":"1-619db7ba8551c0de3f3a178775509611"}`)
	})

	doc := &testDocument{Field: 999}
	rev, err := srv.Db("db").Put("doc", doc)
	if err != nil {
		t.Fatal(err)
	}
	check(t, "returned rev", "1-619db7ba8551c0de3f3a178775509611", rev)
}

func TestDelete(t *testing.T) {
	srv := newTestServer(t)
	srv.Handle("DELETE /db/doc", func(resp http.ResponseWriter, req *http.Request) {
		check(t, "request query string", "rev=1-619db7ba8551c0de3f3a178775509611", req.URL.RawQuery)

		resp.Header().Set("ETag", `"2-619db7ba8551c0de3f3a178775509611"`)
		resp.WriteHeader(http.StatusOK)
		io.WriteString(resp, `{"id":"doc","ok":true,"rev":"2-619db7ba8551c0de3f3a178775509611"}`)
	})

	delrev := "1-619db7ba8551c0de3f3a178775509611"
	if rev, err := srv.Db("db").Delete("doc", delrev); err != nil {
		t.Fatal(err)
	} else {
		check(t, "returned rev", "2-619db7ba8551c0de3f3a178775509611", rev)
	}
}

func TestDbUpdatesFeed(t *testing.T) {
	srv := newTestServer(t)
	srv.Handle("GET /_db_updates", func(resp http.ResponseWriter, req *http.Request) {
		check(t, "request query string", "feed=continuous", req.URL.RawQuery)
		io.WriteString(resp, `{"db_name":"db","ok":true,"type":"created"}`+"\n")
	})

	feed, err := srv.Updates(nil)
	if err != nil {
		t.Fatal(err)
	}
	event, err := feed.Next()
	if err != nil {
		t.Fatal(err)
	}
	check(t, "event name", "db", event.Name)
	check(t, "event type", "created", event.Type)

	feed.Close()
}

func TestChangesFeed(t *testing.T) {
	srv := newTestServer(t)
	srv.Handle("GET /db/_changes", func(resp http.ResponseWriter, req *http.Request) {
		check(t, "request query string", "feed=continuous", req.URL.RawQuery)
		io.WriteString(resp, `{"seq":1,"id":"doc","changes":[{"rev":"1-619db7ba8551c0de3f3a178775509611"}]}`)
	})

	db := srv.Db("db")
	feed, err := db.Changes(nil)
	if err != nil {
		t.Fatal(err)
	}
	event, err := feed.Next()
	if err != nil {
		t.Fatal(err)
	}

	check(t, "event id", "doc", event.Id)
	check(t, "event seq", int64(1), event.Seq)
	check(t, "event database", db, event.Database)

	feed.Close()
}

func check(t *testing.T, field string, expected, actual interface{}) {
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("%s mismatch: want %#v, got %#v", field, expected, actual)
	}
}
