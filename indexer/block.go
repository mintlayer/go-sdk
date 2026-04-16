package indexer

import (
	"context"
	"fmt"
)

// GetBlock returns the full block data for the given block id (hex).
func (c *Client) GetBlock(ctx context.Context, id string) (*Block, error) {
	var result Block
	if err := c.get(ctx, fmt.Sprintf("/block/%s", id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetBlockHeader returns only the header for the given block id (hex).
func (c *Client) GetBlockHeader(ctx context.Context, id string) (*BlockHeader, error) {
	var result BlockHeader
	if err := c.get(ctx, fmt.Sprintf("/block/%s/header", id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetBlockReward returns the reward outputs for the given block id (hex).
func (c *Client) GetBlockReward(ctx context.Context, id string) ([]TxOutput, error) {
	var result []TxOutput
	if err := c.get(ctx, fmt.Sprintf("/block/%s/reward", id), nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetBlockTransactionIDs returns all transaction ids in the given block (hex block id).
func (c *Client) GetBlockTransactionIDs(ctx context.Context, id string) ([]string, error) {
	var result []string
	if err := c.get(ctx, fmt.Sprintf("/block/%s/transaction-ids", id), nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}
