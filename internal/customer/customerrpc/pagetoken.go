package customerrpc

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"uuid"

	"connectrpc.com/connect/v2"

	"github.com/telemac/connectrpc-v2-poc/internal/customer"
)

// pageToken is the decoded form of ListCustomers page tokens. It carries the
// order it was issued for so a token cannot be replayed with another sort,
// which would silently skip or repeat customers. Clients only see opaque
// base64; the format can change freely.
type pageToken struct {
	Order customer.Order `json:"o"`
	Key   string         `json:"k,omitzero"`
	ID    uuid.UUID      `json:"i"`
}

func encodePageToken(order customer.Order, after customer.Cursor) (string, error) {
	b, err := json.Marshal(pageToken{Order: order, Key: after.Key, ID: after.ID})
	if err != nil {
		return "", fmt.Errorf("encoding page token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func decodePageToken(token string, order customer.Order) (customer.Cursor, error) {
	invalid := connect.NewError(connect.CodeInvalidArgument, "invalid page_token")
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return customer.Cursor{}, invalid
	}
	var t pageToken
	if err := json.Unmarshal(b, &t); err != nil || t.ID == uuid.Nil() {
		return customer.Cursor{}, invalid
	}
	if t.Order != order {
		return customer.Cursor{}, connect.NewError(connect.CodeInvalidArgument, "page_token was issued for a different order_by or descending")
	}
	return customer.Cursor{Key: t.Key, ID: t.ID}, nil
}
