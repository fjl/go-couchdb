package couchdb_test

import (
	"github.com/fjl/go-couchdb"
	"net/http"
	"testing"
)

func TestBasicAuth(t *testing.T) {
	tests := []struct{ username, password, header string }{
		{"", "", "Basic Og=="},
		{"user", "", "Basic dXNlcjo="},
		{"", "password", "Basic OnBhc3N3b3Jk"},
		{"user", "password", "Basic dXNlcjpwYXNzd29yZA=="},
	}

	for _, test := range tests {
		req, _ := http.NewRequest("GET", "http://localhost/", nil)
		auth := couchdb.BasicAuth(test.username, test.password)
		auth.AddAuth(req)

		expected := http.Header{"Authorization": {test.header}}
		check(t, "req headers", expected, req.Header)
	}
}

func TestProxyAuthWithoutToken(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://localhost/", nil)
	auth := couchdb.ProxyAuth("user", []string{"role1", "role2"}, "")
	auth.AddAuth(req)

	expected := http.Header{
		"X-Auth-Couchdb-Username": {"user"},
		"X-Auth-Couchdb-Roles":    {"role1,role2"},
	}
	check(t, "req headers", expected, req.Header)
}

func TestProxyAuthWithToken(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://localhost/", nil)
	auth := couchdb.ProxyAuth("user", []string{"role1", "role2"}, "secret")
	auth.AddAuth(req)

	expected := http.Header{
		"X-Auth-Couchdb-Username": {"user"},
		"X-Auth-Couchdb-Roles":    {"role1,role2"},
		"X-Auth-Couchdb-Token":    {"027da48c8c642ca4c58eb982eec81915179e77a3"},
	}
	check(t, "req headers", expected, req.Header)
}
