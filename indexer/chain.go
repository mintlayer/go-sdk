package indexer

import (
	"context"
	"fmt"
)

// GetTip returns the current chain tip (highest confirmed block).
func (c *Client) GetTip(ctx context.Context) (*ChainTip, error) {
	var result ChainTip
	if err := c.get(ctx, "/chain/tip", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetGenesis returns the genesis block info.
func (c *Client) GetGenesis(ctx context.Context) (*GenesisInfo, error) {
	var result GenesisInfo
	if err := c.get(ctx, "/chain/genesis", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetBlockIDAtHeight returns the block id (hex) at the given mainchain height.
// Returns an *HTTPError with StatusCode 404 if no block exists at that height.
func (c *Client) GetBlockIDAtHeight(ctx context.Context, height uint64) (string, error) {
	var result string
	if err := c.get(ctx, fmt.Sprintf("/chain/%d", height), nil, &result); err != nil {
		return "", err
	}
	return result, nil
}
