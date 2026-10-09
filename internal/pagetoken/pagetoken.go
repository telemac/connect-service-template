// Package pagetoken encodes List page tokens. A token carries the listing
// parameters it was issued for (filters, order) and the cursor of the last
// item returned, so it cannot be replayed with other parameters, which would
// silently skip or repeat items. Clients only see opaque base64; the format
// can change freely.
package pagetoken

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

var (
	ErrMalformed = errors.New("malformed page token")
	ErrMismatch  = errors.New("page token was issued for other parameters")
)

type token[P comparable, C any] struct {
	Params P `json:"p"`
	Cursor C `json:"c"`
}

func Encode[P comparable, C any](params P, cursor C) (string, error) {
	b, err := json.Marshal(token[P, C]{Params: params, Cursor: cursor})
	if err != nil {
		return "", fmt.Errorf("encoding page token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Decode returns the cursor of s, or ErrMalformed, or ErrMismatch when s was
// issued for parameters other than params.
func Decode[P comparable, C any](s string, params P) (C, error) {
	var t token[P, C]
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return t.Cursor, ErrMalformed
	}
	if err := json.Unmarshal(b, &t); err != nil {
		return t.Cursor, ErrMalformed
	}
	if t.Params != params {
		return t.Cursor, ErrMismatch
	}
	return t.Cursor, nil
}
