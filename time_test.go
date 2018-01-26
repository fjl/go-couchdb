package couchdb

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTimeUnmarshal(t *testing.T) {
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
		// nil string
		{`null`, time.Time{}, false},
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

func TestTimeMarshal(t *testing.T) {
	var cases = []struct {
		Given    time.Time
		Expected string
	}{
		{time.Date(2009, time.November, 10, 23, 19, 45, 0, time.UTC), `"2009-11-10T23:19:45.000Z"`},
		{time.Date(2009, time.November, 10, 13, 19, 4, 0, time.UTC), `"2009-11-10T13:19:04.000Z"`},
		{time.Time{}, `null`},
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

func TestTimeNow(t *testing.T) {
	ct := TimeNow()
	if z, _ := ct.Zone(); z != "UTC" {
		t.Errorf("Failed to get current time in UTC, got: %v", z)
	}
}

func TestIsNull(t *testing.T) {
	ct := Time{}
	if !ct.IsNull() {
		t.Error("Expected empty Couch Time to be null")
	}
}

func TestIsNullWithZone(t *testing.T) {
	ct := TimeWithZone{}
	if !ct.IsNull() {
		t.Error("Expected empty Couch TimeWithZone to be null")
	}
}

func TestTimeWithZoneUnmarshal(t *testing.T) {
	tl, _ := time.LoadLocation("America/Mexico_City")
	var cases = []struct {
		Given    string
		Expected time.Time
		Error    bool
	}{
		// long form
		{`"2009-11-10T23:19:45-0600"`, time.Date(2009, time.November, 10, 23, 19, 45, 0, tl), false},
		// bad form
		{`"Z2009-11-10T13:19:04Z"`, time.Time{}, true},
		// nil string
		{`null`, time.Time{}, false},
	}

	for _, c := range cases {
		payload := []byte(c.Given)
		var output TimeWithZone
		if err := json.Unmarshal(payload, &output); err != nil && !c.Error {
			t.Error(err)
		}
		if !output.Equal(c.Expected) {
			t.Errorf("Expected: %q, Given: %q", c.Expected, output)
		}
	}
}

func TestTimeWithZoneMarshal(t *testing.T) {
	tl, _ := time.LoadLocation("America/Mexico_City")
	var cases = []struct {
		Given    time.Time
		Expected string
	}{
		{time.Date(2009, time.November, 10, 23, 19, 30, 0, tl), `"2009-11-10T23:19:30-0600"`},
		{time.Date(2009, time.November, 10, 13, 19, 4, 0, tl), `"2009-11-10T13:19:04-0600"`},
		{time.Time{}, `null`},
	}

	for _, c := range cases {
		ct := TimeWithZone{c.Given}
		output, err := json.Marshal(ct)
		if err != nil {
			t.Error(err)
		}
		if string(output) != c.Expected {
			t.Errorf("Expected: %q, Given: %q", c.Expected, output)
		}
	}
}
