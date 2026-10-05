// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package indexer

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrorKind classifies the well-known api-server v2 error responses. Every
// non-2xx reply carries a JSON body of the form {"error": "<message>"}; the
// message is matched against the documented server error strings.
type ErrorKind int

const (
	// ErrorKindOther is used for any error response without a recognised message.
	ErrorKindOther ErrorKind = iota
	// ErrorKindBadRequest is a generic 400 "Bad request" (e.g. a missing or
	// invalid query parameter combination).
	ErrorKindBadRequest
	// ErrorKindInvalidCursor is 400 "Invalid cursor": the cursor was not issued
	// by this endpoint (or by this side of the order book), is malformed, or
	// points outside the listing.
	ErrorKindInvalidCursor
	// ErrorKindInvalidNumItems is 400 "Invalid number of items": items was 0 or
	// above the per-page cap (MAX_NUM_ITEMS = 100). The server rejects items=0
	// on every paginated v2 endpoint, offset-based listings included.
	ErrorKindInvalidNumItems
	// ErrorKindInvalidOffsetMode is 400 "Invalid offset mode" (transactions).
	ErrorKindInvalidOffsetMode
	// ErrorKindInvalidPoolsSortOrder is 400 "Invalid pools sort order".
	ErrorKindInvalidPoolsSortOrder
	// ErrorKindInvalidTokenID is 400 "Invalid token Id" (malformed token id in
	// a token-scoped path, e.g. the token holders or order book endpoints).
	ErrorKindInvalidTokenID
	// ErrorKindInvalidOrderPair is 400 "Invalid order trading pair".
	ErrorKindInvalidOrderPair
	// ErrorKindTokenNotFound is 404 "Token not found" (unknown token id).
	ErrorKindTokenNotFound
)

// Sentinel errors for the well-known api-server v2 failures. Use errors.Is to
// match an *HTTPError without string comparison:
//
//	if errors.Is(err, indexer.ErrInvalidCursor) { ... }
var (
	// ErrBadRequest matches 400 "Bad request" responses (e.g. an invalid
	// option combination the SDK forwards, such as a pools cursor with a
	// non-default sort).
	ErrBadRequest = errors.New("bad request")
	// ErrInvalidCursor matches 400 "Invalid cursor" responses.
	ErrInvalidCursor = errors.New("invalid cursor")
	// ErrInvalidNumItems matches 400 "Invalid number of items" responses.
	ErrInvalidNumItems = errors.New("invalid number of items")
	// ErrInvalidOffsetMode matches 400 "Invalid offset mode" responses.
	ErrInvalidOffsetMode = errors.New("invalid offset mode")
	// ErrInvalidPoolsSortOrder matches 400 "Invalid pools sort order"
	// responses.
	ErrInvalidPoolsSortOrder = errors.New("invalid pools sort order")
	// ErrInvalidTokenID matches 400 "Invalid token Id" responses.
	ErrInvalidTokenID = errors.New("invalid token id")
	// ErrInvalidOrderPair matches 400 "Invalid order trading pair" responses.
	ErrInvalidOrderPair = errors.New("invalid order trading pair")
	// ErrTokenNotFound matches 404 "Token not found" responses.
	ErrTokenNotFound = errors.New("token not found")
)

// RequestError reports a request that client-side validation rejected before it
// was sent. The Option field names the offending list option (e.g. "WithItems").
type RequestError struct {
	Option string
	Reason string
}

func (e *RequestError) Error() string {
	return fmt.Sprintf("indexer: invalid %s: %s", e.Option, e.Reason)
}

// Is maps an *HTTPError onto the sentinel errors declared in this package, so
// that errors.Is works across the typed error surface — every ErrorKind other
// than ErrorKindOther has a matching sentinel:
//
//	errors.Is(err, indexer.ErrInvalidCursor)
//	errors.Is(err, indexer.ErrInvalidNumItems)
//	errors.Is(err, indexer.ErrTokenNotFound)
func (e *HTTPError) Is(target error) bool {
	switch target {
	case ErrBadRequest:
		return e.Kind == ErrorKindBadRequest
	case ErrInvalidCursor:
		return e.Kind == ErrorKindInvalidCursor
	case ErrInvalidNumItems:
		return e.Kind == ErrorKindInvalidNumItems
	case ErrInvalidOffsetMode:
		return e.Kind == ErrorKindInvalidOffsetMode
	case ErrInvalidPoolsSortOrder:
		return e.Kind == ErrorKindInvalidPoolsSortOrder
	case ErrInvalidTokenID:
		return e.Kind == ErrorKindInvalidTokenID
	case ErrInvalidOrderPair:
		return e.Kind == ErrorKindInvalidOrderPair
	case ErrTokenNotFound:
		return e.Kind == ErrorKindTokenNotFound
	}
	return false
}

// classifyError extracts the server error message and its ErrorKind from a
// non-2xx response body. The api-server renders client errors as
// {"error": "<message>"} with the exact strings below on status 400, and the
// token lookup miss as 404 "Token not found". Message matching is
// case-insensitive and gated on the status code, so a mislabeled or proxied
// 5xx body degrades to ErrorKindOther instead of steering callers into wrong
// handling; the raw message and body are always preserved.
func classifyError(statusCode int, body string) (string, ErrorKind) {
	var payload struct {
		Error string `json:"error"`
	}
	message := strings.TrimSpace(body)
	if err := json.Unmarshal([]byte(body), &payload); err == nil && payload.Error != "" {
		message = payload.Error
	}

	switch statusCode {
	case http.StatusBadRequest:
		// the message is matched case-insensitively: the api-server renders the
		// strings above, but wording/casing drift should degrade to
		// ErrorKindOther, not crash the caller
		switch strings.ToLower(message) {
		case "bad request":
			return message, ErrorKindBadRequest
		case "invalid cursor":
			return message, ErrorKindInvalidCursor
		case "invalid number of items":
			return message, ErrorKindInvalidNumItems
		case "invalid offset mode":
			return message, ErrorKindInvalidOffsetMode
		case "invalid pools sort order":
			return message, ErrorKindInvalidPoolsSortOrder
		case "invalid token id":
			return message, ErrorKindInvalidTokenID
		case "invalid order trading pair":
			return message, ErrorKindInvalidOrderPair
		}
	case http.StatusNotFound:
		if strings.EqualFold(message, "token not found") {
			return message, ErrorKindTokenNotFound
		}
	}
	return message, ErrorKindOther
}
