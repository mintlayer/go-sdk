// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package indexer

import (
	"fmt"
	"net/url"
	"strconv"
)

// Listing constants shared by the cursor-paginated api-server v2 endpoints.
const (
	// MaxNumItems is the server-side per-page cap (MAX_NUM_ITEMS). A larger
	// items value is rejected with 400 "Invalid number of items".
	MaxNumItems = 100

	// SideAsk selects the ask side of an order book: orders asking for the base
	// currency (first in the pair) while giving the quote currency.
	SideAsk = "ask"
	// SideBid selects the bid side of an order book: the reverse of ask.
	SideBid = "bid"

	// SortByHeight is the default pool sort: newest creation height first. It
	// is the only sort that supports cursor pagination.
	SortByHeight = "by_height"
	// SortByPledge sorts pools by largest staker balance first. It does not
	// support cursor pagination (a cursor combined with it is a server 400).
	SortByPledge = "by_pledge"

	// OffsetModeLegacy selects the legacy offset semantics of the global
	// transaction listing (the pre-1.4.1 behaviour). It cannot be combined
	// with a cursor (server 400).
	OffsetModeLegacy = "legacy"
	// OffsetModeAbsolute treats the offset as an absolute tx_global_index
	// boundary. It cannot be combined with a cursor (server 400).
	OffsetModeAbsolute = "absolute"
)

// listParam is a bitset of the list options a caller set explicitly. It drives
// per-endpoint applicability validation (e.g. WithSide is only meaningful for
// the order book) so that mistakes surface as clear client-side errors instead
// of silently ignored parameters.
type listParam uint8

const (
	paramItems listParam = 1 << iota
	paramOffset
	paramCursor
	paramSort
	paramOffsetMode
	paramSide
)

// listParams carries the resolved listing parameters of one request.
type listParams struct {
	items      uint32
	offset     uint64
	cursor     string
	sort       string
	offsetMode string
	side       string

	set listParam
}

// ListOption customises a cursor-paginated listing request (the *Page methods,
// the holders/order book endpoints, and the pagers built from them). Options
// are validated per endpoint: passing an option the endpoint does not support
// (or an out-of-range value) fails the request with a *RequestError before any
// network traffic.
//
// The offset-based listings (ListPools, ListTransactions, ListOrders, …) keep
// taking the plain PageOpts / PoolListOpts structs and are unaffected.
type ListOption func(*listParams) error

// WithItems sets the page size. The accepted range is 1..=100 (MaxNumItems):
// the server rejects items=0 with 400 "Invalid number of items" on every
// paginated v2 endpoint (offset-based listings included), and caps the page at
// 100. Omit WithItems to use the server default (10).
func WithItems(items uint32) ListOption {
	return func(p *listParams) error {
		if items == 0 {
			return &RequestError{Option: "WithItems", Reason: "items must be at least 1; the server rejects items=0 with 400 invalid number of items"}
		}
		if items > MaxNumItems {
			return &RequestError{Option: "WithItems", Reason: fmt.Sprintf("items must be at most %d (server MAX_NUM_ITEMS)", MaxNumItems)}
		}
		p.items = items
		p.set |= paramItems
		return nil
	}
}

// WithOffset sets the offset of an offset-based listing. Only the transaction
// listing and the pools listing still honour an offset when no cursor is used;
// everywhere else the cursor defines the page position server-side and the
// offset is ignored (items still applies). The SDK sends both parameters
// as-is and does not second-guess the server.
func WithOffset(offset uint64) ListOption {
	return func(p *listParams) error {
		p.offset = offset
		p.set |= paramOffset
		return nil
	}
}

// WithCursor resumes a listing from a cursor previously returned by the server
// (CursorPage.NextCursor / Pager.NextCursor). Cursors are opaque: never
// construct, decode, or modify one — a fabricated or cross-endpoint cursor is
// rejected by the server with 400 "Invalid cursor". An empty string is
// rejected client-side; omit WithCursor entirely to start from the beginning.
//
// Cursors are endpoint-specific and, for the order book, side-specific: a
// cursor minted by the ask book is rejected on the bid side and vice versa.
func WithCursor(cursor string) ListOption {
	return func(p *listParams) error {
		if cursor == "" {
			return &RequestError{Option: "WithCursor", Reason: "cursor must be a non-empty server-issued value; omit WithCursor to start from the beginning"}
		}
		p.cursor = cursor
		p.set |= paramCursor
		return nil
	}
}

