# What's this?

go-couchdb is yet another CouchDB client written in Go.
It was written because all the other ones didn't provide
functionality that I need.

The API is not fully baked at this time and may change.

This project contains two Go packages:

## package couchdb

    import "github.com/fjl/go-couchdb"

This wraps the CouchDB HTTP API.

## package couchapp

    import "github.com/fjl/go-couchdb/couchapp"

This provides functionality similar to the original
[couchapp](https://github.com/couchapp/couchapp) tool,
namely compiling a filesystem directory into a JSON object
and storing the object as a CouchDB design document. 

# Tests

You can run the unit tests with `go test`.

The tests expect CouchDB to be running at `http://127.0.0.1:5984`.

