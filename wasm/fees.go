// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mintlayer

// FungibleTokenIssuanceFee returns the fee required to issue a new fungible token
// at the given block height.
func (c *Client) FungibleTokenIssuanceFee(currentBlockHeight uint64, network Network) (Amount, error) {
	return c.callReturnAmount("fungible_token_issuance_fee", currentBlockHeight, uint64(network))
}

// NftIssuanceFee returns the fee required to issue a new NFT at the given block height.
func (c *Client) NftIssuanceFee(currentBlockHeight uint64, network Network) (Amount, error) {
	return c.callReturnAmount("nft_issuance_fee", currentBlockHeight, uint64(network))
}

// DataDepositFee returns the fee required to create a DataDeposit output at the given block height.
func (c *Client) DataDepositFee(currentBlockHeight uint64, network Network) (Amount, error) {
	return c.callReturnAmount("data_deposit_fee", currentBlockHeight, uint64(network))
}

// TokenSupplyChangeFee returns the fee required to mint or unmint tokens at the given block height.
func (c *Client) TokenSupplyChangeFee(currentBlockHeight uint64, network Network) (Amount, error) {
	return c.callReturnAmount("token_supply_change_fee", currentBlockHeight, uint64(network))
}

// TokenFreezeFee returns the fee required to freeze or unfreeze a token at the given block height.
func (c *Client) TokenFreezeFee(currentBlockHeight uint64, network Network) (Amount, error) {
	return c.callReturnAmount("token_freeze_fee", currentBlockHeight, uint64(network))
}

// TokenChangeAuthorityFee returns the fee required to change a token's authority at the given block height.
func (c *Client) TokenChangeAuthorityFee(currentBlockHeight uint64, network Network) (Amount, error) {
	return c.callReturnAmount("token_change_authority_fee", currentBlockHeight, uint64(network))
}
