// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package indexer

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// GetCoinStatistics returns aggregate ML coin supply statistics.
func (c *Client) GetCoinStatistics(ctx context.Context) (*CoinStats, error) {
	var result CoinStats
	if err := c.get(ctx, "/statistics/coin", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetTokenStatistics returns aggregate supply statistics for a specific token (bech32 id).
func (c *Client) GetTokenStatistics(ctx context.Context, tokenID string) (*CoinStats, error) {
	var result CoinStats
	if err := c.get(ctx, fmt.Sprintf("/statistics/token/%s", tokenID), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetFeeRate returns the current fee rate (atoms per kilobyte) required to be in the
// top inTopXMb megabytes of the mempool. Pass 0 to use the server default (5 MB).
func (c *Client) GetFeeRate(ctx context.Context, inTopXMb uint32) (string, error) {
	q := url.Values{}
	if inTopXMb > 0 {
		q.Set("in_top_x_mb", strconv.FormatUint(uint64(inTopXMb), 10))
	}
	var result string
	if err := c.get(ctx, "/feerate", q, &result); err != nil {
		return "", err
	}
	return result, nil
}

// fetchCoinHolders fetches one page of the native coin holders listing.
func (c *Client) fetchCoinHolders(ctx context.Context, p *listParams) ([]Holder, *string, bool, error) {
	if err := p.reject("ListCoinHolders", paramSide|paramSort|paramOffsetMode); err != nil {
		return nil, nil, false, err
	}
	// The listing is keyset-only: the server parses but ignores the offset, and
	// a present cursor silently overrides the page position. The SDK sends the
	// parameters as-is and leaves the server behaviour untouched.
	page, err := getPage[Holder](ctx, c, "/statistics/coin/holders", p.listQuery(true))
	return pageResult(page, err)
}

// fetchTokenHolders fetches one page of a token's holders listing.
func (c *Client) fetchTokenHolders(ctx context.Context, tokenID string, p *listParams) ([]Holder, *string, bool, error) {
	if err := p.reject("ListTokenHolders", paramSide|paramSort|paramOffsetMode); err != nil {
		return nil, nil, false, err
	}
	if tokenID == "" {
		return nil, nil, false, &RequestError{Option: "tokenID", Reason: "must not be empty"}
	}
	page, err := getPage[Holder](ctx, c, fmt.Sprintf("/statistics/token/%s/holders", url.PathEscape(tokenID)), p.listQuery(true))
	return pageResult(page, err)
}

// ListCoinHolders returns one page of the native coin holders, largest
// balance first, as an address-balance listing. The result is a cursor page:
// pass NextCursor to WithCursor (or use CoinHoldersPager) to continue the
// walk. There is no offset mode for this endpoint — the server ignores the
// offset parameter, and the cursor defines the page position.
//
// Page stability is only guaranteed once the indexer's scanner is fully caught
// up; a walk during catch-up or a reorg may skip or repeat an entry.
func (c *Client) ListCoinHolders(ctx context.Context, opts ...ListOption) (*CursorPage[Holder], error) {
	p, err := applyListOptions(opts)
	if err != nil {
		return nil, err
	}
	items, next, _, err := c.fetchCoinHolders(ctx, p)
	if err != nil {
		return nil, err
	}
	return &CursorPage[Holder]{Items: items, NextCursor: next}, nil
}

// ListTokenHolders returns one page of the holders of the token with the given
// id (bech32, case-sensitive — pass it exactly as issued; the SDK never
// changes its case). Unknown token ids fail with an *HTTPError matching
// ErrTokenNotFound. See ListCoinHolders for paging semantics.
func (c *Client) ListTokenHolders(ctx context.Context, tokenID string, opts ...ListOption) (*CursorPage[Holder], error) {
	p, err := applyListOptions(opts)
	if err != nil {
		return nil, err
	}
	items, next, _, err := c.fetchTokenHolders(ctx, tokenID, p)
	if err != nil {
		return nil, err
	}
	return &CursorPage[Holder]{Items: items, NextCursor: next}, nil
}

// CoinHoldersPager returns a pager that walks the native coin holders listing
// (largest balance first), following the server cursor automatically.
func CoinHoldersPager(c *Client, opts ...ListOption) *Pager[Holder] {
	p, err := applyListOptions(opts)
	if err != nil {
		return fail[Holder](err)
	}
	if err := p.reject("CoinHoldersPager", paramSide|paramSort|paramOffsetMode); err != nil {
		return fail[Holder](err)
	}
	return NewPager(func(ctx context.Context, cursor *string) ([]Holder, *string, bool, error) {
		fetch := p
		if cursor != nil {
			fetch = cloneParams(fetch, *cursor)
		}
		return c.fetchCoinHolders(ctx, fetch)
	})
}

// TokenHoldersPager returns a pager that walks the holders listing of the
// token with the given id (bech32, case-sensitive).
func TokenHoldersPager(c *Client, tokenID string, opts ...ListOption) *Pager[Holder] {
	p, err := applyListOptions(opts)
	if err == nil {
		err = p.reject("TokenHoldersPager", paramSide|paramSort|paramOffsetMode)
	}
	if err == nil && tokenID == "" {
		err = &RequestError{Option: "tokenID", Reason: "must not be empty"}
	}
	if err != nil {
		return fail[Holder](err)
	}
	return NewPager(func(ctx context.Context, cursor *string) ([]Holder, *string, bool, error) {
		fetch := p
		if cursor != nil {
			fetch = cloneParams(fetch, *cursor)
		}
		return c.fetchTokenHolders(ctx, tokenID, fetch)
	})
}
