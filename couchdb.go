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
	auth   *auth
}

type auth struct {
	username, password string
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
func (opts Options) encode() (string, error) {
	buf := new(bytes.Buffer)
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
				return "", err
			}
			buf.WriteString(url.QueryEscape(string(jsonv)))
		}
		amp = true
	}
	return buf.String(), nil
}

func path(segs ...string) (r string) {
	for _, seg := range segs {
		r += "/"
		r += url.QueryEscape(seg)
	}
	return
}

func optpath(opts Options, segs ...string) (r string, err error) {
	r = path(segs...)
	if len(opts) > 0 {
		if os, err := opts.encode(); err == nil {
			r += os
		}
	}
	return
}

func (srv *Server) newRequest(
	method, path string,
	body io.Reader,
) (*http.Request, error) {
	req, err := http.NewRequest(method, srv.prefix+path, body)
	if err != nil {
		return nil, err
	}
	if srv.auth != nil {
		req.SetBasicAuth(srv.auth.username, srv.auth.password)
	}
	return req, nil
}

// request sends an HTTP request to a CouchDB server.
// The request URL is constructed from the server's
// prefix and the given path, which may contain an
// encoded query string.
//
// Status codes >= 400 are treated as errors.
func (srv *Server) request(
	method, path string,
	body io.Reader,
) (*http.Response, error) {
	req, err := srv.newRequest(method, path, body)
	if err != nil {
		return nil, err
	}
	resp, err := srv.http.Do(req)
	if err != nil {
		return nil, err
	} else if resp.StatusCode >= 400 {
		return nil, dbError(resp) // the Body is closed by dbError
	} else {
		return resp, nil
	}
}

// closedRequest sends a for-effect HTTP request.
func (srv *Server) closedRequest(
	method, path string,
	body io.Reader,
) (*http.Response, error) {
	resp, err := srv.request(method, path, body)
	if err == nil {
		resp.Body.Close()
	}
	return resp, err
}

// URL returns the URL prefix of the server.
//
// The prefix does not contain a trailing '/'.
func (srv *Server) URL() string {
	return srv.prefix
}

// Ping can be used to check whether a server is alive.
// It sends an HTTP HEAD request to the server's URL.
func (srv *Server) Ping() error {
	req, err := srv.newRequest("HEAD", "/", nil)
	if err != nil {
		return err
	}
	_, err = srv.http.Do(req)
	return err
}

// Login initiates a user session.
//
// Any requests made after a successful call to
// Login will be authenticated.
func (srv *Server) Login(username, password string) error {
	req, err := srv.newRequest("GET", "/_session", nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(username, password)
	if resp, err := srv.http.Do(req); err != nil {
		return err
	} else if resp.StatusCode >= 400 {
		return dbError(resp)
	} else {
		srv.auth = &auth{username, password}
		resp.Body.Close()
		return nil
	}
}

// Logout deletes the active session.
func (srv *Server) Logout() error {
	srv.auth = nil
	return nil
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
	if _, err = srv.closedRequest("GET", path(dbname), nil); err == nil {
		db = srv.Db(dbname)
	}
	return
}

// Create a new database on the given server.
func (srv *Server) CreateDb(dbname string) (db *Database, err error) {
	if _, err = srv.closedRequest("PUT", path(dbname), nil); err == nil {
		db = srv.Db(dbname)
	}
	return
}

func (srv *Server) DeleteDb(dbname string) error {
	_, err := srv.closedRequest("DELETE", path(dbname), nil)
	return err
}

func (srv *Server) AllDbs() (names []string, err error) {
	resp, err := srv.request("GET", "/_all_dbs", nil)
	if err == nil {
		err = readBody(resp, &names)
	}
	return
}

type Database struct {
	name string
	srv  *Server
}

type DbSecurity struct {
	Admins  DbMembers `json:"admins"`
	Members DbMembers `json:"members"`
}

type DbMembers struct {
	Names []string `json:"names,omitempty"`
	Roles []string `json:"roles,omitempty"`
}

// Name returns the database's name
func (db *Database) Name() string {
	return db.name
}

func (db *Database) Security() (*DbSecurity, error) {
	secobj := new(DbSecurity)
	resp, err := db.srv.request("GET", path(db.name, "_security"), nil)
	if err != nil {
		return nil, err
	}
	if resp.ContentLength == 0 {
		// empty reply means defaults
		return secobj, nil
	}
	if err = readBody(resp, secobj); err != nil {
		return nil, err
	}
	return secobj, nil
}

func (db *Database) SetSecurity(secobj *DbSecurity) error {
	json, _ := json.Marshal(secobj)
	body := bytes.NewReader(json)
	_, err := db.srv.request("PUT", path(db.name, "_security"), body)
	return err
}

// Retrieve a document from the given database.
func (db *Database) Get(id string, opts Options, doc interface{}) error {
	path, err := optpath(opts, db.name, id)
	if err != nil {
		return err
	}
	resp, err := db.srv.request("GET", path, nil)
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
		b := bytes.NewReader(json)
		return responseRev(db.srv.closedRequest("PUT", path(db.name, id), b))
	}
}

func (db *Database) PutRev(id, rev string, doc interface{}) (string, error) {
	if json, err := json.Marshal(doc); err != nil {
		return "", err
	} else {
		path, _ := optpath(Options{"rev": rev}, db.name, id)
		b := bytes.NewReader(json)
		return responseRev(db.srv.closedRequest("PUT", path, b))
	}
}

func (db *Database) Delete(id, rev string) (string, error) {
	path, _ := optpath(Options{"rev": rev}, db.name, id)
	return responseRev(db.srv.closedRequest("DELETE", path, nil))
}

// responseRev returns the unquoted Etag of a response.
func responseRev(resp *http.Response, err error) (string, error) {
	if err != nil {
		return "", nil
	} else if etag := resp.Header.Get("Etag"); etag == "" {
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
	path, err := optpath(opts, db.name, "_design", ddoc, "_view", view)
	if err != nil {
		return err
	}
	resp, err := db.srv.request("GET", path, nil)
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
	return ErrorStatus(err, http.StatusNotFound)
}

// Convenience function that checks whether the given error
// is a DatabaseError with StatusCode == 401.
func Unauthorized(err error) bool {
	return ErrorStatus(err, http.StatusUnauthorized)
}

// Convenience function that checks whether the given error
// is a DatabaseError with StatusCode == 409.
func Conflict(err error) bool {
	return ErrorStatus(err, http.StatusConflict)
}

// ErrorStatus checks whether the given error
// is a DatabaseError with a matching statusCode.
func ErrorStatus(err error, statusCode int) bool {
	dberr, ok := err.(DatabaseError)
	return ok && dberr.StatusCode == statusCode
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
