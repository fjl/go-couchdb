// Package couchapp implements an convenience mechanism for CouchDB design documents.
//
// CouchDB design documents, which contain view definitions etc., are stored
// as JSON objects in the database. A 'couchapp' is a directory structure
// that is compiled into a design document and then installed into the database.
//
// Directory Layout
//
// A typical couchapp directory looks like this:
//
//     <root>/
//       language.txt            // contains, e.g. "javascript"
//         views/
//           foobar/
//             map.js            // contains javascript function
//             reduce.js         // contains javascript function
//         shows/
//           foobar2/
//             some-document.js  // contains javascript function
//
// This would be compiled into the following JSON object:
//
//     {
//         "language": "javascript",           // content of language.txt
//         "views": {
//             "foobar": {
//                 "map": "function () ..."    // content of map.js
//                 "reduce": "function () ..." // content of reduce.js
//             }
//         },
//         "show": {
//             ...
//         }
//     }
package couchapp

import (
	"github.com/fjl/go-couchdb"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
)

var (
	DefaultIgnorePatterns = []string{
		"~$",   // editor swap files
		"^\\.", // hidden files
		"^_",   // CouchDB system fields
	}
)

type Config struct {
	// regexp patterns for ignored files
	// if nil, the default patterns are used
	IgnorePatterns []string
}

type DesignDoc map[string]interface{}

func Store(db *couchdb.Database, id string, ddoc DesignDoc) (err error) {
	designid := "_design/" + id
	var indb struct { Rev string `json:"_rev"` }
	err = db.Get(designid, nil, &indb);
	if err == nil {
		_, err = db.PutRev(designid, indb.Rev, ddoc)
	} else if couchdb.NotFound(err) {
		_, err = db.Put(designid, ddoc)
	}
	return
}

// Compile transforms a directory structure on disk
// into a JSON object according to the scheme laid out in the
// overview section.
//
// If the config is nil, default values are used.
func Compile(dirname string, config *Config) (DesignDoc, error) {
	pats := DefaultIgnorePatterns
	if config != nil && config.IgnorePatterns != nil {
		pats = config.IgnorePatterns
	}

	// compile ignore patterns
	c := &compiler{
		ignore: make([]*regexp.Regexp, len(pats)),
	}
	for i, pat := range pats {
		if rx, err := regexp.Compile(pat); err != nil {
			return nil, err
		} else {
			c.ignore[i] = rx
		}
	}

	return c.compileDir(dirname)
}

type compiler struct {
	ignore []*regexp.Regexp
}

// compileDir creates the JSON representation of
// a design document from a filesystem directory.
func (c *compiler) compileDir(dirname string) (DesignDoc, error) {
	var (
		sub   interface{}
		err   error
		files []os.FileInfo
		obj   = make(DesignDoc)
	)

	if files, err = ioutil.ReadDir(dirname); err != nil {
		return nil, err
	}

	for _, info := range files {
		if c.isIgnored(info.Name()) {
			continue
		}

		subpath := fmt.Sprintf("%s%c%s", dirname, os.PathSeparator, info.Name())
		if info.IsDir() {
			sub, err = c.compileDir(subpath)
		} else {
			sub, err = loadFile(subpath)
		}
		if err != nil {
			return nil, err
		}

		obj[stripExtension(info.Name())] = sub
	}

	return obj, nil
}

// isIgnored tests whether a filename is matched by one
// of the compiler's ignore patterns.
func (c *compiler) isIgnored(filename string) bool {
	for _, rx := range c.ignore {
		if rx.MatchString(filename) {
			return true
		}
	}
	return false
}

// loadFile returns the given file's contents as a string
// and strips off any surrounding whitespace.
func loadFile(filename string) (string, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	} else {
		return string(bytes.Trim(data, " \n\r")), nil
	}
}

// loadFile returns the given filename without its extension.
func stripExtension(filename string) string {
	if i := strings.LastIndex(filename, "."); i == -1 {
		return filename
	} else {
		return filename[:i]
	}
}
