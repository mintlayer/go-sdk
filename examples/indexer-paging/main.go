// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// Command indexer-paging demonstrates the cursor-paginated api-server v2
// endpoints (1.4.1+) of the Mintlayer indexer:
//
//  1. A full walk of the native coin holders listing.
//  2. A both-side walk of an order book, including the truncated-page stop.
//  3. The global transaction listing: with a cursor, and with offset_mode.
//  4. A walk of the pools listing (default creation-height sort only — the
//     by-pledge sort does not support cursors).
//
// Usage:
//
//	go run ./examples/indexer-paging -indexer http://127.0.0.1:3000 \
//	  -pair ML_tmltken1vq0f7na4htjv9c5fmaaun8z6t4fgcs2gn3tctxtl3kc9vmz9tvqzhhm6
//
// The pair flag is optional; pass it as {base}_{quote} with the native coin
// ticker case-insensitive and token IDs exact (case-sensitive bech32).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mintlayer/go-sdk/indexer"
)

func main() {
	indexerURL := flag.String("indexer", "http://127.0.0.1:3000", "indexer base URL")
	pair := flag.String("pair", "", "order-book pair as {base}_{quote} (optional)")
	items := flag.Uint("items", 10, "page size, 1..100")
	flag.Parse()

	if *items == 0 || *items > indexer.MaxNumItems {
		log.Fatalf("items must be 1..%d", indexer.MaxNumItems)
	}

	c := indexer.New(*indexerURL, indexer.WithTimeout(30*time.Second))
	// The parent cap must cover the sum of the per-walk budgets below (up to
	// five walks at 30s each) plus slack, so later demos never inherit an
	// already-expired context.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Each walk derives its own deadline so one slow listing (e.g. every
	// transaction on a sizeable chain) cannot starve the demos after it.
	walkCtx := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(ctx, 30*time.Second)
	}

	// Every walk reports its failure to main so a non-zero exit marks the
	// run as failed (usable as a smoke check in CI).
	failed := false
	run := func(name string, walk func(context.Context) error) {
		wctx, cancelWalk := walkCtx()
		defer cancelWalk()
		if err := walk(wctx); err != nil {
			log.Printf("%s: %v", name, err)
			failed = true
		}
	}

	run("coin holders walk", func(ctx context.Context) error {
		return walkCoinHolders(ctx, c, uint32(*items))
	})
	run("transactions walk", func(ctx context.Context) error {
		return walkTransactionsWithCursor(ctx, c, uint32(*items))
	})
	run("offset listing", func(ctx context.Context) error {
		return listTransactionsWithOffsetMode(ctx, c)
	})
	run("pools walk", func(ctx context.Context) error {
		return walkPools(ctx, c, uint32(*items))
	})
	if *pair != "" {
		run("order book walk", func(ctx context.Context) error {
			return walkOrderBook(ctx, c, *pair, uint32(*items))
		})
	}
	if failed {
		os.Exit(1)
	}
}

// walkCoinHolders pages through the native coin holders, largest balance
// first, until the server runs out of cursors.
func walkCoinHolders(ctx context.Context, c *indexer.Client, items uint32) error {
	fmt.Println("== Coin holders (cursor walk) ==")

	pager := indexer.CoinHoldersPager(c, indexer.WithItems(items))
	n := 0
	err := pager.Walk(ctx, func(h indexer.Holder) bool {
		if n < 5 {
			fmt.Printf("  %s  %s ML\n", h.Address, h.Amount.Decimal)
		}
		n++
		return true
	})
	if err != nil {
		return err
	}
	fmt.Printf("  ... %d holders in total\n\n", n)
	return nil
}

// walkTransactionsWithCursor walks the global transaction listing (newest
// block first, transactions in block order within each block). BlockID is the
// confirming block's hash; it is empty for pending (mempool) transactions.
func walkTransactionsWithCursor(ctx context.Context, c *indexer.Client, items uint32) error {
	fmt.Println("== Global transactions (cursor walk) ==")

	pager := indexer.TransactionsPager(c, indexer.WithItems(items))
	var pending int
	for {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return err
		}
		if page == nil {
			break // nil NextCursor from the server: listing exhausted
		}
		for _, tx := range page {
			if tx.BlockID == "" {
				pending++
			}
		}
	}
	fmt.Printf("  done; %d pending transactions seen\n\n", pending)
	return nil
}

