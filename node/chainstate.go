// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package node

import (
	"context"
	"encoding/json"
)

// ChainstateInfo returns a summary of chain state: best block, height,
// timestamp, median time, and IBD flag.
func (c *Client) ChainstateInfo(ctx context.Context) (*ChainstateInfo, error) {
	var result ChainstateInfo
	if err := c.call(ctx, "chainstate_info", struct{}{}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// BestBlockID returns the current tip block ID as a hex string.
func (c *Client) BestBlockID(ctx context.Context) (string, error) {
	var result string
	if err := c.call(ctx, "chainstate_best_block_id", struct{}{}, &result); err != nil {
		return "", err
	}
	return result, nil
}

// BestBlockHeight returns the current tip block height.
func (c *Client) BestBlockHeight(ctx context.Context) (uint64, error) {
	var result uint64
	if err := c.call(ctx, "chainstate_best_block_height", struct{}{}, &result); err != nil {
		return 0, err
	}
	return result, nil
}

// BlockIDAtHeight returns the block ID at the given mainchain height.
// Returns nil if no block exists at that height.
func (c *Client) BlockIDAtHeight(ctx context.Context, height uint64) (*string, error) {
	var result *string
	params := struct {
		Height uint64 `json:"height"`
	}{Height: height}
	if err := c.call(ctx, "chainstate_block_id_at_height", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// BlockHeightInMainChain returns the mainchain height for a block ID.
// Returns nil if the block is not in the mainchain.
func (c *Client) BlockHeightInMainChain(ctx context.Context, blockID string) (*uint64, error) {
	var result *uint64
	params := struct {
		BlockID string `json:"block_id"`
	}{BlockID: blockID}
	if err := c.call(ctx, "chainstate_block_height_in_main_chain", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetBlock returns the hex-encoded serialized bytes of a block.
// Returns nil if the block is not found. Genesis cannot be retrieved here.
func (c *Client) GetBlock(ctx context.Context, id string) (*string, error) {
	var result *string
	params := struct {
		ID string `json:"id"`
	}{ID: id}
	if err := c.call(ctx, "chainstate_get_block", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetBlockJSON returns the block parsed as a JSON object.
// Returns nil if the block is not found.
func (c *Client) GetBlockJSON(ctx context.Context, id string) (json.RawMessage, error) {
	var result json.RawMessage
	params := struct {
		ID string `json:"id"`
	}{ID: id}
	if err := c.call(ctx, "chainstate_get_block_json", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetMainchainBlocks returns up to maxCount consecutive mainchain blocks
// starting at height from, each as a hex-encoded byte string.
func (c *Client) GetMainchainBlocks(ctx context.Context, from uint64, maxCount uint32) ([]string, error) {
	var result []string
	params := struct {
		From     uint64 `json:"from"`
		MaxCount uint32 `json:"max_count"`
	}{From: from, MaxCount: maxCount}
	if err := c.call(ctx, "chainstate_get_mainchain_blocks", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetUTXO returns the TxOutput at a UTXO outpoint as raw JSON.
// Returns nil if the outpoint is not found or already spent.
func (c *Client) GetUTXO(ctx context.Context, outpoint Outpoint) (json.RawMessage, error) {
	var result json.RawMessage
	params := struct {
		Outpoint Outpoint `json:"outpoint"`
	}{Outpoint: outpoint}
	if err := c.call(ctx, "chainstate_get_utxo", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// StakePoolBalance returns the total balance of a pool (staker + all delegations).
// Returns nil if the pool is not found.
func (c *Client) StakePoolBalance(ctx context.Context, poolAddress string) (*Amount, error) {
	var result *Amount
	params := struct {
		PoolAddress string `json:"pool_address"`
	}{PoolAddress: poolAddress}
	if err := c.call(ctx, "chainstate_stake_pool_balance", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// StakerBalance returns only the staker (pool owner) portion of a pool's balance,
// excluding delegations. Returns nil if the pool is not found.
func (c *Client) StakerBalance(ctx context.Context, poolAddress string) (*Amount, error) {
	var result *Amount
	params := struct {
		PoolAddress string `json:"pool_address"`
	}{PoolAddress: poolAddress}
	if err := c.call(ctx, "chainstate_staker_balance", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// PoolDecommissionDestination returns the decommission address (bech32) for a pool.
// Returns nil if the pool is not found.
func (c *Client) PoolDecommissionDestination(ctx context.Context, poolAddress string) (*string, error) {
	var result *string
	params := struct {
		PoolAddress string `json:"pool_address"`
	}{PoolAddress: poolAddress}
	if err := c.call(ctx, "chainstate_pool_decommission_destination", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// DelegationShare returns the amount owned by a delegation in a pool.
// Returns nil if not found.
func (c *Client) DelegationShare(ctx context.Context, poolAddress, delegationAddress string) (*Amount, error) {
	var result *Amount
	params := struct {
		PoolAddress       string `json:"pool_address"`
		DelegationAddress string `json:"delegation_address"`
	}{PoolAddress: poolAddress, DelegationAddress: delegationAddress}
	if err := c.call(ctx, "chainstate_delegation_share", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// TokenInfo returns token info by token id (bech32). Returns nil if not found.
func (c *Client) TokenInfo(ctx context.Context, tokenID string) (*TokenInfo, error) {
	var result *TokenInfo
	params := struct {
		TokenID string `json:"token_id"`
	}{TokenID: tokenID}
	if err := c.call(ctx, "chainstate_token_info", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// TokensInfo is the batch version of TokenInfo. Returns one entry per requested
// token id in the same order.
func (c *Client) TokensInfo(ctx context.Context, tokenIDs []string) ([]TokenInfo, error) {
	var result []TokenInfo
	params := struct {
		TokenIDs []string `json:"token_ids"`
	}{TokenIDs: tokenIDs}
	if err := c.call(ctx, "chainstate_tokens_info", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// OrderInfo returns the current state of an on-chain order.
// Returns nil if the order is not found.
func (c *Client) OrderInfo(ctx context.Context, orderID string) (*OrderInfo, error) {
	var result *OrderInfo
	params := struct {
		OrderID string `json:"order_id"`
	}{OrderID: orderID}
	if err := c.call(ctx, "chainstate_order_info", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// OrdersInfoByCurrencies returns all orders matching the given ask/give currencies.
// Pass nil for either currency to match any. The returned map key is the order id (hex).
func (c *Client) OrdersInfoByCurrencies(ctx context.Context, ask, give *Currency) (map[string]OrderInfo, error) {
	var result map[string]OrderInfo
	params := struct {
		AskCurrency *Currency `json:"ask_currency"`
		GiveCurrency *Currency `json:"give_currency"`
	}{AskCurrency: ask, GiveCurrency: give}
	if err := c.call(ctx, "chainstate_orders_info_by_currencies", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SubmitBlock submits a fully serialized block (hex) to the node.
// Validation is still enforced; used by stakers after producing a block.
func (c *Client) SubmitBlock(ctx context.Context, blockHex string) error {
	params := struct {
		BlockHex string `json:"block_hex"`
	}{BlockHex: blockHex}
	return c.call(ctx, "chainstate_submit_block", params, nil)
}
