package couchdb

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Attachment represents document attachments.
type Attachment struct {
	Name string    // Filename
	Type string    // MIME type of the Body
	MD5  []byte    // MD5 checksum of the Body
	Body io.Reader // The body itself
}

// AttachmentInfo requests attachment metadata.
// The returned attachment's Body is always nil.
func (c *Client) AttachmentInfo(db, docid, name, rev string) (*Attachment, error) {
	if docid == "" {
		return nil, fmt.Errorf("couchdb.GetAttachment: empty docid")
	}
	if name == "" {
		return nil, fmt.Errorf("couchdb.GetAttachment: empty attachment Name")
	}

	resp, err := c.closedRequest("HEAD", attpath(db, docid, name, rev), nil)
	if err != nil {
		return nil, err
	}
	return attFromHeaders(name, resp)
}

// GetAttachment retrieves an attachment.
// The caller is responsible for closing the attachment's Body if
// the error is nil.
func (c *Client) GetAttachment(db, docid, name, rev string) (*Attachment, error) {
	if docid == "" {
		return nil, fmt.Errorf("couchdb.GetAttachment: empty docid")
	}
	if name == "" {
		return nil, fmt.Errorf("couchdb.GetAttachment: empty attachment Name")
	}

	resp, err := c.request("GET", attpath(db, docid, name, rev), nil)
	if err != nil {
		return nil, err
	}
	att, err := attFromHeaders(name, resp)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}
	att.Body = resp.Body
	return att, nil
}

// PutAttachment creates or updates an attachment.
// To create an attachment on a non-existing document, pass an empty
// string as the rev.
func (c *Client) PutAttachment(db, docid string, att *Attachment, rev string) (newrev string, err error) {
	if docid == "" {
		return rev, fmt.Errorf("couchdb.PutAttachment: empty docid")
	}
	if att.Name == "" {
		return rev, fmt.Errorf("couchdb.PutAttachment: empty attachment Name")
	}
	if att.Body == nil {
		return rev, fmt.Errorf("couchdb.PutAttachment: nil attachment Body")
	}

	req, err := c.newRequest("PUT", attpath(db, docid, att.Name, rev), att.Body)
	if err != nil {
		return rev, err
	}
	req.Header.Set("content-type", att.Type)

	resp, err := c.http.Do(req)
	if err != nil {
		return rev, err
	}
	var result struct{ Rev string }
	if err := readBody(resp, &result); err != nil {
		return rev, fmt.Errorf("couchdb.PutAttachment: couldn't decode rev: %v", err)
	}
	return result.Rev, nil
}

// DeleteAttachment removes an attachment.
func (c *Client) DeleteAttachment(db, docid, name, rev string) (newrev string, err error) {
	if docid == "" {
		return rev, fmt.Errorf("couchdb.PutAttachment: empty docid")
	}
	if name == "" {
		return rev, fmt.Errorf("couchdb.PutAttachment: empty name")
	}

	resp, err := c.closedRequest("DELETE", attpath(db, docid, name, rev), nil)
	return responseRev(resp, err)
}

func attpath(db, docid, name, rev string) string {
	if rev == "" {
		return path(db, docid, name)
	} else {
		return path(db, docid, name) + "?rev=" + url.QueryEscape(rev)
	}
}

func attFromHeaders(name string, resp *http.Response) (*Attachment, error) {
	att := &Attachment{Name: name, Type: resp.Header.Get("content-type")}
	md5 := resp.Header.Get("content-md5")
	if md5 != "" {
		if len(md5) < 22 || len(md5) > 24 {
			return nil, fmt.Errorf("couchdb: Content-MD5 header has invalid size %d", len(md5))
		}
		sum, err := base64.StdEncoding.DecodeString(md5)
		if err != nil {
			return nil, fmt.Errorf("couchdb: invalid base64 in Content-MD5 header: %v", err)
		}
		att.MD5 = sum
	}
	return att, nil
}
