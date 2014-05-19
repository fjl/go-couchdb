// Package couchdaemon provides utilities for processes running
// as a CouchDB os_daemon.
package couchdaemon

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

var (
	initOnce sync.Once
)

var (
	// ErrNotFound is returned by the config API when a key is not available.
	ErrNotFound = errors.New("couchdaemon: config key not found")

	// ErrNotInitialized is returned by all API functions
	// before Init has been called.
	ErrNotInitialized = errors.New("couchdaemon: not initialized")
)

// Init configures stdin and stdout for communication with couchdb.
//
// The argument can be a writable channel or nil. If it is nil, the process
// will exit with status 0 when CouchDB signals that is exiting. If the value
// is a channel, the channel will be closed instead.
//
// Stdin or stdout directly will confuse CouchDB should therefore be avoided.
//
// You should call this function early in your initialization.
// The other API functions will return ErrNotInitialized until Init
// has been called.
func Init(exit chan<- struct{}) {
	initOnce.Do(func() {
		if exit == nil {
			start(os.Stdin, os.Stdout, func() { os.Exit(0) })
		} else {
			start(os.Stdin, os.Stdout, func() { os.Exit(0) })
		}
	})
}

// ConfigSection reads a whole section from the CouchDB configuration.
// If the section is not present, the error will be ErrNotFound and
// the returned map will be nil.
func ConfigSection(section string) (map[string]string, error) {
	var val *map[string]string
	err := request(&val, "get", section)
	switch {
	case err != nil:
		return nil, err
	case val == nil:
		return nil, ErrNotFound
	default:
		return *val, nil
	}
}

// ConfigVal reads a parameter value from the CouchDB configuration.
// If the parameter is unset, the error will be ErrNotFound and the
// returned string will be empty.
func ConfigVal(section, item string) (string, error) {
	var val *string
	err := request(&val, "get", section, item)
	switch {
	case err != nil:
		return "", err
	case val == nil:
		return "", ErrNotFound
	default:
		return *val, nil
	}
}

// ServerURL returns the URL of the CouchDB server that started the daemon.
func ServerURL() (string, error) {
	port, err := ConfigVal("httpd", "port")
	if err != nil {
		return "", err
	}
	addr, err := ConfigVal("httpd", "bind_address")
	if err != nil {
		return "", err
	}
	if addr == "0.0.0.0" {
		addr = "127.0.0.1"
	}
	return "http://" + addr + ":" + port + "/", nil
}

// A LogWriter writes messages to the CouchDB log.
// Its method set is a subset of the methods provided by log/syslog.Writer.
type LogWriter interface {
	io.Writer
	// Err writes a message with level "error"
	Err(msg string) error
	// Info writes a message with level "info"
	Info(msg string) error
	// Info writes a message with level "debug"
	Debug(msg string) error
}

type logger struct{}

// NewLogWriter creates a log writer that outputs to the CouchDB log.
func NewLogWriter() LogWriter { return logger{} }

func (logger) Err(msg string) error   { return logwrite(msg, &optsError) }
func (logger) Info(msg string) error  { return logwrite(msg, &optsInfo) }
func (logger) Debug(msg string) error { return logwrite(msg, &optsDebug) }

func (logger) Write(msg []byte) (int, error) {
	if err := logwrite(string(msg), nil); err != nil {
		return 0, err
	}
	return len(msg), nil
}

var (
	optsError = json.RawMessage(`{"level":"error"}`)
	optsInfo  = json.RawMessage(`{"level":"info"}`)
	optsDebug = json.RawMessage(`{"level":"debug"}`)
)

func logwrite(msg string, opts *json.RawMessage) error {
	msg = strings.TrimRight(msg, "\n")
	if opts == nil {
		return request(nil, "log", msg)
	}
	return request(nil, "log", msg, opts)
}

var (
	// mutex protects the globals during initialization and request I/O
	mutex sync.Mutex

	exit   func()
	stdin  io.ReadCloser
	stdout io.Writer
	inputc chan []byte
)

func start(in io.ReadCloser, out io.Writer, ef func()) {
	mutex.Lock()
	defer mutex.Unlock()

	exit = ef
	stdin = in
	stdout = out
	inputc = make(chan []byte)
	go inputloop(in, inputc, exit)
}

// inputloop reads lines from stdin until it is closed.
func inputloop(in io.Reader, inputc chan<- []byte, exit func()) {
	bufin := bufio.NewReader(in)
	for {
		line, err := bufin.ReadBytes('\n')
		if err != nil {
			break
		}
		inputc <- line
	}
	exit()
	close(inputc)
}

func request(result interface{}, query ...interface{}) error {
	mutex.Lock()
	defer mutex.Unlock()

	if exit == nil {
		return ErrNotInitialized
	}
	line, err := json.Marshal(query)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "%s\n", line); err != nil {
		return err
	}
	if result != nil {
		if err := json.Unmarshal(<-inputc, result); err != nil {
			return err
		}
	}
	return nil
}
