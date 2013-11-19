// Package couchdb implements wrappers for the CouchDB HTTP API.
//
// Unless otherwise noted, all functions in this package
// can be called from more than one goroutine at the same time.
package couchdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Server struct {
	prefix string // URL prefix
	http   *http.Client
}

// NewServer creates a new server object.
//
// The second argument can be nil to use the default
// http.RoundTripper, which should be good enough in most cases.
func NewServer(url string, transport http.RoundTripper) *Server {
	return &Server{
		prefix: strings.TrimRight(url, "/"),
		http:   &http.Client{Transport: transport},
	}
}

func (srv *Server) url(options Options, path ...string) (string, error) {
	buf := new(bytes.Buffer)
	buf.WriteString(srv.prefix)
	for _, p := range path {
		buf.WriteRune('/')
		buf.WriteString(url.QueryEscape(p))
	}
	if len(options) > 0 {
		if err := options.writeTo(buf); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

// CouchDB query string values are URL-encoded JSON.
type Options map[string]interface{}

// clone creates a shallow copy of an Options map
func (opts Options) clone() (result Options) {
	result = make(Options)
	for k, v := range opts {
		result[k] = v
	}
	return
}

// query encodes an Options map as an URL query string
func (opts Options) writeTo(buf *bytes.Buffer) error {
	buf.WriteRune('?')
	amp := false
	for k, v := range opts {
		if amp {
			buf.WriteRune('&')
		}
		buf.WriteString(url.QueryEscape(k))
		buf.WriteRune('=')

		if strval, ok := v.(string); ok {
			buf.WriteString(url.QueryEscape(strval))
		} else {
			jsonv, err := json.Marshal(v)
			if err != nil {
				return err
			}
			buf.WriteString(url.QueryEscape(string(jsonv)))
		}
		amp = true
	}
	return nil
}

// request sends an HTTP request to a CouchDB server.
// The request URL is constructed from the path segments
// and the encoded query string.
// Status codes >= 400 are treated as errors.
func (srv *Server) request(
	method string,
	body io.Reader,
	options Options,
	path ...string,
) (*http.Response, error) {
	url, err := srv.url(options, path...)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	resp, err := srv.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, dbError(resp) // the Body is closed by dbError
	}
	return resp, nil
}

// closedRequest sends a for-effect HTTP request.
func (srv *Server) closedRequest(
	method string,
	body io.Reader,
	options Options,
	path ...string,
) (*http.Response, error) {
	resp, err := srv.request(method, body, options, path...)
	if err == nil {
		resp.Body.Close()
	}
	return resp, err
}

// Ping can be used to check whether a server is alive.
// It sends an HTTP HEAD request to the server's URL.
func (srv *Server) Ping() error {
	_, err := srv.http.Head(srv.prefix + "/")
	return err
}

// Db returns a database object attached to the given
// server. No HTTP request is performed, so it is unclear
// whether the database actually exists.
func (srv *Server) Db(dbname string) *Database {
	return &Database{srv: srv, name: dbname}
}

// Check whether the given database exists on the server.
func (srv *Server) OpenDb(dbname string) (db *Database, err error) {
	// maybe HEAD would be more appropriate
	if _, err = srv.closedRequest("GET", nil, nil, dbname); err == nil {
		db = srv.Db(dbname)
	}
	return
}

// Create a new database on the given server.
func (srv *Server) CreateDb(dbname string) (db *Database, err error) {
	if _, err = srv.closedRequest("PUT", nil, nil, dbname); err == nil {
		db = srv.Db(dbname)
	}
	return
}

func (srv *Server) DeleteDb(dbname string) error {
	_, err := srv.closedRequest("DELETE", nil, nil, dbname)
	return err
}

func (srv *Server) AllDbs() (names []string, err error) {
	resp, err := srv.request("GET", nil, nil, "_all_dbs")
	if err == nil {
		err = readBody(resp, &names)
	}
	return
}

type Database struct {
	name string
	srv  *Server
}

// Name returns the database's name
func (db *Database) Name() string {
	return db.name
}

// Retrieve a document from the given database.
func (db *Database) Get(id string, opts Options, doc interface{}) error {
	resp, err := db.srv.request("GET", nil, opts, db.name, id)
	if err != nil {
		return err
	}
	return readBody(resp, &doc)
}

// Store a document into the given database.
func (db *Database) Put(id string, doc interface{}) (string, error) {
	if json, err := json.Marshal(doc); err != nil {
		return "", err
	} else {
		reader := bytes.NewReader(json)
		resp, err := db.srv.closedRequest("PUT", reader, nil, db.name, id)
		if err != nil {
			return "", err
		}
		return responseRev(resp)
	}
}

func (db *Database) PutRev(id, rev string, doc interface{}) (string, error) {
	if json, err := json.Marshal(doc); err != nil {
		return "", err
	} else {
		opts := Options{"rev": rev}
		reader := bytes.NewReader(json)
		resp, err := db.srv.closedRequest("PUT", reader, opts, db.name, id)
		if err != nil {
			return "", err
		}
		return responseRev(resp)
	}
}

func (db *Database) Delete(id, rev string) (string, error) {
	opts := Options{"rev": rev}
	resp, err := db.srv.closedRequest("DELETE", nil, opts, db.name, id)
	if err != nil {
		return "", err
	}
	return responseRev(resp)
}

// responseRev returns the unquoted Etag of a response.
func responseRev(resp *http.Response) (string, error) {
	if etag := resp.Header.Get("Etag"); etag == "" {
		return "", fmt.Errorf("no Etag in response")
	} else {
		return etag[1 : len(etag)-1], nil
	}
}

func (db *Database) Query(
	ddoc, view string,
	opts Options,
	doc interface{},
) error {
	resp, err :=
		db.srv.request("GET", nil, opts, db.name, "_design", ddoc, "_view", view)
	if err != nil {
		return err
	}
	return readBody(resp, &doc)
}

// Errors of this type are returned for API-level errors,
// i.e. for all errors that are reported by CouchDB as
//    {"error": <ErrorCode>, "reason": <Reason>}
type DatabaseError struct {
	Method     string // HTTP method of the request
	URL        string // HTTP URL of the request
	StatusCode int    // HTTP status code of the response
	ErrorCode  string // Error reason provided by CouchDB
	Reason     string // Error message provided by CouchDB
}

func (e DatabaseError) Error() string {
	return fmt.Sprintf("%v %v: (%v) %v: %v",
		e.Method, e.URL, e.StatusCode, e.ErrorCode, e.Reason)
}

// Convenience function that checks whether the given error
// is a DatabaseError with StatusCode == 404. This is useful
// for conditional creation of databases and documents.
func NotFound(err error) bool {
	dberr, ok := err.(DatabaseError)
	return ok && dberr.StatusCode == 404
}

func dbError(resp *http.Response) error {
	var reply struct{ Error, Reason string }
	if err := readBody(resp, &reply); err != nil {
		return fmt.Errorf("couldn't decode CouchDB error: %v", err)
	}

	return DatabaseError{
		Method:     resp.Request.Method,
		URL:        resp.Request.URL.String(),
		StatusCode: resp.StatusCode,
		ErrorCode:  reply.Error,
		Reason:     reply.Reason,
	}
}

func readBody(resp *http.Response, v interface{}) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(&v)
}
