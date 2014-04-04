package couchdaemon

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"testing"
	"time"
)

type testHost struct {
	output   *bytes.Buffer
	exitchan chan struct{}
	config   testConfig
	outW     io.Closer
}

type testConfig map[string]map[string]string

func startTestHost(t *testing.T, config testConfig) *testHost {
	inR, inW := io.Pipe()   // input stream (testHost writes, daemon reads)
	outR, outW := io.Pipe() // output stream (testHost reads, daemon writes)
	th := &testHost{
		output:   new(bytes.Buffer),
		exitchan: make(chan struct{}),
		config:   config,
		outW:     outW,
	}

	go func() {
		enc := json.NewEncoder(inW)
		bufoutR := bufio.NewReader(outR)
		var req []interface{}
		for {
			line, err := bufoutR.ReadBytes('\n')
			if err != nil {
				return
			}

			th.output.Write(line)
			if err := json.Unmarshal(line, &req); err != nil {
				t.Errorf("testHost: could not decode request: %v", err)
				return
			}

			t.Logf("testHost: got request %v", req)
			switch {
			case len(req) <= 1:
				t.Errorf("request array to short")
			case req[0] == "log":
				break
			case req[0] == "get" && req[1] == "garbage":
				io.WriteString(inW, "garbage line\n")
			case req[0] == "get" && len(req) == 2:
				enc.Encode(config[req[1].(string)])
			case req[0] == "get" && len(req) == 3:
				if v, found := config[req[1].(string)][req[2].(string)]; found {
					enc.Encode(v)
				} else {
					enc.Encode(nil)
				}
			default:
				t.Errorf("testHost: unmatched request")
			}
		}
	}()

	start(inR, outW, func() { close(th.exitchan) })
	return th
}

func (th *testHost) stop() {
	th.outW.Close()
	stdin.Close()
}

func TestNotInitialized(t *testing.T) {
	if _, err := ConfigSection("s"); err != ErrNotInitialized {
		t.Errorf("ConfigSection err mismatch, got %v, want ErrNotInitialized")
	}
	if _, err := ConfigVal("s", "k"); err != ErrNotInitialized {
		t.Errorf("ConfigVal err mismatch, got %v, want ErrNotInitialized")
	}
	if _, err := ServerURL(); err != ErrNotInitialized {
		t.Errorf("ServerURL err mismatch, got %v, want ErrNotInitialized")
	}
	log := NewLogWriter()
	if _, err := log.Write([]byte("foo")); err != ErrNotInitialized {
		t.Errorf("log.Write err mismatch, got %v, want ErrNotInitialized")
	}
	if err := log.Err("foo"); err != ErrNotInitialized {
		t.Errorf("log.Err err mismatch, got %v, want ErrNotInitialized")
	}
	if err := log.Info("foo"); err != ErrNotInitialized {
		t.Errorf("log.Info err mismatch, got %v, want ErrNotInitialized")
	}
	if err := log.Debug("foo"); err != ErrNotInitialized {
		t.Errorf("log.Debug err mismatch, got %v, want ErrNotInitialized")
	}
}

