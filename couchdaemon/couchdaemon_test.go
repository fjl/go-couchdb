package couchdaemon

import (
	"bytes"
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
	defer stdout.Reset()
	go func() { io.WriteString(stdin_w, "12345678\n") }()

	var res int
	if err := Config(&res, "a", "b", "c"); err != nil {
		t.Errorf("Config() call returned error: %v", err)
	}
	if res != 12345678 {
		t.Errorf("result value doesn't match: %v", res)
	}
	if stdout.String() != `["get","a","b","c"]`+"\n" {
		t.Errorf("wrong JSON output: %q", stdout.String())
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
