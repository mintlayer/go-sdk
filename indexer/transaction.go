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
)

// ListTransactions returns paginated transactions.
func (c *Client) ListTransactions(ctx context.Context, opts PageOpts) ([]Transaction, error) {
	var result []Transaction
	if err := c.get(ctx, "/transaction", pageQuery(opts), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// transactionsQuery renders the listing query for the global transaction path.
// offset_mode selects the offset-based listing (no cursors); a cursor combined
// with offset_mode is rejected client-side by fetchTransactions. Without
// offset_mode the request takes the cursor path (cursor param present, empty
// when unset), so a WithOffset position is silently overridden server-side —
// documented on ListTransactionsPage.
func transactionsQuery(p *listParams) url.Values {
	forceCursor := p.cursor != "" || p.offsetMode == ""
	q := p.listQuery(forceCursor)
	if p.offsetMode != "" {
		q.Set("offset_mode", p.offsetMode)
	}
	return q
}

// fetchTransactions fetches one page of the global transaction listing.
func (c *Client) fetchTransactions(ctx context.Context, p *listParams) ([]Transaction, *string, bool, error) {
	if err := p.reject("ListTransactionsPage", paramSide|paramSort); err != nil {
		return nil, nil, false, err
	}
	if p.set&(paramCursor|paramOffsetMode) == (paramCursor | paramOffsetMode) {
		return nil, nil, false, &RequestError{Option: "WithOffsetMode", Reason: "cannot be combined with WithCursor"}
	}
	page, err := getPage[Transaction](ctx, c, "/transaction", transactionsQuery(p))
	return pageResult(page, err)
}

// ListTransactionsPage returns one page of the global transaction listing as a
// cursor page; pass NextCursor to WithCursor (or use TransactionsPager) to
// continue the walk. The listing is ordered newest block first, transactions
// in block order within each block; Transaction.BlockID carries the hash of
// the confirming block ("" for pending/mempool transactions).
//
// Combining WithCursor with WithOffsetMode is rejected client-side with a
// *RequestError. WithOffsetMode (legacy or absolute) selects the
// offset-based listing instead, which has no cursors — such a page arrives as
// a bare array and is returned with a nil NextCursor. In the default (cursor)
// mode the cursor parameter is always present (empty on a first page), and the
// server silently overrides the WithOffset page position with the cursor — use
// WithOffsetMode for offset-based pages. Per-block transaction listings remain
// offset-based (see the block endpoints).
//
// Page stability is only guaranteed once the indexer's scanner is fully caught
// up; a walk during catch-up or a reorg may skip or repeat an entry.
func (c *Client) ListTransactionsPage(ctx context.Context, opts ...ListOption) (*CursorPage[Transaction], error) {
	p, err := applyListOptions(opts)
	if err != nil {
		return nil, err
	}
	items, next, _, err := c.fetchTransactions(ctx, p)
	if err != nil {
		return nil, err
	}
	return &CursorPage[Transaction]{Items: items, NextCursor: next}, nil
}

// TransactionsPager returns a pager that walks the global transaction listing
// (newest block first, transactions in block order within each block),
// following the server cursor automatically. It cannot be combined with
// WithOffsetMode; see ListTransactionsPage.
func TransactionsPager(c *Client, opts ...ListOption) *Pager[Transaction] {
	p, err := applyListOptions(opts)
	if err != nil {
		return fail[Transaction](err)
	}
	if err := p.reject("TransactionsPager", paramSide|paramSort|paramOffsetMode); err != nil {
		return fail[Transaction](err)
	}
	return NewPager(func(ctx context.Context, cursor *string) ([]Transaction, *string, bool, error) {
		fetch := p
		if cursor != nil {
			fetch = cloneParams(fetch, *cursor)
		}
		return c.fetchTransactions(ctx, fetch)
	})
}

// GetTransaction returns the transaction with the given id (hex).
// BlockID, Timestamp, and Confirmations are empty strings if the transaction
// is not yet confirmed.
func (c *Client) GetTransaction(ctx context.Context, id string) (*Transaction, error) {
	var result Transaction
	if err := c.get(ctx, fmt.Sprintf("/transaction/%s", id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetTransactionMerklePath returns the Merkle inclusion proof for a transaction.
// Returns an *HTTPError with StatusCode 404 if the transaction is not yet in a block.
func (c *Client) GetTransactionMerklePath(ctx context.Context, id string) (*MerklePath, error) {
	var result MerklePath
	if err := c.get(ctx, fmt.Sprintf("/transaction/%s/merkle-path", id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetTransactionOutput returns a single output by transaction id and output index.
// The returned JSON includes a "spent_at_block_height" field (null if unspent).
func (c *Client) GetTransactionOutput(ctx context.Context, txID string, idx uint32) (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.get(ctx, fmt.Sprintf("/transaction/%s/output/%d", txID, idx), nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SubmitTransaction submits a signed transaction (hex-encoded bytes) to the network.
// Returns the transaction id on success.
// Note: POST routes must be enabled on the server (--enable-post-routes).
func (c *Client) SubmitTransaction(ctx context.Context, signedTxHex string) (string, error) {
	var result struct {
		TxID string `json:"tx_id"`
	}
	if err := c.post(ctx, "/transaction", signedTxHex, &result); err != nil {
		return "", err
	}
	return result.TxID, nil
}
