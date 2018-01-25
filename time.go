package couchdb

import (
	"fmt"
	"time"
)

// ISOTimeFormat is the CouchDB time format
const (
	TimeFormat         = "2006-01-02T15:04:05.000Z"
	TimeFormatShort    = "2006-01-02T15:04:05Z"
	TimeFormatTimeZone = "2006-01-02T15:04:05-0700"
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
	if s == "null" {
		return nil
	}
	c, err := time.Parse(`"`+TimeFormat+`"`, s)
	if err != nil {
		var errShort error
		c, errShort = time.Parse(`"`+TimeFormatShort+`"`, s)
		if errShort != nil {
			return fmt.Errorf("unable to parse time, tried two formats. %s; %s", err, errShort)
		}
	}
	t.Time = c
	return nil
}

// MarshalJSON allows type to be passed to json.MarshalJSON
func (t Time) MarshalJSON() ([]byte, error) {
	if t.IsNull() {
		return []byte(`null`), nil
	}
	return []byte(`"` + t.String() + `"`), nil
}

// UnmarshalJSON allows type to be passed to json.Unmarshal
func (t *TimeWithZone) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		return nil
	}
	c, err := time.Parse(`"`+TimeFormatTimeZone+`"`, s)
	if err != nil {
		return fmt.Errorf("unable to parse time with zone. %s", err)
	}
	t.Time = c
	return nil
}

// MarshalJSON allows type to be passed to json.MarshalJSON
func (t TimeWithZone) MarshalJSON() ([]byte, error) {
	if t.IsNull() {
		return []byte(`null`), nil
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
	o, err := time.Parse(TimeFormat, timeString)
	return Time{o}, err
}

// ParseTimeWithZone reads the provided ISO time string and creates a time object
// which references the zone.
func ParseTimeWithZone(timeString string) (TimeWithZone, error) {
	o, err := time.Parse(TimeFormatTimeZone, timeString)
	return TimeWithZone{o}, err
}

// String outputs back time in CouchDB format
func (t *Time) String() string {
	return t.Format(TimeFormat)
}

// StringWithTimeZone outputs the time including a timezone
func (t *Time) StringWithTimeZone() string {
	return t.Format(TimeFormatTimeZone)
}

// String provides the text version of the time with the zone included.
func (t *TimeWithZone) String() string {
	return t.Format(TimeFormatTimeZone)
}
