// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package indexer_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mintlayer/go-sdk/indexer"
)

// readFixture loads a canned JSON response from indexer/testdata.
func readFixture(t *testing.T, name string) json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return json.RawMessage(raw)
}

// recordedRequest is one request captured by a routed test server.
type recordedRequest struct {
	path   string
	params url.Values
}

// route describes one fixture served for a given incoming cursor value.
type route struct {
	cursor string // value of the "cursor" query param ("" = first request)
	file   string // fixture served from testdata
	status int    // when non-zero, serve this status with a JSON error body instead
	errMsg string // error message for status != 0
}

// routedServer serves fixtures per the given routes (matched on the "cursor"
// query param) and records every request. Requests are matched in order for
// status-only routes.
func routedServer(t *testing.T, routes []route) (*httptest.Server, *[]recordedRequest) {
	t.Helper()
	byCursor := map[string]route{}
	for _, r := range routes {
		byCursor[r.cursor] = r
	}
	requests := &[]recordedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*requests = append(*requests, recordedRequest{path: r.URL.Path, params: r.URL.Query()})
		cursor := r.URL.Query().Get("cursor")
		rt, ok := byCursor[cursor]
		if !ok {
			t.Errorf("unexpected cursor %q for %s", cursor, r.URL.Path)
			http.Error(w, `{"error":"Bad request"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if rt.status != 0 {
			w.WriteHeader(rt.status)
			fmt.Fprintf(w, `{"error":%q}`, rt.errMsg)
			return
		}
		w.Write(readFixture(t, rt.file))
	}))
	t.Cleanup(srv.Close)
	return srv, requests
}

// firstPageCursorEmpty asserts the cursor parameter is present but empty on a
// first-page request: the api-server switches to the cursor envelope based on
// parameter presence, even for an empty cursor.
func firstPageCursorEmpty(t *testing.T, reqs *[]recordedRequest) {
	t.Helper()
	if len(*reqs) == 0 {
		t.Fatal("no requests recorded")
	}
	q, ok := (*reqs)[0].params["cursor"]
	if !ok {
		t.Fatalf("first request missing cursor param: %v", (*reqs)[0].params)
	}
	if len(q) != 1 || q[0] != "" {
		t.Fatalf("first request cursor = %v, want present-but-empty", q)
	}
}

// --- Pools ---

func TestPoolsPagerFullWalk(t *testing.T) {
	srv, reqs := routedServer(t, []route{
		{cursor: "", file: "pools_page1.json"},
		{cursor: "pools-cursor-2", file: "pools_page2.json"},
	})
	c := indexer.New(srv.URL)

	pager := indexer.PoolsPager(c, indexer.WithItems(50))
	var ids []string
	err := pager.Walk(context.Background(), func(p indexer.Pool) bool {
		ids = append(ids, p.PoolID)
		return true
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("walked %d pools, want 3 (%v)", len(ids), ids)
	}
	if pager.NextCursor() != nil || pager.Truncated() {
		t.Fatalf("exhausted pager reports cursor=%v truncated=%v", pager.NextCursor(), pager.Truncated())
	}
	firstPageCursorEmpty(t, reqs)
	second := (*reqs)[1].params
	if second.Get("cursor") != "pools-cursor-2" || second.Get("items") != "50" {
		t.Fatalf("second request params = %v", second)
	}
	// A retry after exhaustion is a no-op.
	page, err := pager.NextPage(context.Background())
	if page != nil || err != nil {
		t.Fatalf("NextPage after walk = (%v, %v), want (nil, nil)", page, err)
	}
}

func TestPoolsListPageLastPage(t *testing.T) {
	srv, _ := routedServer(t, []route{
		{cursor: "pools-cursor-2", file: "pools_page2.json"},
	})
	c := indexer.New(srv.URL)

	page, err := c.ListPoolsPage(context.Background(), indexer.WithCursor("pools-cursor-2"))
	if err != nil {
		t.Fatalf("ListPoolsPage: %v", err)
	}
	if page.NextCursor != nil {
		t.Fatalf("last page NextCursor = %v, want nil", *page.NextCursor)
	}
	if len(page.Items) != 1 {
		t.Fatalf("last page items = %d, want 1", len(page.Items))
	}
}

func TestPoolsSortWithCursorServerReject(t *testing.T) {
	// Cursor + non-default sort is rejected by the server; the SDK propagates the 400.
	srv, reqs := routedServer(t, []route{
		{cursor: "some-cursor", status: http.StatusBadRequest, errMsg: "Bad request"},
	})
	c := indexer.New(srv.URL)

	_, err := c.ListPoolsPage(context.Background(),
		indexer.WithSort(indexer.SortByPledge), indexer.WithCursor("some-cursor"))
	if err == nil {
		t.Fatal("ListPoolsPage with sort+cursor, want 400")
	}
	var httpErr *indexer.HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("err = %v, want *HTTPError 400", err)
	}
	if got := (*reqs)[0].params.Get("sort"); got != indexer.SortByPledge {
		t.Fatalf("sort param = %q", got)
	}
}

func TestPoolsNonDefaultSortTakesOffsetPath(t *testing.T) {
	// Without an explicit cursor, a non-default sort must take the offset-based
	// path: no cursor parameter at all, and a bare-array response wrapped with a
	// nil NextCursor.
	var requests []recordedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, recordedRequest{path: r.URL.Path, params: r.URL.Query()})
		if _, ok := r.URL.Query()["cursor"]; ok {
			t.Errorf("by-pledge listing sent cursor param %q", r.URL.Query().Get("cursor"))
		}
		if got := r.URL.Query().Get("sort"); got != indexer.SortByPledge {
			t.Errorf("sort param = %q, want by_pledge", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(readFixture(t, "pools_offset.json"))
	}))
	defer srv.Close()
	c := indexer.New(srv.URL)

	page, err := c.ListPoolsPage(context.Background(), indexer.WithSort(indexer.SortByPledge))
	if err != nil {
		t.Fatalf("ListPoolsPage: %v", err)
	}
	if len(page.Items) != 1 || page.NextCursor != nil {
		t.Fatalf("offset page = (%d items, cursor %v)", len(page.Items), page.NextCursor)
	}
}

// --- Holders ---

func TestCoinHoldersPagerFullWalk(t *testing.T) {
	srv, reqs := routedServer(t, []route{
		{cursor: "", file: "holders_page1.json"},
		{cursor: "holders-cursor-2", file: "holders_page2.json"},
	})
	c := indexer.New(srv.URL)

	pager := indexer.CoinHoldersPager(c, indexer.WithItems(2))
	var got []indexer.Holder
	err := pager.Walk(context.Background(), func(h indexer.Holder) bool {
		got = append(got, h)
		return true
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("walked %d holders, want 3", len(got))
	}
	if got[0].Amount.Atoms != "1500000000000" || got[0].Amount.Decimal != "1500.000000000" {
		t.Fatalf("holder amount = %+v", got[0].Amount)
	}
	firstPageCursorEmpty(t, reqs)
	// The cursor silently overrides the offset server-side; the SDK sends both
	// parameters as-is.
	second := (*reqs)[1].params
	if second.Get("cursor") != "holders-cursor-2" {
		t.Fatalf("second request cursor = %q", second.Get("cursor"))
	}
}

func TestTokenHoldersWalkAndNotFound(t *testing.T) {
	const tokenID = "tglp0vg6mdcs9o6npgcf5785xzsdq2c4rtq5wl6asanjk5mvgc2vmdy5y3vmm4z"

	srv, reqs := routedServer(t, []route{
		{cursor: "", file: "holders_page1.json"},
		{cursor: "holders-cursor-2", file: "holders_page2.json"},
	})
	c := indexer.New(srv.URL)

	// The token id must reach the server byte-for-byte (bech32 is case-sensitive).
	pager := indexer.TokenHoldersPager(c, tokenID)
	n := 0
	err := pager.Walk(context.Background(), func(indexer.Holder) bool { n++; return true })
	if err != nil || n != 3 {
		t.Fatalf("Walk = (%d holders, %v), want (3, nil)", n, err)
	}
	if got := (*reqs)[0].path; got != "/api/v2/statistics/token/"+tokenID+"/holders" {
		t.Fatalf("path = %q", got)
	}

	// Unknown token: 404 "Token not found" maps to ErrTokenNotFound.
	errSrv := errorHandler(t, http.StatusNotFound, `{"error":"Token not found"}`)
	defer errSrv.Close()
	_, err = indexer.New(errSrv.URL).ListTokenHolders(context.Background(), tokenID)
	if !errors.Is(err, indexer.ErrTokenNotFound) {
		t.Fatalf("err = %v, want errors.Is(ErrTokenNotFound)", err)
	}
	var httpErr *indexer.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Kind != indexer.ErrorKindTokenNotFound || httpErr.Message != "Token not found" {
		t.Fatalf("typed error = %#v", err)
	}
}

// --- Global transactions ---

func TestTransactionsPagerFullWalk(t *testing.T) {
	srv, reqs := routedServer(t, []route{
		{cursor: "", file: "transactions_page1.json"},
		{cursor: "tx-cursor-2", file: "transactions_page2.json"},
	})
	c := indexer.New(srv.URL)

	pager := indexer.TransactionsPager(c)
	var txs []indexer.Transaction
	err := pager.Walk(context.Background(), func(tx indexer.Transaction) bool {
		txs = append(txs, tx)
		return true
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(txs) != 3 {
		t.Fatalf("walked %d transactions, want 3", len(txs))
	}
	// block_id carries the confirming block hash; null renders as "" (pending).
	if txs[0].BlockID != "aa11456bd8f83b02f0f5e4b1c1a2d3e4f5061728394a5b6c7d8e9f0a1b2c3d4e" {
		t.Fatalf("confirmed tx block_id = %q", txs[0].BlockID)
	}
	if txs[1].BlockID != "" {
		t.Fatalf("pending tx block_id = %q, want empty", txs[1].BlockID)
	}
	firstPageCursorEmpty(t, reqs)
}

func TestTransactionsOffsetModeSinglePage(t *testing.T) {
	// offset_mode selects the offset-based listing (bare array, no cursors).
	srv := jsonHandler(t, json.RawMessage(readFixture(t, "transactions_offset.json")))
	defer srv.Close()
	c := indexer.New(srv.URL)

	page, err := c.ListTransactionsPage(context.Background(),
		indexer.WithOffsetMode(indexer.OffsetModeAbsolute), indexer.WithOffset(10), indexer.WithItems(5))
	if err != nil {
		t.Fatalf("ListTransactionsPage: %v", err)
	}
	if len(page.Items) != 1 || page.NextCursor != nil {
		t.Fatalf("offset page = (%d items, cursor %v)", len(page.Items), page.NextCursor)
	}
}

func TestTransactionsCursorWithOffsetModeClientReject(t *testing.T) {
	// cursor + offset_mode is rejected client-side with a RequestError before
	// any request is made.
	srv, reqs := routedServer(t, nil)
	c := indexer.New(srv.URL)

	_, err := c.ListTransactionsPage(context.Background(),
		indexer.WithCursor("tx-cursor-2"), indexer.WithOffsetMode(indexer.OffsetModeLegacy))
	var reqErr *indexer.RequestError
	if !errors.As(err, &reqErr) {
		t.Fatalf("err = %v, want *indexer.RequestError", err)
	}
	if reqErr.Option != "WithOffsetMode" {
		t.Fatalf("RequestError.Option = %q, want WithOffsetMode", reqErr.Option)
	}
	if len(*reqs) != 0 {
		t.Fatalf("made %d requests, want 0 (rejected client-side)", len(*reqs))
	}
}

// --- Order book ---

func TestOrderBookBothSidesWalk(t *testing.T) {
	srv, reqs := routedServer(t, []route{
		{cursor: "", file: "book_asks_page1.json"},
		{cursor: "book-ask-cursor-2", file: "book_asks_page2.json"},
	})
	c := indexer.New(srv.URL)
	const pair = "ML_tmlp0vg6mdcs9o6npgcf5785xzsdq2c4rtq5wl6asanjk5mvgc2vmdy5y3vmm4z"

	// Ask side: exact rational price plus floored decimal rendering.
	page, err := c.GetOrderBook(context.Background(), pair, indexer.WithSide(indexer.SideAsk), indexer.WithItems(10))
	if err != nil {
		t.Fatalf("GetOrderBook: %v", err)
	}
	if page.Truncated {
		t.Fatal("first ask page must not be truncated")
	}
	if page.NextCursor == nil || *page.NextCursor != "book-ask-cursor-2" {
		t.Fatalf("ask NextCursor = %v", page.NextCursor)
	}
	if got := page.Levels[0].Price; got.Atoms != "1/3" || got.Decimal != "0.333333333" {
		t.Fatalf("ask price = %+v, want 1/3 floored to 0.333333333", got)
	}

	// Continue with the pager until the end of the book.
	pager := indexer.OrderBookPager(c, pair, indexer.SideAsk,
		indexer.WithCursor(*page.NextCursor), indexer.WithItems(10))
	var levels []indexer.OrderBookLevel
	if err := pager.Walk(context.Background(), func(l indexer.OrderBookLevel) bool {
		levels = append(levels, l)
		return true
	}); err != nil {
		t.Fatalf("ask Walk: %v", err)
	}
	if len(levels) != 1 || pager.Truncated() {
		t.Fatalf("ask continuation = %d levels, truncated %v", len(levels), pager.Truncated())
	}
	for i, r := range *reqs {
		if r.params.Get("side") != indexer.SideAsk {
			t.Fatalf("request %d side = %q", i, r.params.Get("side"))
		}
	}
	// The pair must reach the server unchanged (case preserved).
	if got := (*reqs)[0].path; !strings.Contains(got, "/order/pair/"+pair+"/book") {
		t.Fatalf("path = %q", got)
	}
	// The order book always answers with the envelope, so the first request
	// carries no cursor parameter at all (absence is not the pools/holders
	// style present-but-empty signal).
	if _, ok := (*reqs)[0].params["cursor"]; ok {
		t.Fatalf("first book request sent a cursor param: %v", (*reqs)[0].params)
	}

	// Bid side: descending prices, ends after one page.
	bidSrv, bidReqs := routedServer(t, []route{{cursor: "", file: "book_bids_page1.json"}})
	bidPager := indexer.OrderBookPager(indexer.New(bidSrv.URL), "ML_"+strings.ToUpper("tmlp0vg6mdcs9o6npgcf5785xzsdq2c4rtq5wl6asanjk5mvgc2vmdy5y3vmm4z"), indexer.SideBid)
	n := 0
	if err := bidPager.Walk(context.Background(), func(indexer.OrderBookLevel) bool { n++; return true }); err != nil {
		t.Fatalf("bid Walk: %v", err)
	}
	if n != 2 || bidPager.NextCursor() != nil {
		t.Fatalf("bid walk = %d levels, cursor %v", n, bidPager.NextCursor())
	}
	if got := (*bidReqs)[0].params.Get("side"); got != indexer.SideBid {
		t.Fatalf("bid side param = %q", got)
	}
}

func TestOrderBookTruncatedStopsWalk(t *testing.T) {
	// The server hit the aggregation cap: truncated=true and next_cursor=null.
	// The walk must stop even though the book is incomplete — and no
	// continuation may be attempted.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		if cursor != "" {
			t.Errorf("walk continued after truncation with cursor %q", cursor)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(readFixture(t, "book_asks_truncated.json"))
	}))
	defer srv.Close()
	c := indexer.New(srv.URL)

	pager := indexer.OrderBookPager(c, "ML_tmlp0vg6mdcs9o6npgcf5785xzsdq2c4rtq5wl6asanjk5mvgc2vmdy5y3vmm4z", indexer.SideAsk)
	var n int
	if err := pager.Walk(context.Background(), func(indexer.OrderBookLevel) bool { n++; return true }); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if n != 1 {
		t.Fatalf("truncated walk served %d levels, want 1", n)
	}
	if !pager.Truncated() {
		t.Fatal("pager.Truncated() = false after truncated page")
	}
	if pager.NextCursor() != nil {
		t.Fatal("truncated walk exposes a continuation cursor; must be nil")
	}

	// The raw page mirrors the invariant: Truncated true, NextCursor nil.
	page, err := c.GetOrderBook(context.Background(), "ML_tmlp0vg6mdcs9o6npgcf5785xzsdq2c4rtq5wl6asanjk5mvgc2vmdy5y3vmm4z", indexer.WithSide(indexer.SideAsk))
	if err != nil {
		t.Fatalf("GetOrderBook: %v", err)
	}
	if !page.Truncated || page.NextCursor != nil {
		t.Fatalf("truncated page = (truncated %v, cursor %v)", page.Truncated, page.NextCursor)
	}
}

func TestOrderBookCrossSideCursorRejected(t *testing.T) {
	// An ask-side cursor on a bid walk: 400 "Invalid cursor", mapped to
	// ErrInvalidCursor.
	srv := errorHandler(t, http.StatusBadRequest, `{"error":"Invalid cursor"}`)
	defer srv.Close()
	c := indexer.New(srv.URL)

	_, err := c.GetOrderBook(context.Background(), "ML_tmlp0vg6mdcs9o6npgcf5785xzsdq2c4rtq5wl6asanjk5mvgc2vmdy5y3vmm4z",
		indexer.WithSide(indexer.SideBid), indexer.WithCursor("book-ask-cursor-2"))
	if !errors.Is(err, indexer.ErrInvalidCursor) {
		t.Fatalf("err = %v, want errors.Is(ErrInvalidCursor)", err)
	}
}

func TestOrderBookSideRequired(t *testing.T) {
	srv := jsonHandler(t, map[string]any{})
	defer srv.Close()
	c := indexer.New(srv.URL)

	if _, err := c.GetOrderBook(context.Background(), "ML_tml"); err == nil {
		t.Fatal("GetOrderBook without side, want client-side error")
	} else {
		var reqErr *indexer.RequestError
		if !errors.As(err, &reqErr) || reqErr.Option != "WithSide" {
			t.Fatalf("err = %v, want RequestError{WithSide}", err)
		}
	}
	pager := indexer.OrderBookPager(c, "ML_tml", "")
	if _, err := pager.NextPage(context.Background()); err == nil {
		t.Fatal("OrderBookPager without side, want deferred error")
	}
}

func TestOrderBookPairValidation(t *testing.T) {
	srv := jsonHandler(t, map[string]any{})
	defer srv.Close()
	c := indexer.New(srv.URL)

	for _, pair := range []string{"", "ML", "a_b_c", "_tml", "ML_"} {
		if _, err := c.GetOrderBook(context.Background(), pair, indexer.WithSide(indexer.SideAsk)); err == nil {
			t.Fatalf("GetOrderBook(%q) accepted, want RequestError", pair)
		}
	}
}

// --- Client-side option validation ---

func TestListOptionValidation(t *testing.T) {
	srv := jsonHandler(t, map[string]any{})
	defer srv.Close()
	c := indexer.New(srv.URL)
	ctx := context.Background()

	cases := []struct {
		name     string
		exercise func() error
	}{
		{"items=0", func() error { _, err := c.ListCoinHolders(ctx, indexer.WithItems(0)); return err }},
		{"items>100", func() error { _, err := c.ListPoolsPage(ctx, indexer.WithItems(101)); return err }},
		{"empty cursor", func() error { _, err := c.ListTransactionsPage(ctx, indexer.WithCursor("")); return err }},
		{"bad offset mode", func() error { _, err := c.ListTransactionsPage(ctx, indexer.WithOffsetMode("bogus")); return err }},
		{"bad side", func() error { _, err := c.GetOrderBook(ctx, "ML_tml", indexer.WithSide("bogus")); return err }},
		{"bad sort", func() error { _, err := c.ListPoolsPage(ctx, indexer.WithSort("byheight")); return err }},
		{"sort on book", func() error {
			_, err := c.GetOrderBook(ctx, "ML_tml", indexer.WithSide(indexer.SideAsk), indexer.WithSort(indexer.SortByHeight))
			return err
		}},
		{"offset mode on pools", func() error {
			_, err := c.ListPoolsPage(ctx, indexer.WithOffsetMode(indexer.OffsetModeAbsolute))
			return err
		}},
		{"side on transactions", func() error { _, err := c.ListTransactionsPage(ctx, indexer.WithSide(indexer.SideAsk)); return err }},
		{"offset mode on holders", func() error {
			_, err := c.ListCoinHolders(ctx, indexer.WithOffsetMode(indexer.OffsetModeLegacy))
			return err
		}},
		{"sort on holders", func() error {
			_, err := c.ListTokenHolders(ctx, "tok", indexer.WithSort(indexer.SortByPledge))
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.exercise()
			var reqErr *indexer.RequestError
			if !errors.As(err, &reqErr) {
				t.Fatalf("err = %v, want *indexer.RequestError", err)
			}
			if reqErr.Option == "" || reqErr.Reason == "" {
				t.Fatalf("RequestError missing option/reason: %+v", reqErr)
			}
		})
	}

	// Valid boundary values are accepted client-side.
	if _, err := c.ListPoolsPage(ctx, indexer.WithItems(100)); err != nil {
		t.Fatalf("WithItems(100): %v", err)
	}
	if _, err := c.ListPoolsPage(ctx, indexer.WithItems(1)); err != nil {
		t.Fatalf("WithItems(1): %v", err)
	}
}

// --- Typed error shapes ---

func TestTypedErrorShapes(t *testing.T) {
	cases := []struct {
		name         string
		status       int
		body         string
		wantKind     indexer.ErrorKind
		wantSentinel error
	}{
		{"invalid cursor", 400, `{"error":"Invalid cursor"}`, indexer.ErrorKindInvalidCursor, indexer.ErrInvalidCursor},
		{"invalid num items", 400, `{"error":"Invalid number of items"}`, indexer.ErrorKindInvalidNumItems, indexer.ErrInvalidNumItems},
		{"bad request", 400, `{"error":"Bad request"}`, indexer.ErrorKindBadRequest, indexer.ErrBadRequest},
		{"invalid offset mode", 400, `{"error":"Invalid offset mode"}`, indexer.ErrorKindInvalidOffsetMode, indexer.ErrInvalidOffsetMode},
		{"invalid pools sort", 400, `{"error":"Invalid pools sort order"}`, indexer.ErrorKindInvalidPoolsSortOrder, indexer.ErrInvalidPoolsSortOrder},
		{"invalid token id", 400, `{"error":"Invalid token Id"}`, indexer.ErrorKindInvalidTokenID, indexer.ErrInvalidTokenID},
		{"invalid order pair", 400, `{"error":"Invalid order trading pair"}`, indexer.ErrorKindInvalidOrderPair, indexer.ErrInvalidOrderPair},
		{"token not found", 404, `{"error":"Token not found"}`, indexer.ErrorKindTokenNotFound, indexer.ErrTokenNotFound},
		{"unrecognised body", 500, `{"error":"boom"}`, indexer.ErrorKindOther, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := errorHandler(t, tc.status, tc.body)
			defer srv.Close()
			_, err := indexer.New(srv.URL).GetCoinStatistics(context.Background())

			var httpErr *indexer.HTTPError
			if !errors.As(err, &httpErr) {
				t.Fatalf("err = %v, want *indexer.HTTPError", err)
			}
			if httpErr.Kind != tc.wantKind {
				t.Fatalf("Kind = %v, want %v", httpErr.Kind, tc.wantKind)
			}
			if tc.wantSentinel != nil && !errors.Is(err, tc.wantSentinel) {
				t.Fatalf("errors.Is(%v, %v) = false", err, tc.wantSentinel)
			}
			if !strings.Contains(httpErr.Error(), fmt.Sprintf("%d", tc.status)) {
				t.Fatalf("Error() = %q, want status mention", httpErr.Error())
			}
		})
	}
}

func TestCoinStatsZeroCounters(t *testing.T) {
	// Unwritten counters render as "0"; the SDK keeps the server rendering.
	srv := jsonHandler(t, map[string]any{
		"circulating_supply": map[string]string{"atoms": "0", "decimal": "0.000000000"},
		"preminted":          map[string]string{"atoms": "0", "decimal": "0.000000000"},
		"burned":             map[string]string{"atoms": "0", "decimal": "0.000000000"},
		"staked":             map[string]string{"atoms": "0", "decimal": "0.000000000"},
	})
	defer srv.Close()

	stats, err := indexer.New(srv.URL).GetCoinStatistics(context.Background())
	if err != nil {
		t.Fatalf("GetCoinStatistics: %v", err)
	}
	for _, f := range []struct {
		name   string
		amount indexer.Amount
	}{
		{"circulating_supply", stats.CirculatingSupply},
		{"preminted", stats.Preminted},
		{"burned", stats.Burned},
		{"staked", stats.Staked},
	} {
		if f.amount.Atoms != "0" {
			t.Fatalf("%s atoms = %q, want \"0\" (always present)", f.name, f.amount.Atoms)
		}
	}
}

// --- Offset-based listings unchanged ---

func TestOffsetListingsUnchanged(t *testing.T) {
	pools := []indexer.Pool{{PoolID: "pool_escrow1qzz"}}
	txs := []indexer.Transaction{{ID: "0f96"}}

	poolSrv := jsonHandler(t, pools)
	defer poolSrv.Close()
	txSrv := jsonHandler(t, txs)
	defer txSrv.Close()

	gotPools, err := indexer.New(poolSrv.URL).ListPools(context.Background(),
		indexer.PoolListOpts{PageOpts: indexer.PageOpts{Offset: 5, Items: 10}})
	if err != nil {
		t.Fatalf("ListPools: %v", err)
	}
	if len(gotPools) != 1 || gotPools[0].PoolID != pools[0].PoolID {
		t.Fatalf("ListPools = %v", gotPools)
	}

	gotTxs, err := indexer.New(txSrv.URL).ListTransactions(context.Background(), indexer.PageOpts{Offset: 5, Items: 10})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if len(gotTxs) != 1 || gotTxs[0].ID != txs[0].ID {
		t.Fatalf("ListTransactions = %v", gotTxs)
	}
}

// --- Cycle-2 review hardening ---

// TestPagerEmptyNextCursorEndsWalk pins the invariant that a server-issued
// empty cursor ("next_cursor": "") is end-of-listing, never a continuation
// position (which would re-serve page 1 forever).
func TestPagerEmptyNextCursorEndsWalk(t *testing.T) {
	srv, reqs := routedServer(t, []route{
		{cursor: "", file: "holders_page_empty_cursor.json"},
	})
	c := indexer.New(srv.URL)

	pager := indexer.CoinHoldersPager(c)
	count := 0
	err := pager.Walk(context.Background(), func(indexer.Holder) bool {
		count++
		return true
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if count != 1 {
		t.Fatalf("walked %d holders, want 1", count)
	}
	if pager.NextCursor() != nil || pager.Truncated() {
		t.Fatalf("empty next_cursor reported as cursor=%v truncated=%v", pager.NextCursor(), pager.Truncated())
	}
	if len(*reqs) != 1 {
		t.Fatalf("made %d requests, want 1 (no continuation attempt)", len(*reqs))
	}
}

// TestPoolsPagerRejectsNonDefaultSort covers the pools-specific constructor
// rule (cursor walks require the default creation-height sort).
func TestPoolsPagerRejectsNonDefaultSort(t *testing.T) {
	srv := jsonHandler(t, map[string]any{})
	defer srv.Close()
	c := indexer.New(srv.URL)
	ctx := context.Background()

	pager := indexer.PoolsPager(c, indexer.WithSort(indexer.SortByPledge))
	var reqErr *indexer.RequestError
	_, err := pager.NextPage(ctx)
	if !errors.As(err, &reqErr) || reqErr.Option != "WithSort" {
		t.Fatalf("err = %v, want RequestError{WithSort}", err)
	}
	// The default and explicit by_height sorts remain valid.
	for _, sort := range []string{"", indexer.SortByHeight} {
		opts := []indexer.ListOption{}
		if sort != "" {
			opts = append(opts, indexer.WithSort(sort))
		}
		if p := indexer.PoolsPager(c, opts...); p == nil {
			t.Fatalf("PoolsPager(sort=%q) = nil", sort)
		}
	}
}

// TestTransactionsPagerRejectsOffsetMode covers the transactions-specific
// constructor rule (cursor walks cannot be combined with an offset mode).
func TestTransactionsPagerRejectsOffsetMode(t *testing.T) {
	srv := jsonHandler(t, map[string]any{})
	defer srv.Close()
	c := indexer.New(srv.URL)
	ctx := context.Background()

	pager := indexer.TransactionsPager(c, indexer.WithOffsetMode(indexer.OffsetModeAbsolute))
	var reqErr *indexer.RequestError
	_, err := pager.NextPage(ctx)
	if !errors.As(err, &reqErr) || reqErr.Option != "WithOffsetMode" {
		t.Fatalf("err = %v, want RequestError{WithOffsetMode}", err)
	}
}

// TestHoldersPagerConstructorValidation covers the holder pagers rejecting
// side/sort/offset-mode options at construction, named after the pager.
func TestHoldersPagerConstructorValidation(t *testing.T) {
	srv := jsonHandler(t, map[string]any{})
	defer srv.Close()
	c := indexer.New(srv.URL)
	ctx := context.Background()

	if _, err := indexer.CoinHoldersPager(c, indexer.WithSide(indexer.SideAsk)).NextPage(ctx); err == nil {
		t.Fatal("CoinHoldersPager accepted WithSide")
	} else {
		var reqErr *indexer.RequestError
		if !errors.As(err, &reqErr) || reqErr.Option != "WithSide" {
			t.Fatalf("err = %v, want RequestError{WithSide}", err)
		}
	}
	if _, err := indexer.TokenHoldersPager(c, "tok", indexer.WithOffsetMode(indexer.OffsetModeLegacy)).NextPage(ctx); err == nil {
		t.Fatal("TokenHoldersPager accepted WithOffsetMode")
	} else {
		var reqErr *indexer.RequestError
		if !errors.As(err, &reqErr) || reqErr.Option != "WithOffsetMode" {
			t.Fatalf("err = %v, want RequestError{WithOffsetMode}", err)
		}
	}
	if _, err := indexer.TokenHoldersPager(c, "tok", indexer.WithSort(indexer.SortByPledge)).NextPage(ctx); err == nil {
		t.Fatal("TokenHoldersPager accepted WithSort")
	}
}

// TestTypedErrorsGatedOnStatus pins that message-based kind matching only
// fires on the documented status codes: a 500 body reading
// {"error":"Invalid cursor"} degrades to ErrorKindOther and does not match
// ErrInvalidCursor.
func TestTypedErrorsGatedOnStatus(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		wantKind   indexer.ErrorKind
		wantCursor bool // errors.Is(err, ErrInvalidCursor)
	}{
		{"400 invalid cursor", 400, `{"error":"Invalid cursor"}`, indexer.ErrorKindInvalidCursor, true},
		{"500 invalid cursor body", 500, `{"error":"Invalid cursor"}`, indexer.ErrorKindOther, false},
		{"404 invalid cursor body", 404, `{"error":"Invalid cursor"}`, indexer.ErrorKindOther, false},
		{"404 token not found", 404, `{"error":"Token not found"}`, indexer.ErrorKindTokenNotFound, false},
		{"500 token not found body", 500, `{"error":"Token not found"}`, indexer.ErrorKindOther, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := errorHandler(t, tc.status, tc.body)
			defer srv.Close()
			_, err := indexer.New(srv.URL).GetCoinStatistics(context.Background())
			var httpErr *indexer.HTTPError
			if !errors.As(err, &httpErr) {
				t.Fatalf("err = %v, want *indexer.HTTPError", err)
			}
			if httpErr.Kind != tc.wantKind {
				t.Fatalf("Kind = %v, want %v", httpErr.Kind, tc.wantKind)
			}
			if got := errors.Is(err, indexer.ErrInvalidCursor); got != tc.wantCursor {
				t.Fatalf("errors.Is(ErrInvalidCursor) = %v, want %v", got, tc.wantCursor)
			}
			if tc.status == 404 && tc.wantKind == indexer.ErrorKindTokenNotFound {
				if !errors.Is(err, indexer.ErrTokenNotFound) {
					t.Fatal("errors.Is(ErrTokenNotFound) = false, want true")
				}
			}
		})
	}
}

// TestNewPagerNilFetch pins that a nil fetch function surfaces as a deferred
// error on first use instead of panicking.
func TestNewPagerNilFetch(t *testing.T) {
	pager := indexer.NewPager[indexer.Holder](nil)
	if _, err := pager.NextPage(context.Background()); err == nil {
		t.Fatal("NewPager(nil) NextPage, want deferred error")
	}
	if err := pager.Walk(context.Background(), func(indexer.Holder) bool { return true }); err == nil {
		t.Fatal("NewPager(nil) Walk, want deferred error")
	}
}

// TestPagerNextPageServesRemainder pins that NextPage returns the unserved
// remainder of a partially consumed page before fetching the next one, so
// mixed Walk/NextPage usage cannot silently drop entries.
func TestPagerNextPageServesRemainder(t *testing.T) {
	srv, reqs := routedServer(t, []route{
		{cursor: "", file: "holders_page1.json"},
		{cursor: "holders-cursor-2", file: "holders_page2.json"},
	})
	c := indexer.New(srv.URL)
	ctx := context.Background()

	pager := indexer.CoinHoldersPager(c)
	// Consume one item, then stop mid-page.
	served := 0
	err := pager.Walk(ctx, func(indexer.Holder) bool {
		served++
		return false
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if served != 1 {
		t.Fatalf("served %d items before stopping, want 1", served)
	}

	// The first NextPage returns the remainder of page 1, not a new fetch.
	remainder, err := pager.NextPage(ctx)
	if err != nil {
		t.Fatalf("NextPage remainder: %v", err)
	}
	if len(remainder) != 1 {
		t.Fatalf("remainder = %d items, want 1", len(remainder))
	}
	if len(*reqs) != 1 {
		t.Fatalf("made %d requests after remainder, want 1 (no fetch yet)", len(*reqs))
	}

	// The following NextPage fetches page 2; after that the walk is finished.
	page2, err := pager.NextPage(ctx)
	if err != nil {
		t.Fatalf("NextPage page2: %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("page2 = %d items, want 1", len(page2))
	}
	if page, err := pager.NextPage(ctx); page != nil || err != nil {
		t.Fatalf("NextPage after end = (%v, %v), want (nil, nil)", page, err)
	}
}

// TestPagerNextPageThenWalk pins that a pager driven by NextPage and then
// handed to Walk does not re-serve items the NextPage caller already received:
// Walk resumes with the page after the one NextPage handed out whole.
func TestPagerNextPageThenWalk(t *testing.T) {
	srv, reqs := routedServer(t, []route{
		{cursor: "", file: "holders_page1.json"},
		{cursor: "holders-cursor-2", file: "holders_page2.json"},
	})
	c := indexer.New(srv.URL)
	ctx := context.Background()

	pager := indexer.CoinHoldersPager(c)
	first, err := pager.NextPage(ctx)
	if err != nil {
		t.Fatalf("NextPage: %v", err)
	}
	if len(first) != 2 {
		t.Fatalf("first page = %d items, want 2", len(first))
	}

	// Walk must continue after the items already handed out, not restart.
	var seen []string
	err = pager.Walk(ctx, func(h indexer.Holder) bool {
		seen = append(seen, h.Address)
		return true
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(seen) != 1 {
		t.Fatalf("Walk served %d items after a full NextPage handout, want 1 (page 2 only): %v", len(seen), seen)
	}
	if len(*reqs) != 2 {
		t.Fatalf("made %d requests, want 2 (no re-fetch of page 1)", len(*reqs))
	}
}

// TestPagerZeroValueNoPanic pins that a zero-value Pager (bypassing
// NewPager) surfaces a deferred RequestError on first use instead of
// panicking on the nil fetch function.
func TestPagerZeroValueNoPanic(t *testing.T) {
	pager := &indexer.Pager[indexer.Pool]{}
	if _, err := pager.NextPage(context.Background()); err == nil {
		t.Fatal("NextPage on zero-value Pager = nil error, want RequestError")
	}
	if err := pager.Walk(context.Background(), func(indexer.Pool) bool { return true }); err == nil {
		t.Fatal("Walk on zero-value Pager = nil error, want error")
	}
}

// TestTokenHoldersEmptyTokenID pins the client-side rejection of an empty
// token id on both the single-page method and the pager constructor.
func TestTokenHoldersEmptyTokenID(t *testing.T) {
	srv, reqs := routedServer(t, nil)
	c := indexer.New(srv.URL)
	ctx := context.Background()

	if _, err := c.ListTokenHolders(ctx, ""); err == nil {
		t.Fatal("ListTokenHolders with empty token id = nil error, want RequestError")
	}
	pager := indexer.TokenHoldersPager(c, "")
	if _, err := pager.NextPage(ctx); err == nil {
		t.Fatal("TokenHoldersPager with empty token id: NextPage = nil error, want RequestError")
	}
	if len(*reqs) != 0 {
		t.Fatalf("made %d requests, want 0 (rejected client-side)", len(*reqs))
	}
}

// TestPagerNilItemsWithCursor pins that a fetched page with a null/empty items
// array is normalized to a non-nil slice, so page == nil uniquely means the
// walk is finished even when the server still issued a cursor.
func TestPagerNilItemsWithCursor(t *testing.T) {
	srv, reqs := routedServer(t, []route{
		{cursor: "", file: "holders_page_empty_items.json"},
		{cursor: "holders-cursor-empty-items", file: "holders_page2.json"},
	})
	c := indexer.New(srv.URL)
	ctx := context.Background()

	pager := indexer.CoinHoldersPager(c)
	page, err := pager.NextPage(ctx)
	if err != nil {
		t.Fatalf("NextPage: %v", err)
	}
	if page == nil {
		t.Fatal("empty-items page returned nil, which would read as exhaustion")
	}
	if len(page) != 0 {
		t.Fatalf("page = %d items, want 0", len(page))
	}
	page2, err := pager.NextPage(ctx)
	if err != nil {
		t.Fatalf("NextPage page2: %v", err)
	}
	if page2 == nil || len(page2) != 1 {
		t.Fatalf("page2 = (%v), want 1 item", page2)
	}
	if len(*reqs) != 2 {
		t.Fatalf("made %d requests, want 2", len(*reqs))
	}
}

// TestTokenHoldersPagerDeferredOptionError pins that an invalid option passed
// to TokenHoldersPager surfaces as a deferred RequestError on the first
// NextPage instead of a nil-pointer panic (applyListOptions fails, so the
// constructor must not capture a nil params pointer).
func TestTokenHoldersPagerDeferredOptionError(t *testing.T) {
	srv, reqs := routedServer(t, nil)
	c := indexer.New(srv.URL)
	ctx := context.Background()

	pager := indexer.TokenHoldersPager(c, "ttml1qvalidaddress", indexer.WithItems(0))
	_, err := pager.NextPage(ctx)
	var reqErr *indexer.RequestError
	if !errors.As(err, &reqErr) {
		t.Fatalf("err = %v, want *indexer.RequestError", err)
	}
	if len(*reqs) != 0 {
		t.Fatalf("made %d requests, want 0 (rejected before any request)", len(*reqs))
	}
}

// TestBareArrayLeadingWhitespace pins that a legacy bare-array response with
// leading whitespace still decodes as an offset page (sniffing happens after
// whitespace is trimmed, not on a zero-value envelope).
func TestBareArrayLeadingWhitespace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "  %s  ", readFixture(t, "pools_offset.json"))
	}))
	defer srv.Close()
	c := indexer.New(srv.URL)

	pools, err := c.ListPools(context.Background(), indexer.PoolListOpts{})
	if err != nil {
		t.Fatalf("ListPools: %v", err)
	}
	if len(pools) != 1 {
		t.Fatalf("got %d pools, want 1", len(pools))
	}
}

// TestPagerNextCursorDefensiveCopy pins that mutating the string behind the
// pointer returned by NextCursor does not corrupt the pager's resume position.
func TestPagerNextCursorDefensiveCopy(t *testing.T) {
	srv, reqs := routedServer(t, []route{
		{cursor: "", file: "holders_page1.json"},
		{cursor: "holders-cursor-2", file: "holders_page2.json"},
	})
	c := indexer.New(srv.URL)
	ctx := context.Background()

	pager := indexer.CoinHoldersPager(c)
	if _, err := pager.NextPage(ctx); err != nil {
		t.Fatalf("NextPage: %v", err)
	}
	cur := pager.NextCursor()
	if cur == nil || *cur != "holders-cursor-2" {
		t.Fatalf("NextCursor = %v, want holders-cursor-2", cur)
	}
	*cur = "corrupted-by-caller"

	if _, err := pager.NextPage(ctx); err != nil {
		t.Fatalf("NextPage after mutation: %v", err)
	}
	if len(*reqs) != 2 {
		t.Fatalf("made %d requests, want 2", len(*reqs))
	}
}
