package couchdb

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

// Auth is implemented by HTTP authentication mechanisms.
type Auth interface {
	// AddAuth should add authentication information (e.g. headers)
	// to the given http request.
	AddAuth(*http.Request)
}

type basicauth string

func BasicAuth(username, password string) Auth {
	auth := []byte(username + ":" + password)
	hdr := "Basic " + base64.StdEncoding.EncodeToString(auth)
	return basicauth(hdr)
}

func (a basicauth) AddAuth(req *http.Request) {
	req.Header.Set("Authorization", string(a))
}

type proxyauth struct {
	username, roles, tok string
}

func ProxyAuth(username string, roles []string, secret string) Auth {
	pa := &proxyauth{username, strings.Join(roles, ","), ""}
	if secret != "" {
		hash := sha1.New()
		hash.Write([]byte(secret + username))
		pa.tok = fmt.Sprintf("%x", hash.Sum(nil))
	}
	return pa
}

func (a proxyauth) AddAuth(req *http.Request) {
	req.Header.Set("X-Auth-CouchDB-UserName", a.username)
	req.Header.Set("X-Auth-CouchDB-Roles", a.roles)
	if a.tok != "" {
		req.Header.Set("X-Auth-CouchDB-Token", a.tok)
	}
}
