package couchdb

import "encoding/json"

type bulkID struct {
	ID string `json:"id"`
}

type BulkGet struct {
	Docs []struct{ ID string } `json:"docs"`
}

type errorWrapper struct {
	ID     string `json:"id"`
	Rev    string `json:"rev"`
	Error  string `json:"error"`
	Reason string `json:"reason"`
}

type docWrapper struct {
	Ok    *json.RawMessage `json:"ok"`
	Error *errorWrapper    `json:"error"`
}

type bulkRes struct {
	Id   string       `json:"id"`
	Docs []docWrapper `json:"docs"`
}

type bulkResp struct {
	Results []bulkRes
}
