// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// CursorPage is the envelope of a cursor-paginated listing: one page of items
// plus the opaque cursor of the next page. NextCursor is nil when the listing
// is exhausted — and, for the order book, also when the response is truncated
// (see OrderBookPage), in which case the walk cannot be continued at all.
type CursorPage[T any] struct {
	// Items holds the entries of this page. Empty (but non-nil handling is up
	// to the caller) only for an exhausted listing queried from the start.
	Items []T `json:"items"`
	// NextCursor is the server-issued cursor of the next page, or nil when
	// there is no next page (or the walk cannot continue). Pass its value to
	// WithCursor to fetch the following page; never construct or mutate one.
	NextCursor *string `json:"next_cursor"`
}

// pageFetcher fetches one page of a listing at the given cursor position
// (nil cursor = from the beginning). truncated reports that the response was a
// truncated order-book page: no continuation cursor exists and the walk must
// stop even though the book is incomplete.
type pageFetcher[T any] func(ctx context.Context, cursor *string) (items []T, next *string, truncated bool, err error)

// Pager transparently walks a cursor-paginated listing (pools, the global
// transaction listing, the coin/token holders, and the order book). A pager
// follows the server-issued next_cursor automatically, stops on a null cursor,
// and surfaces typed errors (*HTTPError with Kind, *RequestError for invalid
// options).
//
// A pager is single-use and not safe for concurrent use: pages are fetched
// lazily by NextPage, or item by item via Walk.
//
// Page stability is only guaranteed once the indexer's scanner is fully caught
// up; a walk performed during catch-up or a reorg may skip or repeat an entry.
type Pager[T any] struct {
	fetch pageFetcher[T]

	cursor    *string // position of the next page to fetch
	items     []T     // current page
	pos       int     // items of the current page already consumed (served via Walk or handed out via NextPage)
	done      bool    // the listing is exhausted (or truncated)
	truncated bool    // last fetched page reported truncation (order book)
	failed    error   // deferred construction/validation error
	started   bool    // at least one page was fetched
}

// NewPager creates a pager over a page-fetching function. It is the escape
// hatch for walking listings that the package does not wrap directly; prefer
// the typed constructors (PoolsPager, TransactionsPager, CoinHoldersPager,
// TokenHoldersPager, OrderBookPager) whenever one exists.
//
// The fetch function must return the page items, the next cursor (nil when the
// listing is exhausted), and whether the page was truncated (order book only;
// false elsewhere). A truncated page must come with a nil next cursor. A nil
// fetch function yields a pager that surfaces a deferred error on first use
// instead of panicking.
func NewPager[T any](fetch pageFetcher[T]) *Pager[T] {
	if fetch == nil {
		return fail[T](&RequestError{Option: "NewPager", Reason: "fetch function must not be nil"})
	}
	return &Pager[T]{fetch: fetch}
}

// fail returns a pager whose first use reports err (used for construction-time
// option validation errors).
func fail[T any](err error) *Pager[T] {
	return &Pager[T]{failed: err, done: true}
}

// NextPage fetches and returns the next batch of items neither consumed yet:
// the unserved remainder of the current page first (whatever Walk or a
// previous NextPage call has not reached), then a fresh page fetch. Once the
// listing is exhausted (or an order book walk hit a truncated page), it
// returns (nil, nil). Options and transport errors are returned as-is and are
// retryable: a failed NextPage does not advance the walk. A fetched page is
// normalized to a non-nil slice, so page == nil uniquely means the walk is
// finished. An empty fetched page also ends the walk: the api-server issues a
// cursor only alongside items, so an empty page cannot legitimately continue
// (this also bounds Walk against a non-compliant peer that never advances).
// Walk and NextPage share one consumption position, so they can be
// mixed on the same pager without duplicating or dropping items. The returned
// slice aliases the pager's internal page buffer (Walk iterates over the same
// backing array); callers must not modify it, and should treat it as valid
// only until the next call on the pager.
func (p *Pager[T]) NextPage(ctx context.Context) ([]T, error) {
	if p.failed != nil {
		return nil, p.failed
	}
	if p.fetch == nil {
		// The zero value bypasses NewPager's nil check; surface the same
		// deferred-error contract instead of panicking.
		return nil, &RequestError{Option: "NewPager", Reason: "use NewPager to construct a Pager; the zero value has no fetch function"}
	}
	if p.pos < len(p.items) {
		// Hand out whatever neither Walk nor a previous NextPage call consumed
		// of the current page before fetching anew.
		items := p.items[p.pos:]
		p.pos = len(p.items)
		return items, nil
	}
	if p.done {
		return nil, nil
	}
	items, next, truncated, err := p.fetch(ctx, p.cursor)
	if err != nil {
		return nil, err
	}
	p.started = true
	p.truncated = truncated
	if items == nil {
		items = []T{}
	}
	p.items = items
	p.pos = len(items) // the page is handed out whole via the return value
	if next == nil || truncated || *next == "" || len(items) == 0 {
		// A server-issued empty cursor is not a continuation position (it would
		// re-serve the first page forever); treat it as end-of-listing, matching
		// the client-side WithCursor validation. An empty page likewise ends the
		// walk: the api-server issues a cursor only alongside items, so an empty
		// page with a cursor would mean a non-compliant peer — terminating
		// instead of looping unboundedly.
		p.done = true
		p.cursor = nil
	} else {
		p.cursor = next
	}
	return items, nil
}

