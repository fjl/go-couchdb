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

type Client struct {
	prefix string // URL prefix
	http   *http.Client
	auth   *auth
}

type auth struct {
	username, password string
}

// NewClient creates a new client object.
//
// The second argument can be nil to use the default
// http.RoundTripper, which should be good enough in most cases.
func NewClient(url string, transport http.RoundTripper) *Client {
	return &Client{
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

func (c *Client) newRequest(
	method, path string,
	body io.Reader,
) (*http.Request, error) {
	req, err := http.NewRequest(method, c.prefix+path, body)
	if err != nil {
		return nil, err
	}
	if c.auth != nil {
		req.SetBasicAuth(c.auth.username, c.auth.password)
	}
	return req, nil
}

// request sends an HTTP request to a CouchDB server.
// The request URL is constructed from the server's
// prefix and the given path, which may contain an
// encoded query string.
//
// Status codes >= 400 are treated as errors.
func (c *Client) request(
	method, path string,
	body io.Reader,
) (*http.Response, error) {
	req, err := c.newRequest(method, path, body)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	} else if resp.StatusCode >= 400 {
		return nil, dbError(resp) // the Body is closed by dbError
	} else {
		return resp, nil
	}
}

// closedRequest sends a for-effect HTTP request.
func (c *Client) closedRequest(
	method, path string,
	body io.Reader,
) (*http.Response, error) {
	resp, err := c.request(method, path, body)
	if err == nil {
		resp.Body.Close()
	}
	return resp, err
}

// URL returns the URL prefix of the server.
// The prefix does not contain a trailing '/'.
func (c *Client) URL() string {
	return c.prefix
}

// Ping can be used to check whether a server is alive.
// It sends an HTTP HEAD request to the server's URL.
func (c *Client) Ping() error {
	_, err := c.closedRequest("HEAD", "/", nil)
	return err
}

// Login initiates a user session.
// Any requests made after a successful call to Login will be authenticated.
func (c *Client) Login(username, password string) error {
	req, err := c.newRequest("GET", "/_session", nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(username, password)
	if resp, err := c.http.Do(req); err != nil {
		return err
	} else if resp.StatusCode >= 400 {
		return dbError(resp)
	} else {
		c.auth = &auth{username, password}
		resp.Body.Close()
		return nil
	}
}

// Logout deletes the active session.
func (c *Client) Logout() error {
	c.auth = nil
	return nil
}

// CreateDb creates a new database.
// The request will fail with status "412 Precondition Failed" if
// the database already exists.
func (c *Client) CreateDb(dbname string) error {
	_, err := c.closedRequest("PUT", path(dbname), nil)
	return err
}

func (c *Client) DeleteDb(dbname string) error {
	_, err := c.closedRequest("DELETE", path(dbname), nil)
	return err
}

func (c *Client) AllDbs() (names []string, err error) {
	resp, err := c.request("GET", "/_all_dbs", nil)
	if err == nil {
		err = readBody(resp, &names)
	}
	return
}

type DbSecurity struct {
	Admins  DbMembers `json:"admins"`
	Members DbMembers `json:"members"`
}

type DbMembers struct {
	Names []string `json:"names,omitempty"`
	Roles []string `json:"roles,omitempty"`
}

// Security retrieves the database security object, which defines its
// access control rules.
func (c *Client) Security(db string) (*DbSecurity, error) {
	secobj := new(DbSecurity)
	resp, err := c.request("GET", path(db, "_security"), nil)
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

// SetSecurity sets the database security object.
func (c *Client) SetSecurity(db string, secobj *DbSecurity) error {
	json, _ := json.Marshal(secobj)
	body := bytes.NewReader(json)
	_, err := c.request("PUT", path(db, "_security"), body)
	return err
}

// Get retrieves a document from the given database.
// The document is unmarshalled into the given object.
// Some fields (like _conflicts) will only be returned if the
// options require it. Please refer to the CouchDB HTTP API documentation
// for more information.
func (c *Client) Get(db, id string, opts Options, doc interface{}) error {
	path, err := optpath(opts, db, id)
	if err != nil {
		return err
	}
	resp, err := c.request("GET", path, nil)
	if err != nil {
		return err
	}
	return readBody(resp, &doc)
}

// Rev fetches the current revision of a document.
// It is faster than an equivalent Get request because no body has to parsed.
func (c *Client) Rev(db, id string) (string, error) {
	return responseRev(c.closedRequest("HEAD", path(db, id), nil))
}

// Put stores a document into the given database.
// If the document is already present in the database, the
// marshalled JSON representation of doc must include a _rev member
// or the request will fail with "409 Conflict".
func (c *Client) Put(db, id string, doc interface{}) (string, error) {
	if json, err := json.Marshal(doc); err != nil {
		return "", err
	} else {
		b := bytes.NewReader(json)
		return responseRev(c.closedRequest("PUT", path(db, id), b))
	}
}

// PutRev stores a document into the given database.
// In contrast to the Put method, the current revision must be
// given explicitly, which can be useful if your document representation
// does not include a _rev member.
func (c *Client) PutRev(db, id, rev string, doc interface{}) (string, error) {
	if json, err := json.Marshal(doc); err != nil {
		return "", err
	} else {
		path, _ := optpath(Options{"rev": rev}, db, id)
		b := bytes.NewReader(json)
		return responseRev(c.closedRequest("PUT", path, b))
	}
}

// Delete marks a document revision as deleted.
func (c *Client) Delete(db, id, rev string) (string, error) {
	path, _ := optpath(Options{"rev": rev}, db, id)
	return responseRev(c.closedRequest("DELETE", path, nil))
}

// responseRev returns the unquoted Etag of a response.
func responseRev(resp *http.Response, err error) (string, error) {
	if err != nil {
		return "", err
	} else if etag := resp.Header.Get("Etag"); etag == "" {
		return "", fmt.Errorf("no Etag in response")
	} else {
		return etag[1 : len(etag)-1], nil
	}
}

// Query retrieves the contents of a view.
// The output of the query is unmarshalled into the given viewResult.
// The format of the result depends on the options. Please
// refer to the CouchDB HTTP API documentation for all the possible
// options that can be set.
func (c *Client) Query(
	db, ddoc, view string,
	opts Options,
	viewResult interface{},
) error {
	path, err := optpath(opts, db, "_design", ddoc, "_view", view)
	if err != nil {
		return err
	}
	resp, err := c.request("GET", path, nil)
	if err != nil {
		return err
	}
	return readBody(resp, &viewResult)
}

// Attachment represents document attachments.
type Attachment struct {
	Name string
	Type string // the MIME type name of the Body
	Body io.Reader
}

// PutAttachment creates or updates a document attachment.
// To create an attachment on a non-existing document, pass an empty
// string as the rev.
func (c *Client) PutAttachment(
	db, docid, rev string,
	att *Attachment,
) (newrev string, err error) {
	if docid == "" {
		return "", fmt.Errorf("couchdb.PutAttachment: empty docid")
	}
	if att.Name == "" {
		return "", fmt.Errorf("couchdb.PutAttachment: empty filename")
	}

	// create the request
	var p string
	if rev == "" {
		p = path(db, docid, att.Name)
	} else {
		p, _ = optpath(Options{"rev": rev}, db, docid, att.Name)
	}
	req, err := c.newRequest("PUT", p, att.Body)
	if err != nil {
		return "", err
	}
	if att.Type != "" {
		req.Header.Add("Content-Type", att.Type)
	}

	// execute it
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	} else if resp.StatusCode >= 400 {
		return "", dbError(resp) // the Body is closed by dbError
	} else {
		resp.Body.Close()
		return responseRev(resp, nil)
	}
}

// Errors of this type are returned for API-level errors,
// i.e. for all errors that are reported by CouchDB as
//    {"error": <ErrorCode>, "reason": <Reason>}
type DatabaseError struct {
	Method     string // HTTP method of the request
	URL        string // HTTP URL of the request
	StatusCode int    // HTTP status code of the response

	// These two fields will be empty for HEAD requests.
	ErrorCode string // Error reason provided by CouchDB
	Reason    string // Error message provided by CouchDB
}

func (e DatabaseError) Error() string {
	if e.ErrorCode == "" {
		return fmt.Sprintf("%v %v: %v", e.Method, e.URL, e.StatusCode)
	} else {
		return fmt.Sprintf("%v %v: (%v) %v: %v",
			e.Method, e.URL, e.StatusCode, e.ErrorCode, e.Reason)
	}
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
	if resp.Request.Method != "HEAD" {
		if err := readBody(resp, &reply); err != nil {
			return fmt.Errorf("couldn't decode CouchDB error: %v", err)
		}
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
