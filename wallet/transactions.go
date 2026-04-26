// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package wallet

import (
	"context"
	"encoding/json"
)

// AddressSend sends coins to an address. Fees are calculated automatically.
func (c *Client) AddressSend(ctx context.Context, params SendParams) (*SendResult, error) {
	var result SendResult
	if err := c.call(ctx, "address_send", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// TokenSend sends a token amount to an address. Fees are paid in TML.
func (c *Client) TokenSend(ctx context.Context, params TokenSendParams) (*SendResult, error) {
	var result SendResult
	if err := c.call(ctx, "token_send", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SweepSpendable sweeps all spendable coins and tokens from one or more addresses to a destination.
// Set All to true to sweep all addresses in the account; otherwise list source addresses in FromAddresses.
func (c *Client) SweepSpendable(ctx context.Context, params SweepParams) (*SendResult, error) {
	var result SendResult
	if err := c.call(ctx, "address_sweep_spendable", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SpendUTXO spends a specific UTXO, moving its funds to an address.
func (c *Client) SpendUTXO(ctx context.Context, params UTXOSpendParams) (*SendResult, error) {
	var result SendResult
	if err := c.call(ctx, "utxo_spend", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ComposeTransaction composes a raw transaction from explicit inputs and outputs without signing it.
// Returns a hex-encoded PartiallySignedTransaction that can be passed to SignRawTransaction.
func (c *Client) ComposeTransaction(ctx context.Context, params ComposeParams) (*ComposedTx, error) {
	var result ComposedTx
	if err := c.call(ctx, "transaction_compose", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SignRawTransaction signs a hex-encoded transaction or PartiallySignedTransaction.
// Used for multisig and cold wallet workflows.
func (c *Client) SignRawTransaction(ctx context.Context, account uint32, rawTx string) (*SignedTx, error) {
	var result SignedTx
	params := struct {
		Account uint32    `json:"account"`
		RawTx   string    `json:"raw_tx"`
		Options TxOptions `json:"options"`
	}{Account: account, RawTx: rawTx}
	if err := c.call(ctx, "account_sign_raw_transaction", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// InspectTransaction inspects a raw transaction hex — shows inputs, outputs, fees, and signature
// status without broadcasting.
func (c *Client) InspectTransaction(ctx context.Context, txHex string) (*TxInspection, error) {
	var result TxInspection
	params := struct {
		Transaction string `json:"transaction"`
	}{Transaction: txHex}
	if err := c.call(ctx, "transaction_inspect", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SubmitTransaction submits a fully signed hex transaction to the mempool and broadcasts it.
// Set doNotStore to true to skip storing the transaction in the wallet database.
func (c *Client) SubmitTransaction(ctx context.Context, txHex string, doNotStore bool) (*SubmitResult, error) {
	var result SubmitResult
	params := struct {
		Tx         string `json:"tx"`
		DoNotStore bool   `json:"do_not_store"`
		Options    struct {
			TrustPolicy string `json:"trust_policy"`
		} `json:"options"`
	}{Tx: txHex, DoNotStore: doNotStore}
	params.Options.TrustPolicy = "Trusted"
	if err := c.call(ctx, "node_submit_transaction", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListTransactionsByAddress lists confirmed transactions for an account, optionally filtered by address.
// Pass a nil address to list all transactions. limit controls the maximum number of results.
func (c *Client) ListTransactionsByAddress(ctx context.Context, account uint32, address *string, limit uint32) ([]WalletTx, error) {
	var result []WalletTx
	params := struct {
		Account uint32  `json:"account"`
		Address *string `json:"address"`
		Limit   uint32  `json:"limit"`
	}{Account: account, Address: address, Limit: limit}
	if err := c.call(ctx, "transaction_list_by_address", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListPendingTransactions lists pending (unconfirmed) transaction ids that can be abandoned.
func (c *Client) ListPendingTransactions(ctx context.Context, account uint32) ([]string, error) {
	var result []string
	params := struct {
		Account uint32 `json:"account"`
	}{Account: account}
	if err := c.call(ctx, "transaction_list_pending", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetTransaction returns a transaction from the wallet as raw JSON.
func (c *Client) GetTransaction(ctx context.Context, account uint32, txID string) (json.RawMessage, error) {
	var result json.RawMessage
	params := struct {
		Account       uint32 `json:"account"`
		TransactionID string `json:"transaction_id"`
	}{Account: account, TransactionID: txID}
	if err := c.call(ctx, "transaction_get", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// AbandonTransaction abandons an unconfirmed transaction, releasing its inputs for reuse.
func (c *Client) AbandonTransaction(ctx context.Context, account uint32, txID string) error {
	params := struct {
		Account       uint32 `json:"account"`
		TransactionID string `json:"transaction_id"`
	}{Account: account, TransactionID: txID}
	return c.call(ctx, "transaction_abandon", params, nil)
}

// DepositData stores arbitrary data on the blockchain as a hex-encoded byte string.
// Note: this incurs a higher-than-normal fee.
func (c *Client) DepositData(ctx context.Context, account uint32, dataHex string) (*SendResult, error) {
	var result SendResult
	params := struct {
		Account uint32    `json:"account"`
		Data    string    `json:"data"`
		Options TxOptions `json:"options"`
	}{Account: account, Data: dataHex}
	if err := c.call(ctx, "address_deposit_data", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
