package couchdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// Options represents CouchDB query string parameters.
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

type transport struct {
	prefix string // URL prefix
	http   *http.Client
	mu     sync.RWMutex
	auth   Auth
}

func newTransport(prefix string, rt http.RoundTripper) *transport {
	return &transport{
		prefix: strings.TrimRight(prefix, "/"),
		http:   &http.Client{Transport: rt},
	}
}

func (t *transport) setAuth(a Auth) {
	t.mu.Lock()
	t.auth = a
	t.mu.Unlock()
}

func (t *transport) newRequest(method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, t.prefix+path, body)
	if err != nil {
		return nil, err
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.auth != nil {
		t.auth.AddAuth(req)
	}
	return req, nil
}

// request sends an HTTP request to a CouchDB server.
// The request URL is constructed from the server's
// prefix and the given path, which may contain an
// encoded query string.
//
// Status codes >= 400 are treated as errors.
func (t *transport) request(method, path string, body io.Reader) (*http.Response, error) {
	req, err := t.newRequest(method, path, body)
	if err != nil {
		return nil, err
	}
	resp, err := t.http.Do(req)
	if err != nil {
		return nil, err
	} else if resp.StatusCode >= 400 {
		return nil, parseError(resp) // the Body is closed by parseError
	} else {
		return resp, nil
	}
}

// closedRequest sends an HTTP request and discards the response body.
func (t *transport) closedRequest(method, path string, body io.Reader) (*http.Response, error) {
	resp, err := t.request(method, path, body)
	if err == nil {
		resp.Body.Close()
	}
	return resp, err
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

// responseRev returns the unquoted Etag of a response.
func responseRev(resp *http.Response, err error) (string, error) {
	if err != nil {
		return "", err
	} else if etag := resp.Header.Get("Etag"); etag == "" {
		return "", fmt.Errorf("couchdb: missing Etag header in response")
	} else {
		return etag[1 : len(etag)-1], nil
	}
}

func readBody(resp *http.Response, v interface{}) error {
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		resp.Body.Close()
		return err
	}
	return resp.Body.Close()
}

// Error represents API-level errors, reported by CouchDB as
//    {"error": <ErrorCode>, "reason": <Reason>}
type Error struct {
	Method     string // HTTP method of the request
	URL        string // HTTP URL of the request
	StatusCode int    // HTTP status code of the response

	// These two fields will be empty for HEAD requests.
	ErrorCode string // Error reason provided by CouchDB
	Reason    string // Error message provided by CouchDB
}

func (e *Error) Error() string {
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
	dberr, ok := err.(*Error)
	return ok && dberr.StatusCode == statusCode
}

func parseError(resp *http.Response) error {
	var reply struct{ Error, Reason string }
	if resp.Request.Method != "HEAD" {
		if err := readBody(resp, &reply); err != nil {
			return fmt.Errorf("couldn't decode CouchDB error: %v", err)
		}
	}
	return &Error{
		Method:     resp.Request.Method,
		URL:        resp.Request.URL.String(),
		StatusCode: resp.StatusCode,
		ErrorCode:  reply.Error,
		Reason:     reply.Reason,
	}
}
