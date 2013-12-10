package couchdaemon

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"time"
)

var (
	stdout         = new(bytes.Buffer)
	stdin, stdin_w = io.Pipe()
	exitchan       = make(chan bool)
)

func init() {
	start(stdin, stdout, func() { exitchan <- true })
}

func TestLog(t *testing.T) {
	defer stdout.Reset()
	log := Log()
	msg := "a\"bc\n"

	n, err := io.WriteString(log, msg)
	if err != nil {
		t.Errorf("write error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("short write: %v != %v", n, len(msg))
	}
	if stdout.String() != `["log","a\"bc"]`+"\n" {
		t.Errorf("wrong JSON output: %q", stdout.String())
	}
}

func TestConfig(t *testing.T) {
	expVal := "12345678"

	defer stdout.Reset()
	go func() { fmt.Fprintf(stdin_w, "%q\n", expVal) }()

	val := Config("a/b/c")
	if val != expVal {
		t.Errorf("result value mismatch: want %q, got: %q", expVal, val)
	}
	if stdout.String() != `["get","a","b","c"]`+"\n" {
		t.Errorf("wrong JSON output: %q", stdout.String())
	}
}

func TestServerURL(t *testing.T) {
	url := "http://127.0.0.1:5984/"

	// queue config response
	defer stdout.Reset()
	go func() {
		io.WriteString(stdin_w, `"127.0.0.1"`+"\n")
		io.WriteString(stdin_w, `"5984"`+"\n")
	}()

	respurl := ServerURL()
	if respurl != url {
		t.Errorf("wrong URL returned: %q != %q", respurl, url)
	}
}

// this has to be the last test because it closes the pipe
func TestExit(t *testing.T) {
	stdin_w.Close()
	select {
	case <-exitchan:
		return
	case <-time.After(200 * time.Millisecond):
		t.Error("exit func has not been called")
	}
}
