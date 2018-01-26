package couchdb

import (
	"fmt"
	"time"
)

// ISOTimeFormat is the CouchDB time format
const (
	TimeFormat         = "2006-01-02T15:04:05.000Z"
	TimeFormatShort    = "2006-01-02T15:04:05Z"
	TimeFormatWithZone = "2006-01-02T15:04:05-0700"
	nullString         = "null"
)

// TimeNow return a new CouchTime with current time
func TimeNow() Time {
	return Time{Time: time.Now().UTC()}
}

// Time is used to decode from json times from CouchDB
type Time struct {
	time.Time
}

// TimeWithZone allows times with a zone to be persisted to the database.
// Very useful storing documents in a local time.
type TimeWithZone struct {
	time.Time
}

var nullTime time.Time

// UnmarshalJSON allows type to be passed to json.Unmarshal
func (t *Time) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == nullString {
		return nil
	}
	s = s[1 : len(s)-1] // Remove quotes
	var err error
	*t, err = ParseTime(s)
	return err
}

// MarshalJSON allows type to be passed to json.MarshalJSON
func (t Time) MarshalJSON() ([]byte, error) {
	if t.IsNull() {
		return []byte(nullString), nil
	}
	return []byte(`"` + t.String() + `"`), nil
}

// UnmarshalJSON allows type to be passed to json.Unmarshal
func (t *TimeWithZone) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == nullString {
		return nil
	}
	s = s[1 : len(s)-1] // Remove quotes
	var err error
	*t, err = ParseTimeWithZone(s)
	return err
}

// MarshalJSON allows type to be passed to json.MarshalJSON
func (t TimeWithZone) MarshalJSON() ([]byte, error) {
	if t.IsNull() {
		return []byte(nullString), nil
	}
	return []byte(`"` + t.String() + `"`), nil
}

// IsNull determines if a time value is set or not
func (t *Time) IsNull() bool {
	return t.Equal(nullTime)
}

// IsNull determines if a time value is set or not
func (t *TimeWithZone) IsNull() bool {
	return t.Equal(nullTime)
}

// ParseTime reads the provided ISO time string and creates a time object
func ParseTime(timeString string) (Time, error) {
	// Short parsing has magic to read decimals (thanks Oleg!)
	o, err := time.Parse(TimeFormatShort, timeString)
	if err != nil {
		return Time{nullTime}, fmt.Errorf("unable to parse time in UTC: %s", err)
	}
	return Time{o}, nil
}

// ParseTimeWithZone reads the provided ISO time string and creates a time object
// which references the zone.
func ParseTimeWithZone(timeString string) (TimeWithZone, error) {
	o, err := time.Parse(TimeFormatWithZone, timeString)
	if err != nil {
		return TimeWithZone{nullTime}, fmt.Errorf("unable to parse time with zone: %s", err)
	}
	return TimeWithZone{o}, nil
}

// String outputs back time in CouchDB format
func (t *Time) String() string {
	return t.Format(TimeFormat)
}

// String provides the text version of the time with the zone included.
func (t *TimeWithZone) String() string {
	return t.Format(TimeFormatWithZone)
}
