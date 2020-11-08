// The couchfeed tool logs CouchDB feeds.
// This tool is not very useful, it's mostly an API demo.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/fjl/go-couchdb"
)

func main() {
	var (
		server    = flag.String("server", "http://127.0.0.1:5984/", "CouchDB server URL")
		dbname    = flag.String("db", "", "Database name")
		dbupdates = flag.Bool("dbupdates", false, "Show DB updates feed")
		follow    = flag.Bool("f", false, "Use 'continuous' feed mode")
	)
	flag.Parse()
	if !*dbupdates && *dbname == "" {
		fatalf("-db or -dbupdates is required.")
	}
	opt := couchdb.Options{"feed": "normal"}
	if *follow {
		opt["feed"] = "continuous"
	}

	client, err := couchdb.NewClient(*server, nil)
	if err != nil {
		fatalf("can't create database client: %v")
	}

	var f feed
	var show func()
	if *dbupdates {
		f, err = client.DBUpdates(opt)
		show = func() {
			chf := f.(*couchdb.DBUpdatesFeed)
			fmt.Println(chf.Event, "db", chf.DB, "seq", chf.Seq)
		}
	} else {
		f, err = client.DB(*dbname).Changes(opt)
		show = func() {
			chf := f.(*couchdb.ChangesFeed)
			if chf.Deleted {
				fmt.Println("deleted:", chf.ID, chf.Seq)
			} else {
				fmt.Println("changed:", chf.ID, chf.Seq)
			}
		}
	}
	if err != nil {
		fatalf("can't open feed: %v", err)
	}
	defer f.Close()
	for f.Next() {
		show()
	}
	if f.Err() != nil {
		fatalf("feed error: %#v", f.Err())
	}
}

type feed interface {
	Next() bool
	Err() error
	Close() error
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