func TestLogWrite(t *testing.T) {
	th := startTestHost(t, nil)
	defer th.stop()

	log := NewLogWriter()
	msg := "a\"bc\n"

	n, err := io.WriteString(log, msg)
	if err != nil {
		t.Errorf("write error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("short write: %v != %v", n, len(msg))
	}
	if th.output.String() != `["log","a\"bc"]`+"\n" {
		t.Errorf("wrong JSON output: %s", th.output.String())
	}
}

func TestLogWriteError(t *testing.T) {
	th := startTestHost(t, nil)
	defer th.stop()

	th.outW.Close()

	log := NewLogWriter()
	if _, err := log.Write([]byte("msg")); err != io.ErrClosedPipe {
		t.Errorf(`log.Write("msg") err mismatch, got %v, want io.ErrClosedPipe`)
	}
}

func TestLogLevels(t *testing.T) {
	th := startTestHost(t, nil)
	defer th.stop()

	log := NewLogWriter()
	cases := []struct {
		method func(string) error
		output string
	}{
		{log.Err, `["log","msg",{"level":"error"}]`},
		{log.Info, `["log","msg",{"level":"info"}]`},
		{log.Debug, `["log","msg",{"level":"debug"}]`},
	}

	for _, testcase := range cases {
		th.output.Reset()
		if err := testcase.method("msg"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if th.output.String() != testcase.output+"\n" {
			t.Errorf("wrong JSON output: %s", th.output.String())
		}
	}
}

func TestConfigVal(t *testing.T) {
	th := startTestHost(t, testConfig{
		"a": {"b": "12345678"},
	})
	defer th.stop()

	expVal := th.config["a"]["b"]
	val, err := ConfigVal("a", "b")
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if val != expVal {
		t.Errorf(`ConfigVal("a", "b") got: %q, want: %q`, val, expVal)
	}
	if th.output.String() != `["get","a","b"]`+"\n" {
		t.Errorf("wrong JSON output: %q", th.output.String())
	}
}

func TestConfigValNotFound(t *testing.T) {
	th := startTestHost(t, nil)
	defer th.stop()

	val, err := ConfigVal("missing-s", "missing-k")
	if val != "" {
		t.Errorf(`ConfigVal("missing-s", "missing-k") got: %q, want: ""`, val)
	}
	if err != ErrNotFound {
		t.Errorf(`ConfigVal("missing-s", "missing-k") got err: %v, want: ErrNotFound`, err)
	}
}

func TestConfigValDecodeError(t *testing.T) {
	th := startTestHost(t, nil)
	defer th.stop()

	_, err := ConfigVal("garbage", "key")
	t.Logf("err: %+v", err)
	if err == nil {
		t.Fatalf("expected error but no error was returned")
	}
}

func TestConfigSection(t *testing.T) {
	th := startTestHost(t, testConfig{
		"section1": {
			"a": "value-of-a",
			"b": "value-of-b",
			"c": "value-of-c",
		},
		"section2": {
			"a": "value-of-a-in-section-2",
		},
	})
	defer th.stop()

	expVal := th.config["section1"]
	val, err := ConfigSection("section1")
	if err != nil {
		t.Fatalf(`ConfigSection("section1") returned an error: %v`, err)
	}
	if !reflect.DeepEqual(val, expVal) {
		t.Errorf(`ConfigSection("section1") got: %v, want %v`, val, expVal)
	}
	if th.output.String() != `["get","section1"]`+"\n" {
		t.Errorf("wrong JSON output: %q", th.output.String())
	}
}

func TestConfigSectionNotFound(t *testing.T) {
	th := startTestHost(t, nil)
	defer th.stop()

	val, err := ConfigSection("missing")
	if val != nil {
		t.Errorf(`ConfigSection("missing") got: %q, want: ""`, val)
	}
	if err != ErrNotFound {
		t.Errorf(`ConfigSection("missing") got err: %v, want: ErrNotFound`, err)
	}
}

func TestConfigSectionDecodeError(t *testing.T) {
	th := startTestHost(t, nil)
	defer th.stop()

	val, err := ConfigSection("garbage")
	if err == nil {
		t.Errorf(`ConfigSection("garbage") should've returned an error`)
	}
	if val != nil {
		t.Errorf(`ConfigSection("garbage") got: %v, want nil`, val)
	}
}

func TestServerURL(t *testing.T) {
	th := startTestHost(t, testConfig{
		"httpd": {
			"bind_address": "127.0.0.1",
			"port":         "5984",
		},
	})
	defer th.stop()

	expVal := "http://127.0.0.1:5984/"
	respurl, err := ServerURL()
	if err != nil {
		t.Fatalf("ServerURL() returned error: %v", err)
	}
	if respurl != expVal {
		t.Errorf("ServerURL() mismatch: got %q, want %q", respurl, expVal)
	}
}

func TestExit(t *testing.T) {
	th := startTestHost(t, nil)
	th.stop()

	select {
	case <-th.exitchan:
		return
	case <-time.After(200 * time.Millisecond):
		t.Error("exit func has not been called")
	}
}
