package couchdb

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
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

type transport struct {
	prefix string // URL prefix
	http   *http.Client
	mu     sync.RWMutex
	auth   Auth
}

func newTransport(prefix string, rt http.RoundTripper, auth Auth) *transport {
	return &transport{
		prefix: strings.TrimRight(prefix, "/"),
		http:   &http.Client{Transport: rt},
		auth:   auth,
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
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}

	resp, err := t.http.Do(req)
	if err != nil {
		return nil, err
	} else if resp.StatusCode >= 400 {
		return nil, parseError(req, resp) // the Body is closed by parseError
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

// pathBuilder assists with constructing CouchDB request paths.
type pathBuilder struct {
	buf     bytes.Buffer
	inQuery bool
}

// dbpath returns the root path to a database.
func dbpath(name string) string {
	// TODO: would be nice to use url.PathEscape here,
	// but it only became available in Go 1.8.
	return "/" + url.QueryEscape(name)
}

// path returns the built path.
func (p *pathBuilder) path() string {
	return p.buf.String()
}

func (p *pathBuilder) checkNotInQuery() {
	if p.inQuery {
		panic("can't add path elements after query string")
	}
}

// docID adds a document ID to the path.
func (p *pathBuilder) docID(id string) *pathBuilder {
	p.checkNotInQuery()

	if len(id) > 0 && id[0] != '_' {
		// Normal document IDs can't start with _, only 'reserved' document IDs can.
		p.add(id)
		return p
	}
	// However, it is still useful to be able to retrieve reserved documents such as
	// design documents (path: _design/doc). Avoid escaping the first '/', but do escape
	// anything after that.
	slash := strings.IndexByte(id, '/')
	if slash == -1 {
		p.add(id)
		return p
	}
	p.addRaw(id[:slash])
	p.add(id[slash+1:])
	return p
}

// add adds a segment to the path.
func (p *pathBuilder) add(segment string) *pathBuilder {
	p.checkNotInQuery()
	p.buf.WriteByte('/')
	// TODO: would be nice to use url.PathEscape here,
	// but it only became available in Go 1.8.
	p.buf.WriteString(url.QueryEscape(segment))
	return p
}

// addRaw adds an unescaped segment to the path.
func (p *pathBuilder) addRaw(path string) *pathBuilder {
	p.checkNotInQuery()
	p.buf.WriteByte('/')
	p.buf.WriteString(path)
	return p
}

// rev adds a revision to the query string.
// It returns the built path.
func (p *pathBuilder) rev(rev string) string {
	p.checkNotInQuery()
	p.inQuery = true
	if rev != "" {
		p.buf.WriteString("?rev=")
		p.buf.WriteString(url.QueryEscape(rev))
	}
	return p.path()
}

// options encodes the given options to the query.
func (p *pathBuilder) options(opts Options, jskeys []string) (string, error) {
	p.checkNotInQuery()
	p.inQuery = true

	// Sort keys by name.
	var keys = make([]string, len(opts))
	var i int
	for k := range opts {
		keys[i] = k
		i++
	}
	sort.Strings(keys)

	// Encode to query string.
	p.buf.WriteByte('?')
	amp := false
	for _, k := range keys {
		if amp {
			p.buf.WriteByte('&')
		}
		p.buf.WriteString(url.QueryEscape(k))
		p.buf.WriteByte('=')
		isjson := false
		for _, jskey := range jskeys {
			if k == jskey {
				isjson = true
				break
			}
		}
		if isjson {
			jsonv, err := json.Marshal(opts[k])
			if err != nil {
				return "", fmt.Errorf("invalid option %q: %v", k, err)
			}
			p.buf.WriteString(url.QueryEscape(string(jsonv)))
		} else {
			if err := encval(&p.buf, k, opts[k]); err != nil {
				return "", fmt.Errorf("invalid option %q: %v", k, err)
			}
		}
		amp = true
	}
	return p.path(), nil
}

func encval(w io.Writer, k string, v interface{}) error {
	if v == nil {
		return errors.New("value is nil")
	}
	rv := reflect.ValueOf(v)
	var str string
	switch rv.Kind() {
	case reflect.String:
		str = url.QueryEscape(rv.String())
	case reflect.Bool:
		str = strconv.FormatBool(rv.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		str = strconv.FormatInt(rv.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		str = strconv.FormatUint(rv.Uint(), 10)
	case reflect.Float32:
		str = strconv.FormatFloat(rv.Float(), 'f', -1, 32)
	case reflect.Float64:
		str = strconv.FormatFloat(rv.Float(), 'f', -1, 64)
	default:
		return fmt.Errorf("unsupported type: %s", rv.Type())
	}
	_, err := io.WriteString(w, str)
	return err
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
	}
	return fmt.Sprintf("%v %v: (%v) %v: %v",
		e.Method, e.URL, e.StatusCode, e.ErrorCode, e.Reason)
}

// NotFound checks whether the given errors is a DatabaseError
// with StatusCode == 404. This is useful for conditional creation
// of databases and documents.
func NotFound(err error) bool {
	return ErrorStatus(err, http.StatusNotFound)
}

// Unauthorized checks whether the given error is a DatabaseError
// with StatusCode == 401.
func Unauthorized(err error) bool {
	return ErrorStatus(err, http.StatusUnauthorized)
}

// Conflict checks whether the given error is a DatabaseError
// with StatusCode == 409.
func Conflict(err error) bool {
	return ErrorStatus(err, http.StatusConflict)
}

// ErrorStatus checks whether the given error is a DatabaseError
// with a matching statusCode.
func ErrorStatus(err error, statusCode int) bool {
	dberr, ok := err.(*Error)
	return ok && dberr.StatusCode == statusCode
}

func parseError(req *http.Request, resp *http.Response) error {
	var reply struct{ Error, Reason string }
	if req.Method != "HEAD" {
		if err := readBody(resp, &reply); err != nil {
			return fmt.Errorf("couldn't decode CouchDB error: %v", err)
		}
	}
	return &Error{
		Method:     req.Method,
		URL:        req.URL.String(),
		StatusCode: resp.StatusCode,
		ErrorCode:  reply.Error,
		Reason:     reply.Reason,
	}
}
