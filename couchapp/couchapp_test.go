package couchapp

import (
	"path"
	"reflect"
	"testing"
)

func TestLoadFile(t *testing.T) {
	doc, err := LoadFile("testdata/doc.json")
	if err != nil {
		t.Fatal(err)
	}

	expdoc := Doc{
		"_id":   "doc",
		"float": 1.0,
		"array": []interface{}{1.0, 2.0, 3.0},
	}
	check(t, "doc", expdoc, doc)
}

func TestLoadDirectory(t *testing.T) {
	doc, err := LoadDirectory("testdata/dir", nil)
	if err != nil {
		t.Fatal(err)
	}

	expdoc := Doc{
		"language": "javascript",
		"views": map[string]interface{}{
			"abc.xyz": map[string]interface{}{
				"map": "function (x) { return x; }",
			},
		},
		"options": map[string]interface{}{
			"local_seq": true,
		},
	}
	check(t, "doc", expdoc, doc)
}

func TestBrokenIgnorePattern(t *testing.T) {
	doc, err := LoadDirectory("testdata/dir", []string{"[]"})
	check(t, "doc", Doc(nil), doc)
	check(t, "error", path.ErrBadPattern, err)
}

func check(t *testing.T, field string, expected, actual interface{}) {
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("%s mismatch: want %#v, got %#v", field, expected, actual)
	}
}
