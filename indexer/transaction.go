package indexer

import (
	"context"
	"encoding/json"
	"fmt"
)

// ListTransactions returns paginated transactions.
func (c *Client) ListTransactions(ctx context.Context, opts PageOpts) ([]Transaction, error) {
	var result []Transaction
	if err := c.get(ctx, "/transaction", pageQuery(opts), &result); err != nil {
		return nil, err
	}
	return result, nil
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