// Walk calls fn for every remaining item of the listing, across pages, in
// listing order. It resumes from wherever previous Walk or NextPage calls
// stopped, and never serves an item twice. It stops early — without error —
// when fn returns false; the next Walk call or NextPage call continues after
// the last served item. Transport and validation errors abort the walk and
// are returned.
func (p *Pager[T]) Walk(ctx context.Context, fn func(item T) bool) error {
	for {
		if p.pos >= len(p.items) {
			page, err := p.NextPage(ctx)
			if err != nil {
				return err
			}
			if page == nil {
				return nil
			}
			// Adopt the freshly fetched page: NextPage marked it fully handed
			// out via its return value, but Walk consumes pages internally.
			p.items = page
			p.pos = 0
		}
		for ; p.pos < len(p.items); p.pos++ {
			if !fn(p.items[p.pos]) {
				p.pos++ // fn consumed this item; resume after it
				return nil
			}
		}
	}
}

// NextCursor returns the server-issued cursor that resumes the listing after
// the last page fetched to its end, or nil once the listing is exhausted, the
// walk was truncated (order book), or nothing has been fetched yet. After Walk
// stops early mid-page, the current page still holds unserved items: resuming
// from this cursor in a fresh pager skips them, so only persist it once the
// current page has been consumed fully — or keep using the same pager, which
// serves the remainder first. The returned string is the same value the server
// delivered; it is never constructed or modified client-side. The value is
// defensively copied, so mutating it does not affect the pager's resume
// position.
func (p *Pager[T]) NextCursor() *string {
	if !p.started || p.cursor == nil {
		return nil
	}
	v := *p.cursor
	return &v
}

// Truncated reports whether the last fetched page was a truncated order-book
// page: the per-request aggregation cap (OrderBookMaxOrders live orders) was
// hit, the levels in hand are an incomplete aggregation, and no continuation
// cursor exists. Always false for the non-order-book listings.
func (p *Pager[T]) Truncated() bool {
	return p.truncated
}

// getPage fetches one listing page and decodes the {items, next_cursor}
// envelope. Older api-servers answer cursor-less requests with a bare JSON
// array (the pre-1.4.1 shape); that shape is accepted and wrapped into an
// envelope with a nil next cursor.
func getPage[T any](ctx context.Context, c *Client, path string, query url.Values) (*CursorPage[T], error) {
	raw, err := c.getRaw(ctx, path, query)
	if err != nil {
		return nil, err
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) > 0 && raw[0] == '[' {
		var items []T
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		return &CursorPage[T]{Items: items}, nil
	}
	var page CursorPage[T]
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &page, nil
}

// pageResult adapts a getPage result to the pageFetcher signature.
func pageResult[T any](page *CursorPage[T], err error) ([]T, *string, bool, error) {
	if err != nil {
		return nil, nil, false, err
	}
	return page.Items, page.NextCursor, false, nil
}

// getRaw performs a GET request and returns the raw response body, reusing
// get's transport and error classification (decoding into *json.RawMessage
// preserves the raw bytes).
func (c *Client) getRaw(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := c.get(ctx, path, query, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}
