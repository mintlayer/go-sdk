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
	"time"
)

// ListPools returns staking pools with optional pagination and sorting.
func (c *Client) ListPools(ctx context.Context, opts PoolListOpts) ([]Pool, error) {
	q := pageQuery(opts.PageOpts)
	if opts.Sort != "" {
		q.Set("sort", opts.Sort)
	}
	var result []Pool
	if err := c.get(ctx, "/pool", q, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// poolsQuery renders the listing query for the cursor-paginated pools path.
// Only the default creation-height sort (SortByHeight, the server default)
// supports cursors: with any other sort value an explicit cursor is rejected
// by the server with 400 "Bad request", which the SDK propagates. Without an
// explicit cursor a non-default sort takes the offset-based path (bare array,
// no cursor), so a walk over it ends after one page.
func poolsQuery(p *listParams) url.Values {
	forceCursor := p.cursor != "" || p.sort == "" || p.sort == SortByHeight
	q := p.listQuery(forceCursor)
	if p.sort != "" {
		q.Set("sort", p.sort)
	}
	return q
}

// fetchPools fetches one page of the pools listing at p's cursor position.
func (c *Client) fetchPools(ctx context.Context, p *listParams) ([]Pool, *string, bool, error) {
	if err := p.reject("ListPoolsPage", paramSide|paramOffsetMode); err != nil {
		return nil, nil, false, err
	}
	page, err := getPage[Pool](ctx, c, "/pool", poolsQuery(p))
	return pageResult(page, err)
}

// ListPoolsPage returns one page of staking pools (newest creation height
// first by default) as a cursor page; pass NextCursor to WithCursor (or use
// PoolsPager) to continue the walk. Unlike ListPools, this method always uses
// the cursor envelope (except for a non-default sort without an explicit
// cursor — see WithSort).
//
// The default sort is the only cursor-compatible one: combining WithCursor
// with WithSort(SortByPledge) is rejected by the server with 400 "Bad
// request" and propagated as *HTTPError — only PoolsPager validates this
// combination client-side (with a *RequestError), matching the api-server
// contract this method mirrors.
//
// When WithOffset is set without a cursor, the cursor silently overrides the
// offset page position server-side (items still applies); the SDK sends both
// parameters and does not alter that behaviour.
//
// Page stability is only guaranteed once the indexer's scanner is fully caught
// up; a walk during catch-up or a reorg may skip or repeat an entry.
func (c *Client) ListPoolsPage(ctx context.Context, opts ...ListOption) (*CursorPage[Pool], error) {
	p, err := applyListOptions(opts)
	if err != nil {
		return nil, err
	}
	items, next, _, err := c.fetchPools(ctx, p)
	if err != nil {
		return nil, err
	}
	return &CursorPage[Pool]{Items: items, NextCursor: next}, nil
}

// PoolsPager returns a pager that walks the pools listing (newest creation
// height first by default), following the server cursor automatically. The
// walk only works with the default creation-height sort; see ListPoolsPage
// and WithSort for the sort/cursor restriction.
func PoolsPager(c *Client, opts ...ListOption) *Pager[Pool] {
	p, err := applyListOptions(opts)
	if err != nil {
		return fail[Pool](err)
	}
	if err := p.reject("PoolsPager", paramSide|paramOffsetMode); err != nil {
		return fail[Pool](err)
	}
	if p.sort != "" && p.sort != SortByHeight {
		return fail[Pool](&RequestError{Option: "WithSort", Reason: "pools cursor walks require the default by_height sort; use ListPoolsPage for other sort orders"})
	}
	return NewPager(func(ctx context.Context, cursor *string) ([]Pool, *string, bool, error) {
		fetch := p
		if cursor != nil {
			fetch = cloneParams(fetch, *cursor)
		}
		return c.fetchPools(ctx, fetch)
	})
}

// GetPool returns a single staking pool by id (bech32).
func (c *Client) GetPool(ctx context.Context, id string) (*Pool, error) {
	var result Pool
	if err := c.get(ctx, fmt.Sprintf("/pool/%s", id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetPoolBlockStats returns the number of blocks produced by a pool
// in the half-open interval [from, to).
func (c *Client) GetPoolBlockStats(ctx context.Context, id string, from, to time.Time) (uint64, error) {
	q := url.Values{}
	q.Set("from", strconv.FormatInt(from.Unix(), 10))
	q.Set("to", strconv.FormatInt(to.Unix(), 10))

	var result struct {
		BlockCount uint64 `json:"block_count"`
	}
	if err := c.get(ctx, fmt.Sprintf("/pool/%s/block-stats", id), q, &result); err != nil {
		return 0, err
	}
	return result.BlockCount, nil
}

// GetPoolDelegations returns all delegations in a pool.
func (c *Client) GetPoolDelegations(ctx context.Context, id string) ([]PoolDelegation, error) {
	var result []PoolDelegation
	if err := c.get(ctx, fmt.Sprintf("/pool/%s/delegations", id), nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}
