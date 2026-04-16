package wallet

import "context"

// IssueToken issues a new fungible token. After issuance, tokens must be minted before
// they can be transferred.
func (c *Client) IssueToken(ctx context.Context, params IssueTokenParams) (*IssueTokenResult, error) {
	var result IssueTokenResult
	if err := c.call(ctx, "token_issue_new", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// IssueNFT issues a non-fungible token. The NFT is immediately sent to DestinationAddress
// and cannot be minted again.
func (c *Client) IssueNFT(ctx context.Context, params IssueNFTParams) (*IssueTokenResult, error) {
	var result IssueTokenResult
	if err := c.call(ctx, "token_nft_issue_new", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// MintTokens mints tokens into the circulating supply.
// The authority key must be held by the selected account.
func (c *Client) MintTokens(ctx context.Context, params MintParams) (*SendResult, error) {
	var result SendResult
	if err := c.call(ctx, "token_mint", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UnmintTokens unmints tokens, reducing the circulating supply.
// The wallet must hold both the tokens and the authority key.
func (c *Client) UnmintTokens(ctx context.Context, params UnmintParams) (*SendResult, error) {
	var result SendResult
	if err := c.call(ctx, "token_unmint", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// LockTokenSupply permanently locks the token supply at the current circulating amount.
// This is irreversible and only available for tokens issued with Lockable supply.
// Note: params uses AccountIndex (not Account) to match the wire format.
func (c *Client) LockTokenSupply(ctx context.Context, params LockSupplyParams) (*SendResult, error) {
	var result SendResult
	if err := c.call(ctx, "token_lock_supply", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// FreezeToken freezes a token, blocking all operations.
// Requires the token to have been issued with IsFreezable: true.
func (c *Client) FreezeToken(ctx context.Context, params FreezeParams) (*SendResult, error) {
	var result SendResult
	if err := c.call(ctx, "token_freeze", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UnfreezeToken unfreezes a token. Only possible if frozen with IsUnfreezable: true.
func (c *Client) UnfreezeToken(ctx context.Context, params UnfreezeParams) (*SendResult, error) {
	var result SendResult
	if err := c.call(ctx, "token_unfreeze", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ChangeTokenAuthority transfers the authority address to a new key.
func (c *Client) ChangeTokenAuthority(ctx context.Context, params ChangeAuthorityParams) (*SendResult, error) {
	var result SendResult
	if err := c.call(ctx, "token_change_authority", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SendToken sends a token amount to an address. Fees are paid in TML.
// This is an alias for TokenSend in the transactions module; both call token_send.
func (c *Client) SendToken(ctx context.Context, params TokenSendParams) (*SendResult, error) {
	var result SendResult
	if err := c.call(ctx, "token_send", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
