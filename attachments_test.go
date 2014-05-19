package couchdb_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"github.com/fjl/go-couchdb"
	"io"
	"io/ioutil"
	. "net/http"
	"testing"
)

var (
	md5string   = "2mGd+/VXL8dJsUlrD//Xag=="
	md5bytes, _ = base64.StdEncoding.DecodeString(md5string)
)

func TestAttachment(t *testing.T) {
	c := newTestClient(t)
	c.Handle("GET /db/doc/attachment/1",
		func(resp ResponseWriter, req *Request) {
			resp.Header().Set("content-md5", "2mGd+/VXL8dJsUlrD//Xag==")
			resp.Header().Set("content-type", "text/plain")
			io.WriteString(resp, "the content")
		})

	att, err := c.DB("db").Attachment("doc", "attachment/1", "")
	if err != nil {
		t.Fatal(err)
	}
	body, err := ioutil.ReadAll(att.Body)
	if err != nil {
		t.Fatalf("error reading body: %v", err)
	}

	check(t, "att.Name", "attachment/1", att.Name)
	check(t, "att.Type", "text/plain", att.Type)
	check(t, "att.MD5", md5bytes, att.MD5)
	check(t, "att.Body content", "the content", string(body))
}

func TestAttachmentMeta(t *testing.T) {
	c := newTestClient(t)
	c.Handle("HEAD /db/doc/attachment/1",
		func(resp ResponseWriter, req *Request) {
			resp.Header().Set("content-md5", "2mGd+/VXL8dJsUlrD//Xag==")
			resp.Header().Set("content-type", "text/plain")
			resp.WriteHeader(StatusOK)
		})

	att, err := c.DB("db").AttachmentMeta("doc", "attachment/1", "")
	if err != nil {
		t.Fatal(err)
	}

	check(t, "att.Name", "attachment/1", att.Name)
	check(t, "att.Type", "text/plain", att.Type)
	check(t, "att.MD5", md5bytes, att.MD5)
	check(t, "att.Body", nil, att.Body)
}

func TestPutAttachment(t *testing.T) {
	c := newTestClient(t)
	c.Handle("PUT /db/doc/attachment/1",
		func(resp ResponseWriter, req *Request) {
			reqBodyContent, err := ioutil.ReadAll(req.Body)
			if err != nil {
				t.Fatal(err)
			}
			ctype := req.Header.Get("Content-Type")
			check(t, "request content type", "text/plain", ctype)
			check(t, "request body", "the content", string(reqBodyContent))
			check(t, "request query string",
				"rev=1-619db7ba8551c0de3f3a178775509611",
				req.URL.RawQuery)

			resp.Header().Set("content-md5", md5string)
			resp.Header().Set("content-type", "application/json")
			json.NewEncoder(resp).Encode(map[string]interface{}{
				"ok":  true,
				"id":  "doc",
				"rev": "2-619db7ba8551c0de3f3a178775509611",
			})
		})

	att := &couchdb.Attachment{
		Name: "attachment/1",
		Type: "text/plain",
		Body: bytes.NewBufferString("the content"),
	}
	newrev, err := c.DB("db").PutAttachment("doc", att, "1-619db7ba8551c0de3f3a178775509611")
	if err != nil {
		t.Fatal(err)
	}

	check(t, "newrev", "2-619db7ba8551c0de3f3a178775509611", newrev)
	check(t, "att.Name", "attachment/1", att.Name)
	check(t, "att.Type", "text/plain", att.Type)
	check(t, "att.MD5", []byte(nil), att.MD5)
}

func TestDeleteAttachment(t *testing.T) {
	c := newTestClient(t)
	c.Handle("DELETE /db/doc/attachment/1",
		func(resp ResponseWriter, req *Request) {
			check(t, "request query string",
				"rev=1-619db7ba8551c0de3f3a178775509611",
				req.URL.RawQuery)

			resp.Header().Set("etag", `"2-619db7ba8551c0de3f3a178775509611"`)
			json.NewEncoder(resp).Encode(map[string]interface{}{
				"ok":  true,
				"id":  "doc",
				"rev": "2-619db7ba8551c0de3f3a178775509611",
			})
		})

	newrev, err := c.DB("db").DeleteAttachment("doc", "attachment/1", "1-619db7ba8551c0de3f3a178775509611")
	if err != nil {
		t.Fatal(err)
	}

	check(t, "newrev", "2-619db7ba8551c0de3f3a178775509611", newrev)
}
