// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// OrderBookMaxOrders is the server's per-request cap on live orders scanned
// while aggregating one order-book page. When the cap is hit the response is
// marked Truncated and carries no continuation cursor.
const OrderBookMaxOrders = 10_000

// ListOrders returns active orders with pagination.
func (c *Client) ListOrders(ctx context.Context, opts PageOpts) ([]Order, error) {
	var result []Order
	if err := c.get(ctx, "/order", pageQuery(opts), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetOrder returns a single order by id (bech32).
func (c *Client) GetOrder(ctx context.Context, id string) (*Order, error) {
	var result Order
	if err := c.get(ctx, fmt.Sprintf("/order/%s", id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListOrdersByPair returns orders for a trading pair with pagination.
// askCurrency and giveCurrency are either the coin ticker (e.g. "ML") or a token id (bech32).
func (c *Client) ListOrdersByPair(ctx context.Context, askCurrency, giveCurrency string, opts PageOpts) ([]Order, error) {
	var result []Order
	path := fmt.Sprintf("/order/pair/%s_%s", askCurrency, giveCurrency)
	if err := c.get(ctx, path, pageQuery(opts), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// validateOrderPair checks the {base}_{quote} shape client-side without
// altering the input: the base/quote parts stay exactly as passed (the native
// coin ticker matches case-insensitively server-side, while token ids are
// exact case-sensitive bech32).
func validateOrderPair(pair string) error {
	parts := strings.Split(pair, "_")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return &RequestError{Option: "pair", Reason: fmt.Sprintf("must be {base}_{quote} with two non-empty parts, got %q", pair)}
	}
	return nil
}

// fetchOrderBook fetches one page of the order book for pair at p's cursor
// position. The side is required and cursors are side-specific: a cursor
// minted on the ask side is rejected with 400 "Invalid cursor" on the bid
// side, and vice versa.
func (c *Client) fetchOrderBook(ctx context.Context, pair string, p *listParams) (*OrderBookPage, error) {
	if err := p.reject("GetOrderBook", paramSort|paramOffsetMode); err != nil {
		return nil, err
	}
	if p.side == "" {
		return nil, &RequestError{Option: "WithSide", Reason: fmt.Sprintf("a side is required: pass WithSide(%q) or WithSide(%q)", SideAsk, SideBid)}
	}
	// Unlike the list endpoints (pools, transactions, holders), the order book
	// handler has no offset path and always answers with the
	// {"items": [...], "next_cursor": ...} envelope, so no cursor parameter is
	// forced here: the cursor, when present, is sent verbatim, and on the first
	// page its absence is equivalent to an empty value (both start the walk
	// from the top). forceCursor stays false — it only matters for endpoints
	// where an empty cursor selects the envelope over a bare array.
	q := p.listQuery(false)
	q.Set("side", p.side)
	// PathEscape keeps valid {base}_{quote} pairs byte-identical while making
	// URL metacharacters in caller input harmless ('#', '?', '%').
	return getRawOrderBook(ctx, c, fmt.Sprintf("/order/pair/%s/book", url.PathEscape(pair)), q)
}

// getRawOrderBook fetches and decodes one order-book page, normalising the
// server's additive "truncated" field (present only when true).
func getRawOrderBook(ctx context.Context, c *Client, path string, query url.Values) (*OrderBookPage, error) {
	raw, err := c.getRaw(ctx, path, query)
	if err != nil {
		return nil, err
	}
	var page OrderBookPage
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if page.Truncated {
		// Enforce the documented invariant client-side: a truncated page never
		// carries a continuation cursor.
		page.NextCursor = nil
	}
	return &page, nil
}

// GetOrderBook returns one aggregated page of the order book for the trading
// pair "{base}_{quote}" (e.g. "ML_tmltken1..."). The pair is passed through
// exactly as given: the native coin ticker matches case-insensitively, while
// token ids are exact, case-sensitive bech32. The side is required — pass
// WithSide(SideAsk) or WithSide(SideBid); levels come best-price-first
// (ascending asks, descending bids) with an exact rational price and its
// floored decimal rendering (see OrderBookPrice).
//
// A nil NextCursor means the end of the book — unless Truncated is true, in
// which case the per-request aggregation cap (OrderBookMaxOrders live orders)
// was hit: the levels are an incomplete aggregation and the walk cannot be
// continued (the server sends no cursor for a truncated page; the SDK
// preserves that invariant — NextCursor is always nil when Truncated). Use
// OrderBookPager to walk both sides page by page.
//
// Cursor re-use across sides fails: the server tags cursors per side
// (book-ask / book-bid) and answers a cross-side cursor with 400
// "Invalid cursor".
//
// When WithOffset is set without a cursor, the cursor silently overrides the
// offset page position server-side (items still applies); the SDK sends both
// parameters and does not alter that behaviour.
func (c *Client) GetOrderBook(ctx context.Context, pair string, opts ...ListOption) (*OrderBookPage, error) {
	if err := validateOrderPair(pair); err != nil {
		return nil, err
	}
	p, err := applyListOptions(opts)
	if err != nil {
		return nil, err
	}
	return c.fetchOrderBook(ctx, pair, p)
}

// OrderBookPager returns a pager that walks the order book of pair on the
// given side, following the server cursor automatically and stopping at the
// end of the book — or as soon as a page reports truncation (Pager.Truncated;
// a truncated page has no continuation cursor by construction). The side is
// required and cursors are side-specific; see GetOrderBook.
func OrderBookPager(c *Client, pair string, side string, opts ...ListOption) *Pager[OrderBookLevel] {
	if err := validateOrderPair(pair); err != nil {
		return fail[OrderBookLevel](err)
	}
	p, err := applyListOptions(opts)
	if err != nil {
		return fail[OrderBookLevel](err)
	}
	if err := p.reject("OrderBookPager", paramSort|paramOffsetMode); err != nil {
		return fail[OrderBookLevel](err)
	}
	// The side comes in positionally; a WithSide option must agree with it.
	if p.side != "" && p.side != side {
		return fail[OrderBookLevel](&RequestError{Option: "WithSide", Reason: fmt.Sprintf("side %q disagrees with the side argument %q", p.side, side)})
	}
	if err := WithSide(side)(p); err != nil {
		return fail[OrderBookLevel](err)
	}
	return NewPager(func(ctx context.Context, cursor *string) ([]OrderBookLevel, *string, bool, error) {
		fetch := p
		if cursor != nil {
			fetch = cloneParams(fetch, *cursor)
		}
		page, err := c.fetchOrderBook(ctx, pair, fetch)
		if err != nil {
			return nil, nil, false, err
		}
		// The server sends next_cursor: null on a truncated page and the SDK
		// keeps that invariant; pass it through so the pager stops there too.
		return page.Levels, page.NextCursor, page.Truncated, nil
	})
}
