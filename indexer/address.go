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

// GetAddressInfo returns the balance and transaction history for a bech32 address.
// Returns an *HTTPError with StatusCode 404 if the address has no transaction history.
func (c *Client) GetAddressInfo(ctx context.Context, address string) (*AddressInfo, error) {
	var result AddressInfo
	if err := c.get(ctx, fmt.Sprintf("/address/%s", address), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetSpendableUTXOs returns the currently spendable (confirmed, unspent) UTXOs for an address.
func (c *Client) GetSpendableUTXOs(ctx context.Context, address string) ([]UTXO, error) {
	var result []UTXO
	if err := c.get(ctx, fmt.Sprintf("/address/%s/spendable-utxos", address), nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetAllUTXOs returns all UTXOs (including locked/unspendable) for an address.
func (c *Client) GetAllUTXOs(ctx context.Context, address string) ([]UTXO, error) {
	var result []UTXO
	if err := c.get(ctx, fmt.Sprintf("/address/%s/all-utxos", address), nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetDelegations returns all delegations owned by an address.
func (c *Client) GetDelegations(ctx context.Context, address string) ([]DelegationInfo, error) {
	var result []DelegationInfo
	if err := c.get(ctx, fmt.Sprintf("/address/%s/delegations", address), nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetTokenAuthority returns the ids (bech32) of fungible tokens for which the address
// is the authority (can mint, freeze, etc.).
func (c *Client) GetTokenAuthority(ctx context.Context, address string) ([]string, error) {
	var result []string
	if err := c.get(ctx, fmt.Sprintf("/address/%s/token-authority", address), nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}
