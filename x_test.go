// This file contains stuff that is used across all the tests.

package couchdb_test

import (
	"bytes"
	"github.com/cabify/go-couchdb"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
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
	handlers map[string]http.Handler
}

func (s *testClient) Handle(pat string, f func(http.ResponseWriter, *http.Request)) {
	s.handlers[pat] = http.HandlerFunc(f)
}

func (s *testClient) ClearHandlers() {
	s.handlers = make(map[string]http.Handler)
}

func (s *testClient) RoundTrip(req *http.Request) (*http.Response, error) {
	handler, ok := s.handlers[req.Method+" "+req.URL.Path]
	if !ok {
		s.t.Fatalf("unhandled request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	}
	recorder := httptest.NewRecorder()
	recorder.Body = new(bytes.Buffer)
	handler.ServeHTTP(recorder, req)
	resp := &http.Response{
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		StatusCode:    recorder.Code,
		Status:        http.StatusText(recorder.Code),
		Header:        recorder.HeaderMap,
		ContentLength: int64(recorder.Body.Len()),
		Body:          ioutil.NopCloser(recorder.Body),
		Request:       req,
	}
	return resp, nil
}

func newTestClient(t *testing.T) *testClient {
	tc := &testClient{t: t, handlers: make(map[string]http.Handler)}
	client := couchdb.NewClient(asURL("http://testClient:5984/"), &http.Client{Transport: tc}, nil)
	tc.Client = client
	return tc
}

func check(t *testing.T, field string, expected, actual interface{}) {
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("%s mismatch:\nwant %#v\ngot  %#v", field, expected, actual)
	}
}
