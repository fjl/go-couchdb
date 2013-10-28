package couchapp

import (
	"encoding/json"
	"testing"
)

func TestCompile(t *testing.T) {
	compiled, err := Compile("testdata/ok", nil)
	if err != nil {
		t.Fatal(err)
	}

	if testing.Verbose() {
		enc, _ := json.MarshalIndent(compiled, "", "    ")
		t.Log(string(enc))
	}
}

func TestCompileBrokenIgnorePattern(t *testing.T) {
	config := &Config{
		IgnorePatterns: []string{"("},
	}

	_, err := Compile("testdata/ok", config)
	if err == nil {
		t.Error("no error returned although there were broken regexps")
	}
}
