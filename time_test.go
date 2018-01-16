package couchdb

import (
	"encoding/json"
	"testing"
	"time"
)

func TestUnmarshal(t *testing.T) {
	var cases = []struct {
		Given    string
		Expected time.Time
		Error    bool
	}{
		// long form
		{`"2009-11-10T23:19:45.000Z"`, time.Date(2009, time.November, 10, 23, 19, 45, 0, time.UTC), false},
		// short form
		{`"2009-11-10T13:19:04Z"`, time.Date(2009, time.November, 10, 13, 19, 4, 0, time.UTC), false},
		// bad form
		{`"Z2009-11-10T13:19:04Z"`, time.Time{}, true},
	}

	for _, c := range cases {
		payload := []byte(c.Given)
		var output Time
		if err := json.Unmarshal(payload, &output); err != nil && !c.Error {
			t.Error(err)
		}
		if !output.Equal(c.Expected) {
			t.Errorf("Expected: %q, Given: %q", c.Expected, output)
		}
	}
}

func TestMarshal(t *testing.T) {
	var cases = []struct {
		Given    time.Time
		Expected string
	}{
		{time.Date(2009, time.November, 10, 23, 19, 45, 0, time.UTC), `"2009-11-10T23:19:45.000Z"`},
		{time.Date(2009, time.November, 10, 13, 19, 4, 0, time.UTC), `"2009-11-10T13:19:04.000Z"`},
	}

	for _, c := range cases {
		ct := Time{c.Given}
		output, err := json.Marshal(ct)
		if err != nil {
			t.Error(err)
		}
		if string(output) != c.Expected {
			t.Errorf("Expected: %q, Given: %q", c.Expected, output)
		}
	}
}

func TestIsNull(t *testing.T) {
	ct := Time{}
	if !ct.IsNull() {
		t.Error("Expected empty CouchTime to be null")
	}
}
