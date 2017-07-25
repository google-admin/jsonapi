package jsonapi

import (
	"encoding/json"
	"reflect"
	"strconv"
	"time"
)

// NOTE: reciever for MarshalJSON() should not be a pointer
// https://play.golang.org/p/Cf9yYLIzJA (MarshalJSON() w/ pointer reciever)
// https://play.golang.org/p/5EsItAtgXy (MarshalJSON() w/o pointer reciever)

const iso8601Layout = "2006-01-02T15:04:05Z07:00"

// ISO8601Datetime represents a ISO8601 formatted datetime
// It is a time.Time instance that marshals and unmarshals to the ISO8601 ref
type ISO8601Datetime struct {
	time.Time
}

// MarshalJSON implements the json.Marshaler interface.
func (t ISO8601Datetime) MarshalJSON() ([]byte, error) {
	s := t.Time.Format(iso8601Layout)
	return json.Marshal(s)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (t *ISO8601Datetime) UnmarshalJSON(data []byte) error {
	// Ignore null, like in the main JSON package.
	if string(data) == "null" {
		return nil
	}
	// Fractional seconds are handled implicitly by Parse.
	var err error
	t.Time, err = time.Parse(strconv.Quote(iso8601Layout), string(data))
	return err
}

// ISO8601Datetime.String() - override default String() on time
func (t ISO8601Datetime) String() string {
	return t.Format(iso8601Layout)
}

// UnixMilli (Unix Millisecond) marshals/unmarshals the number of milliseconds elapsed since January 1, 1970 UTC
type UnixMilli struct {
	time.Time
}

// MarshalJSON implements the json.Marshaler interface.
func (t UnixMilli) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.UnixNano() / int64(time.Millisecond))
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (t *UnixMilli) UnmarshalJSON(data []byte) error {
	// Ignore null, like in the main JSON package.
	s := string(data)
	if s == "null" {
		return nil
	}

	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}

	t.Time = time.Unix(v/1000, (v % 1000 * int64(time.Millisecond))).In(time.UTC)

	return nil
}

// func to help determine json.Marshaler implementation
// checks both pointer and non-pointer implementations
func isJSONMarshaler(fv reflect.Value) (json.Marshaler, bool) {
	if u, ok := fv.Interface().(json.Marshaler); ok {
		return u, ok
	}

	if !fv.CanAddr() {
		return nil, false
	}

	u, ok := fv.Addr().Interface().(json.Marshaler)
	return u, ok
}

func doesImplementJSONUnmarshaler(fv reflect.Value) bool {
	_, ok := isJSONUnmarshaler(fv)
	return (ok || isSliceOfJSONUnmarshaler(fv) || isMapOfJSONUnmarshaler(fv))
}

// func to help determine json.Unmarshaler implementation
// checks both pointer and non-pointer implementations
func isJSONUnmarshaler(fv reflect.Value) (json.Unmarshaler, bool) {
	if u, ok := fv.Interface().(json.Unmarshaler); ok {
		return u, ok
	}

	if !fv.CanAddr() {
		return nil, false
	}

	u, ok := fv.Addr().Interface().(json.Unmarshaler)
	return u, ok
}

func isSliceOfJSONUnmarshaler(fv reflect.Value) bool {
	if fv.Kind() != reflect.Slice {
		return false
	}

	typ := reflect.TypeOf(fv.Interface()).Elem()
	_, ok := isJSONUnmarshaler(reflect.Indirect(reflect.New(typ)))
	return ok
}

func isMapOfJSONUnmarshaler(fv reflect.Value) bool {
	if fv.Kind() != reflect.Map {
		return false
	}

	typ := reflect.TypeOf(fv.Interface()).Elem()
	_, ok := isJSONUnmarshaler(reflect.Indirect(reflect.New(typ)))
	return ok
}
