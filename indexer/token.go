// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package indexer

import (
	"context"
	"fmt"
)

// ListTokens returns fungible token ids with pagination.
func (c *Client) ListTokens(ctx context.Context, opts PageOpts) ([]string, error) {
	var result []string
	if err := c.get(ctx, "/token", pageQuery(opts), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetToken returns fungible token details by token id (bech32).
func (c *Client) GetToken(ctx context.Context, id string) (*TokenInfo, error) {
	var result TokenInfo
	if err := c.get(ctx, fmt.Sprintf("/token/%s", id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetTokenTransactions returns transactions involving a token, with pagination.
func (c *Client) GetTokenTransactions(ctx context.Context, id string, opts PageOpts) ([]TokenTx, error) {
	var result []TokenTx
	if err := c.get(ctx, fmt.Sprintf("/token/%s/transactions", id), pageQuery(opts), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// FindTokensByTicker returns token ids (bech32) that match the given ticker string.
func (c *Client) FindTokensByTicker(ctx context.Context, ticker string, opts PageOpts) ([]string, error) {
	var result []string
	if err := c.get(ctx, fmt.Sprintf("/token/ticker/%s", ticker), pageQuery(opts), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetNFT returns NFT details by token id (bech32).
func (c *Client) GetNFT(ctx context.Context, id string) (*NFTInfo, error) {
	var result NFTInfo
	if err := c.get(ctx, fmt.Sprintf("/nft/%s", id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