// listTransactionsWithOffsetMode uses the offset-based listing instead of a
// cursor walk. offset_mode=absolute treats the offset as an absolute
// tx_global_index boundary; the two styles cannot be combined (server 400).
func listTransactionsWithOffsetMode(ctx context.Context, c *indexer.Client) error {
	fmt.Println("== Global transactions (offset_mode=absolute) ==")

	page, err := c.ListTransactionsPage(ctx,
		indexer.WithOffsetMode(indexer.OffsetModeAbsolute),
		indexer.WithOffset(0),
		indexer.WithItems(5),
	)
	if err != nil {
		return err
	}
	// Offset-mode pages have no cursors: NextCursor is always nil here.
	for _, tx := range page.Items {
		fmt.Printf("  %s  block %s\n", tx.ID, orDash(tx.BlockID))
	}
	fmt.Printf("  (%d transactions; offset listings have no NextCursor: %v)\n\n",
		len(page.Items), page.NextCursor == nil)
	return nil
}

// walkPools walks the pools listing. Only the default creation-height sort
// supports cursors: PoolsPager rejects any other sort client-side with a
// *RequestError before sending anything, and the server answers 400 Bad
// request when ListPoolsPage combines an explicit cursor with a non-default
// sort.
func walkPools(ctx context.Context, c *indexer.Client, items uint32) error {
	fmt.Println("== Pools (cursor walk, by_height) ==")

	pager := indexer.PoolsPager(c, indexer.WithItems(items))
	n := 0
	err := pager.Walk(ctx, func(p indexer.Pool) bool {
		if n < 5 {
			fmt.Printf("  %s  staked %s\n", p.PoolID, p.StakerBalance.Decimal)
		}
		n++
		return true
	})
	if err != nil {
		var reqErr *indexer.RequestError
		if errors.As(err, &reqErr) {
			return fmt.Errorf("rejected client-side (cursor walks need the default by_height sort): %w", err)
		}
		var httpErr *indexer.HTTPError
		if errors.As(err, &httpErr) && httpErr.Kind == indexer.ErrorKindBadRequest {
			return fmt.Errorf("rejected by the server (sort+cursor combinations are invalid): %w", err)
		}
		return err
	}
	fmt.Printf("  ... %d pools in total\n\n", n)
	return nil
}

// walkOrderBook walks both sides of the order book. Cursors are side-specific
// (book-ask vs book-bid), and a page truncated at the server's aggregation cap
// (10000 orders) carries no continuation cursor: the walk stops there even
// though the book is incomplete.
func walkOrderBook(ctx context.Context, c *indexer.Client, pair string, items uint32) error {
	fmt.Printf("== Order book %s ==\n", pair)

	for _, side := range []string{indexer.SideAsk, indexer.SideBid} {
		pager := indexer.OrderBookPager(c, pair, side, indexer.WithItems(items))
		fmt.Printf("  -- %s side --\n", side)

		n := 0
		err := pager.Walk(ctx, func(level indexer.OrderBookLevel) bool {
			if n < 5 {
				// Price is an exact rational (numer/denom of remaining quote
				// atoms per remaining base atom); Decimal is its floored
				// rendering at the quote currency's decimals.
				fmt.Printf("  price %s (%s)  amount %s\n",
					level.Price.Atoms, level.Price.Decimal, level.Amount.Decimal)
			}
			n++
			return true
		})
		if err != nil {
			return fmt.Errorf("%s book walk: %w", side, err)
		}
		if pager.Truncated() {
			fmt.Printf("  ... %d levels shown; BOOK TRUNCATED at the aggregation cap — no continuation cursor exists\n", n)
		} else {
			fmt.Printf("  ... %d levels shown; end of book\n", n)
		}
	}
	fmt.Println()
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
