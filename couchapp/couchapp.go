// Package couchapp implements a mapping from files to CouchDB documents.
//
// CouchDB design documents, which contain view definitions etc., are stored
// as JSON objects in the database. A 'couchapp' is a directory structure
// that is compiled into a design document and then installed into the
// database. The functions in this package are probably most useful for
// uploading design documents, but can be used for any kind of document.
package couchapp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/fjl/go-couchdb"
	"io/ioutil"
	"mime"
	"os"
	"path"
	"strings"
)

// DefaultIgnorePatterns contains the default list of glob patterns
// that are ignored when building a document from a directory.
var DefaultIgnorePatterns = []string{
	"*~", // editor swap files
	".*", // hidden files
	"_*", // CouchDB system fields
}

// Doc represents CouchDB documents.
type Doc map[string]interface{}

// LoadFile creates a document from a single JSON file.
func LoadFile(file string) (Doc, error) {
	val, err := loadJSON(file)
	if err != nil {
		return nil, err
	}
	mapval, ok := val.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%s does not contain a JSON object", file)
	}
	return Doc(mapval), nil
}

// LoadDirectory transforms a directory structure on disk
// into a JSON object. All directories become JSON objects
// whose keys are file and directory names. For regular files,
// the file extension is stripped from the key. Files with the
// .json extension are included as JSON.
//
// Example tree:
//
//     <root>/
//       a.txt           // contains `text-a`
//       b.json          // contains `{"key": 1}`
//       c/
//         d.xyz/
//         e/
//           f           // contains `text-f`
//
// This would be compiled into the following JSON object:
//
//     {
//       "a": "text-a",
//       "b": {"key": 1},
//       "c": {
//         "d.xyz": {},
//         "e": {
//           "f": "text-f"
//         }
//       }
//     }
//
// The second argument is a slice of glob patterns for ignored files.
// If nil is given, the default patterns are used. The patterns are
// matched against the basename, not the full path.
func LoadDirectory(dirname string, ignores []string) (Doc, error) {
	stack := &objstack{obj: make(Doc)}
	err := walk(dirname, ignores, func(p string, isDir, dirEnd bool) error {
		if dirEnd {
			stack = stack.parent // pop
			return nil
		}

		name := path.Base(p)
		if isDir {
			val := make(map[string]interface{})
			stack.obj[name] = val
			stack = &objstack{obj: val, parent: stack} // push
		} else {
			content, err := load(p)
			if err != nil {
				return err
			}
			stack.obj[stripExtension(name)] = content
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return stack.obj, err
}

type objstack struct {
	obj    map[string]interface{}
	parent *objstack
}

func load(filename string) (interface{}, error) {
	if path.Ext(filename) == ".json" {
		return loadJSON(filename)
	}
	return loadString(filename)
}

// loadString returns the given file's contents as a string
// and strips off any surrounding whitespace.
func loadString(file string) (string, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(bytes.Trim(data, " \n\r")), nil
}

// loadJSON decodes the content of the given file as JSON.
func loadJSON(file string) (interface{}, error) {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	// TODO: use json.Number
	var val interface{}
	if err := json.Unmarshal(content, &val); err != nil {
		if syntaxerr, ok := err.(*json.SyntaxError); ok {
			line := findLine(content, syntaxerr.Offset)
			err = fmt.Errorf("JSON syntax error at %v:%v: %v", file, line, err)
			return nil, err
		}
		return nil, fmt.Errorf("JSON unmarshal error in %v: %v", file, err)
	}
	return val, nil
}

// findLine returns the line number for the given offset into data.
func findLine(data []byte, offset int64) (line int) {
	line = 1
	for i, r := range string(data) {
		if int64(i) >= offset {
			return
		}
		if r == '\n' {
			line++
		}
	}
	return
}

// stripExtension returns the given filename without its extension.
func stripExtension(filename string) string {
	if i := strings.LastIndex(filename, "."); i != -1 {
		return filename[:i]
	}
	return filename
}

// Store updates the given document in a database.
// If the document exists, it will be overwritten.
// The new revision of the document is returned.
func Store(db *couchdb.DB, docid string, doc Doc) (string, error) {
	if rev, err := db.Rev(docid); err == nil {
		return db.Put(docid, doc, rev)
	} else if couchdb.NotFound(err) {
		return db.Put(docid, doc, "")
	} else {
		return "", err
	}
}

// StoreAttachments uploads the files in a directory as attachments
// to a document extension. The document does not need to exist in the
// database. The MIME type of each file is guessed by the filename.
//
// As with LoadDirectory, ignores is a slice of glob patterns
// that are matched against the file/directory basename. If any one of them
// matches, the file is not uploaded. If a nil slice is given, the default
// patterns are used.
//
// A correct revision id is returned in all cases, even if there was an error.
func StoreAttachments(
	db *couchdb.DB,
	docid, rev, dir string,
	ignores []string,
) (newrev string, err error) {
	newrev = rev
	err = walk(dir, ignores, func(p string, isDir, dirEnd bool) error {
		if isDir {
			return nil
		}

		att := &couchdb.Attachment{
			Name: strings.TrimPrefix(p, dir+"/"),
			Type: mime.TypeByExtension(path.Ext(p)),
		}
		if att.Body, err = os.Open(p); err != nil {
			return err
		}
		newrev, err = db.PutAttachment(docid, att, newrev)
		return err
	})
	return
}

type walkFunc func(path string, isDir, dirEnd bool) error

func walk(dir string, ignores []string, callback walkFunc) error {
	if ignores == nil {
		ignores = DefaultIgnorePatterns
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, info := range files {
		isDir := info.IsDir()
		subpath := path.Join(dir, info.Name())
		// skip ignored files
		for _, pat := range ignores {
			if ign, err := path.Match(pat, info.Name()); err != nil {
				return err
			} else if ign {
				goto next
			}
		}

		if err := callback(subpath, isDir, false); err != nil {
			return err
		}
		if isDir {
			if err := walk(subpath, ignores, callback); err != nil {
				return err
			}
			if err := callback(subpath, true, true); err != nil {
				return err
			}
		}
	next:
	}
	return nil
}
