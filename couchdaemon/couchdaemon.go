// Package couchdaemon provides utilities for processes running
// as a CouchDB os_daemon.
package couchdaemon

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

var (
	// whether the worker has been initialized
	started  = false
	out      = json.NewEncoder(os.Stdout)
	in       = bufio.NewReader(os.Stdin)
	reqchan  = make(chan request)
	respchan = make(chan []byte)
	exitchan chan<- interface{}
)

// Init configures stdin and stdout for communication with couchdb.
//
// You should call this function early in your initialization
// Using stdio after Init has been called will confuse the
// implementation and should therefore be avoided.
//
// The argument can be a writable channel or nil.
// If it is nil, the process will exit with status 0 
// when CouchDB signals that is exiting by closing stdin.
// If it is a channel, the channel will be closed instead.
func Init(exit chan<- interface{}) {
	go writeloop()
	go readloop()
	started = true
}

// Config reads a parameter value from the couchdb configuration.
func Config(val interface{}, path ...string) error {
	if !started {
		return fmt.Errorf("couchdaemon: Init() must be called before Config()")
	}

	req := request{query: make([]string, len(path)+1), readresp: true}
	req.query[0] = "get"
	copy(req.query[1:], path)

	reqchan <- req
	if err := json.Unmarshal(<-respchan, val); err != nil {
		return fmt.Errorf("couldn't get %v from config: %v", path, err)
	}
	return nil
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
	msg = bytes.TrimRight(msg, "\n")
	reqchan <- request{query: []string{"log", string(msg)}}
	<-respchan // wait until the message has been sent
	return len(msg), nil
}

type request struct {
	query    []string
	readresp bool
}

func writeloop() {
	for {
		req := <-reqchan
		out.Encode(req.query)
		if !req.readresp {
			// unblock caller
			respchan <- nil
		}
	}
}

func readloop() {
	for {
		if line, err := in.ReadBytes('\n'); err != nil {
			if exitchan == nil {
				os.Exit(0)
			} else {
				close(exitchan)
			}
		} else {
			respchan <- line
		}
	}
}
