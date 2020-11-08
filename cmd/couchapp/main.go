// The couchapp tool deploys a directory as a CouchDB design document.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/fjl/go-couchdb"
	"github.com/fjl/go-couchdb/couchapp"
)

func main() {
	var (
		server = flag.String("server", "http://127.0.0.1:5984/", "CouchDB server URL")
		dbname = flag.String("db", "", "Database name (required)")
		docid  = flag.String("docid", "", "Design document name (required)")
		ignore = flag.String("ignore", "", "Ignore patterns.")
	)
	flag.Parse()
	if flag.NArg() != 1 {
		fatalf("Need directory as argument.")
	}
	if *docid == "" {
		fatalf("-docid is required.")
	}
	if *dbname == "" {
		fatalf("-db is required.")
	}

	dir := flag.Arg(0)
	ignores := strings.Split(*ignore, ",")
	doc, err := couchapp.LoadDirectory(dir, ignores)
	if err != nil {
		fatalf("%v", err)
	}
	client, err := couchdb.NewClient(*server, nil)
	if err != nil {
		fatalf("can't create database client: %v", err)
	}
	rev, err := couchapp.Store(client.DB(*dbname), *docid, doc)
	if err != nil {
		fatalf("%v", err)
	}
	fmt.Println(rev)
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
