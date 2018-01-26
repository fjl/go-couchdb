package couchdb_test

import (
	"testing"

	"github.com/cabify/go-couchdb"
)

func TestDesignInstantiation(t *testing.T) {
	design := couchdb.NewDesign("test")
	check(t, "ID assigned", "_design/test", design.ID)
	check(t, "default language", "javascript", design.Language)
	if design.Views == nil {
		t.Error("View map is not initialized!")
	}
}

func TestViewChecksum(t *testing.T) {
	design := couchdb.NewDesign("test")
	design.AddView("by_created_at", &couchdb.View{
		Map:    "function(d) { if (d['created_at']) { emit(d['created_at'], 1); } }",
		Reduce: "_sum",
	})
	cs := design.ViewChecksum()
	check(t, "checksum is correct",
		"2e1c80b5f2eb78fec2396a11dce712648710d300d500c9a392034a21d90bbbcd",
		cs)
	design.Views["by_created_at"].Reduce = "_stats"
	if design.ViewChecksum() == cs {
		t.Error("Checksums match when they should differ!")
	}
}
