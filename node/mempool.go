// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package node

import "context"

// ContainsTx reports whether a transaction is in the mempool (not the orphan pool).
func (c *Client) ContainsTx(ctx context.Context, txID string) (bool, error) {
	var result bool
	params := struct {
		TxID string `json:"tx_id"`
	}{TxID: txID}
	if err := c.call(ctx, "mempool_contains_tx", params, &result); err != nil {
		return false, err
	}
	return result, nil
}

// ContainsOrphanTx reports whether a transaction is in the orphan pool
// (its inputs are not yet present in the UTXO set).
func (c *Client) ContainsOrphanTx(ctx context.Context, txID string) (bool, error) {
	var result bool
	params := struct {
		TxID string `json:"tx_id"`
	}{TxID: txID}
	if err := c.call(ctx, "mempool_contains_orphan_tx", params, &result); err != nil {
		return false, err
	}
	return result, nil
}

// GetTransaction returns the mempool entry for a transaction.
// Returns nil if the transaction is not found in the mempool or orphan pool.
func (c *Client) GetTransaction(ctx context.Context, txID string) (*MempoolTx, error) {
	var result *MempoolTx
	params := struct {
		TxID string `json:"tx_id"`
	}{TxID: txID}
	if err := c.call(ctx, "mempool_get_transaction", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// MempoolSubmitTransaction submits a signed transaction to the local mempool only.
// It does NOT broadcast to the P2P network; use P2PSubmitTransaction for that.
func (c *Client) MempoolSubmitTransaction(ctx context.Context, txHex string, trustPolicy TrustPolicy) error {
	params := struct {
		Tx      string `json:"tx"`
		Options struct {
			TrustPolicy TrustPolicy `json:"trust_policy"`
		} `json:"options"`
	}{}
	params.Tx = txHex
	params.Options.TrustPolicy = trustPolicy
	return c.call(ctx, "mempool_submit_transaction", params, nil)
}

// GetFeeRate returns the fee rate that places a transaction in the top inTopXMb
// megabytes of the mempool.
func (c *Client) GetFeeRate(ctx context.Context, inTopXMb uint32) (*FeeRate, error) {
	var result FeeRate
	params := struct {
		InTopXMb uint32 `json:"in_top_x_mb"`
	}{InTopXMb: inTopXMb}
	if err := c.call(ctx, "mempool_get_fee_rate", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetFeeRatePoints returns all data points of the mempool fee-rate curve.
func (c *Client) GetFeeRatePoints(ctx context.Context) ([]FeeRatePoint, error) {
	var result []FeeRatePoint
	if err := c.call(ctx, "mempool_get_fee_rate_points", struct{}{}, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// MemoryUsage returns the estimated memory used by the mempool in bytes.
func (c *Client) MemoryUsage(ctx context.Context) (uint64, error) {
	var result uint64
	if err := c.call(ctx, "mempool_memory_usage", struct{}{}, &result); err != nil {
		return 0, err
	}
	return result, nil
}
