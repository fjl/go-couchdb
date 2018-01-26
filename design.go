package couchdb

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

// Design is a structure that can be used for creating design documents
// ready to be synched with the database.
// At the moment we're only support very basic design documents with views,
// please feel free to add new properties.
type Design struct {
	ID       string `json:"_id"`
	Rev      string `json:"_rev,omitempty"`
	Language string `json:"language"`

	Views map[string]*View `json:"views,omitempty"`
}

// View is a view definition to be used inside a Design document.
type View struct {
	Map    string `json:"map"`
	Reduce string `json:"reduce,omitempty"`
}

// NewDesign will instantiate a design document instance with the base properties
// set and ready to use.
func NewDesign(name string) *Design {
	d := &Design{
		ID:       "_design/" + name,
		Language: "javascript",
		Views:    make(map[string]*View),
	}
	return d
}

// AddView is a helper to be able to add a view definition to the design.
func (d *Design) AddView(name string, view *View) {
	d.Views[name] = view
}

// ViewChecksum will generate a checksum or hash of the design documents
// views such that two design documents can be compared to see if an update
// is required.
// For the time being we assume that the map will maintain the order of
// views.
func (d *Design) ViewChecksum() string {
	var items []string
	var keys []string
	for key := range d.Views {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		view := d.Views[key]
		items = append(items, key, "map", view.Map, "reduce", view.Reduce)
	}
	text := strings.Join(items, ":")
	sum := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%x", sum)
}
