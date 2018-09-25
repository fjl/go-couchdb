// Package couchdb implements wrappers for the CouchDB HTTP API.
//
// Unless otherwise noted, all functions in this package
// can be called from more than one goroutine at the same time.
package couchdb

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// Client represents a remote CouchDB server.
type Client struct {
	*transport
	ctx context.Context
}

// NewClient creates a new Client
// addr should contain scheme and host, and optionally port and path. All other attributes will be ignored
// If client is nil, default http.Client will be used
// If auth is nil, no auth will be set
func NewClient(addr *url.URL, client *http.Client, auth Auth) *Client {
	prefixAddr := *addr
	// cleanup our address
	prefixAddr.User, prefixAddr.RawQuery, prefixAddr.Fragment = nil, "", ""
	return &Client{newTransport(prefixAddr.String(), client, auth), context.Background()}
}

// WithContext returns a copy of the Client with the new context set.
func (c *Client) WithContext(ctx context.Context) *Client {
	c2 := new(Client)
	*c2 = *c
	c2.ctx = ctx
	return c2
}

// Context provides the current context for the Client.
func (c *Client) Context() context.Context {
	return c.ctx
}

// URL returns the URL prefix of the server.
// The url will not contain a trailing '/'.
func (c *Client) URL() string {
	return c.prefix
}

// Ping can be used to check whether a server is alive.
// It sends an HTTP HEAD request to the server's URL.
func (c *Client) Ping() error {
	_, err := c.closedRequest(c.ctx, "HEAD", "/", nil)
	return err
}

// SetAuth sets the authentication mechanism used by the client.
// Use SetAuth(nil) to unset any mechanism that might be in use.
// In order to verify the credentials against the server, issue any request
// after the call the SetAuth.
func (c *Client) SetAuth(a Auth) {
	c.transport.setAuth(a)
}

// CreateDB creates a new database.
// The request will fail with status "412 Precondition Failed" if the database
// already exists. A valid DB object is returned in all cases, even if the
// request fails.
func (c *Client) CreateDB(name string) (*DB, error) {
	if _, err := c.closedRequest(c.ctx, "PUT", path(name), nil); err != nil {
		return c.DB(name), err
	}
	return c.DB(name), nil
}

// CreateDBWithShards creates a new database with the specified number of shards
func (c *Client) CreateDBWithShards(name string, shards int) (*DB, error) {
	_, err := c.closedRequest(c.ctx, "PUT", fmt.Sprintf("%s?q=%d", path(name), shards), nil)

	return c.DB(name), err
}

// EnsureDB ensures that a database with the given name exists.
func (c *Client) EnsureDB(name string) (*DB, error) {
	db, err := c.CreateDB(name)
	if err != nil && !ErrorStatus(err, http.StatusPreconditionFailed) {
		return nil, err
	}
	return db, nil
}

// DeleteDB deletes an existing database.
func (c *Client) DeleteDB(name string) error {
	_, err := c.closedRequest(c.ctx, "DELETE", path(name), nil)
	return err
}

// AllDBs returns the names of all existing databases.
func (c *Client) AllDBs() (names []string, err error) {
	resp, err := c.request(c.ctx, "GET", "/_all_dbs", nil)
	if err != nil {
		return names, err
	}
	err = readBody(resp, &names)
	return names, err
}

// DB creates a database object.
// The database inherits the authentication and http.RoundTripper
// of the client. The database's actual existence is not verified.
func (c *Client) DB(name string) *DB {
	return &DB{c.transport, name, c.ctx}
}
