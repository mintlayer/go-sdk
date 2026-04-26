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
