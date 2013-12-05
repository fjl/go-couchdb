// Package couchdaemon provides utilities for processes running
// as a CouchDB os_daemon.
package couchdaemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

// TODO: the implementation is a bit hard to test due to the
// globals, but I don't feel like changing that.

var (
	reqchan  = make(chan request)
	respchan = make(chan string, 1)
)

// Init configures stdin and stdout for communication with couchdb.
//
// The argument can be a writable channel or nil.
// If it is nil, the process will exit with status 0
// when CouchDB signals that is exiting by closing stdin.
// If it is a channel, the channel will be closed instead.
//
// You should call this function early in your initialization
// Using stdio after Init has been called will confuse the
// implementation and should therefore be avoided.
// You should also refrain from calling Init more than once.
//
// The other API functions will hang until Init has been called.
func Init(exit chan<- bool) {
	if exit == nil {
		start(os.Stdin, os.Stdout, func() { os.Exit(0) })
	} else {
		start(os.Stdin, os.Stdout, func() { close(exit) })
	}
}

// The tests use this function to check everything without using stdio.
func start(stdin io.Reader, stdout io.Writer, exit func()) {
	go writeloop(stdout)
	go readloop(stdin, exit)
}

// Config reads a parameter value from the couchdb configuration.
// The path elements are separated by '/', e.g. "vendor/version".
// The returned string will be empty if the parameter is unset.
func Config(path string) string {
	req := request{readresp: true, query: strings.Split("get/"+path, "/")}
	reqchan <- req
	return <-respchan
}

// ServerURL returns the URL of the CouchDB server that started the daemon.
func ServerURL() (string, error) {
	var urifile = Config("couchdb/uri_file")
	if urifile == "" {
		return "", fmt.Errorf("missing couchdb/uri_file in config")
	}
	if uri, err := ioutil.ReadFile(urifile); err != nil {
		return "", fmt.Errorf("couldn't open CouchDB URI file: %v", err)
	} else {
		return string(bytes.TrimRight(uri, "\r\n")), nil
	}
}

// Log creates a writer that outputs to the CouchDB log.
//
// The returned writer is threadsafe and therefore suitable
// as an input to log.SetOutput()
func Log() io.Writer {
	return &logger{}
}

type logger struct{}

func (l *logger) Write(msg []byte) (int, error) {
	logmsg := bytes.TrimRight(msg, "\n")
	reqchan <- request{query: []string{"log", string(logmsg)}}
	<-respchan // wait until the message has been sent
	return len(msg), nil
}

type request struct {
	query    []string
	readresp bool
}

func writeloop(stdout io.Writer) {
	out := json.NewEncoder(stdout)
	for req := range reqchan {
		out.Encode(req.query)
		if !req.readresp {
			// unblock caller
			respchan <- ""
		}
	}
}

func readloop(stdin io.Reader, exit func()) {
	dec := json.NewDecoder(stdin)
	for {
		var resp string
		err := dec.Decode(&resp)
		if err != nil {
			if err != io.EOF {
				fmt.Fprintf(os.Stderr, "couchdaemon: response decoding error: %v\n", err)
			}
			respchan <- ""
			exit()
			return
		} else {
			respchan <- resp
		}
	}
}
