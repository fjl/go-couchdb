# What's this?

go-couchdb is yet another CouchDB client written in Go.
Forked from [github.com/cabify/go-couchdb](http://github.com/cabify/go-couchdb) but not compatible with it anymore.

This project contains three Go packages:

## package couchdb [![GoDoc](https://godoc.org/github.com/cabify/go-couchdb?status.png)](http://godoc.org/github.com/cabify/go-couchdb)

    import "github.com/cabify/go-couchdb"

This wraps the CouchDB HTTP API.

## package couchapp [![GoDoc](https://godoc.org/github.com/cabify/go-couchdb?status.png)](http://godoc.org/github.com/cabify/go-couchdb/couchapp)

    import "github.com/cabify/go-couchdb/couchapp"

This provides functionality similar to the original
[couchapp](https://github.com/couchapp/couchapp) tool,
namely compiling a filesystem directory into a JSON object
and storing the object as a CouchDB design document.

## package couchdaemon [![GoDoc](https://godoc.org/github.com/cabify/go-couchdb?status.png)](http://godoc.org/github.com/cabify/go-couchdb/couchdaemon)

    import "github.com/cabify/go-couchdb/couchdaemon"

This package contains some functions that help
you write Go programs that run as a daemon started by CouchDB,
e.g. fetching values from the CouchDB config.

# Tests

You can run the unit tests with `make test`.

[![Build Status](https://travis-ci.org/cabify/go-couchdb.png?branch=master)](https://travis-ci.org/cabify/go-couchdb)