// WithSort sets the pool listing sort order: SortByHeight (default) or
// SortByPledge; "" means the server default (SortByHeight). Any other value
// fails client-side. Only the default creation-height sort supports cursor
// pagination; combining any other sort value with a cursor is rejected by the
// server with 400 "Bad request" (the SDK propagates that response).
func WithSort(sort string) ListOption {
	return func(p *listParams) error {
		switch sort {
		case "", SortByHeight, SortByPledge:
		default:
			return &RequestError{Option: "WithSort", Reason: fmt.Sprintf("unknown sort %q: use %q or %q", sort, SortByHeight, SortByPledge)}
		}
		p.sort = sort
		p.set |= paramSort
		return nil
	}
}

// WithOffsetMode selects the offset semantics of the global transaction
// listing: OffsetModeLegacy (default) or OffsetModeAbsolute. The offset modes
// cannot be combined with a cursor — the server rejects the combination with
// 400 "Bad request", which the SDK propagates.
func WithOffsetMode(mode string) ListOption {
	return func(p *listParams) error {
		switch mode {
		case OffsetModeLegacy, OffsetModeAbsolute:
		default:
			return &RequestError{Option: "WithOffsetMode", Reason: fmt.Sprintf("mode must be %q or %q", OffsetModeLegacy, OffsetModeAbsolute)}
		}
		p.offsetMode = mode
		p.set |= paramOffsetMode
		return nil
	}
}

// WithSide selects the side of the order book listing: SideAsk or SideBid. The
// side is required — a book request without it fails client-side. Cursors are
// side-specific (the server tags them book-ask / book-bid), so a cursor minted
// on one side is rejected with 400 "Invalid cursor" on the other.
func WithSide(side string) ListOption {
	return func(p *listParams) error {
		switch side {
		case SideAsk, SideBid:
		default:
			return &RequestError{Option: "WithSide", Reason: fmt.Sprintf("side must be %q or %q", SideAsk, SideBid)}
		}
		p.side = side
		p.set |= paramSide
		return nil
	}
}

// applyListOptions folds opts over a fresh listParams, preserving the first error.
func applyListOptions(opts []ListOption) (*listParams, error) {
	p := &listParams{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(p); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// reject rejects the explicitly-set params in disallow, naming the first
// offending option.
func (p *listParams) reject(endpoint string, disallow listParam) error {
	if p.set&paramSide&disallow != 0 {
		return &RequestError{Option: "WithSide", Reason: fmt.Sprintf("not supported by %s", endpoint)}
	}
	if p.set&paramOffsetMode&disallow != 0 {
		return &RequestError{Option: "WithOffsetMode", Reason: fmt.Sprintf("not supported by %s", endpoint)}
	}
	if p.set&paramSort&disallow != 0 {
		return &RequestError{Option: "WithSort", Reason: fmt.Sprintf("not supported by %s", endpoint)}
	}
	return nil
}

// cloneParams returns a copy of p with the cursor swapped to cursor. Used by
// the pagers to advance an already-validated parameter set; the cursor value
// is always a server-issued one delivered through CursorPage.NextCursor.
func cloneParams(p *listParams, cursor string) *listParams {
	q := *p
	q.cursor = cursor
	q.set |= paramCursor
	return &q
}

// listQuery renders the common listing query parameters. An empty cursor is
// only emitted when forceCursor is set (the api-server returns the
// {items, next_cursor} envelope exactly when the cursor parameter is present,
// even when empty; an empty cursor starts the walk from the beginning).
func (p *listParams) listQuery(forceCursor bool) url.Values {
	q := url.Values{}
	if p.items > 0 {
		q.Set("items", strconv.FormatUint(uint64(p.items), 10))
	}
	if p.set&paramOffset != 0 {
		q.Set("offset", strconv.FormatUint(p.offset, 10))
	}
	if p.cursor != "" {
		q.Set("cursor", p.cursor)
	} else if forceCursor {
		q.Set("cursor", "")
	}
	return q
}
