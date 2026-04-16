package wallet

import "context"

// CreateStakePool creates a new staking pool. The pool can produce blocks and accept delegations.
func (c *Client) CreateStakePool(ctx context.Context, params CreatePoolParams) (*SendResult, error) {
	var result SendResult
	if err := c.call(ctx, "staking_create_pool", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DecommissionStakePool decommissions a pool whose decommission key is held by this wallet.
// Returns the pledge and rewards to OutputAddress in params.
func (c *Client) DecommissionStakePool(ctx context.Context, params DecommissionParams) (*SendResult, error) {
	var result SendResult
	if err := c.call(ctx, "staking_decommission_pool", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListOwnedPools lists all pools whose staking key is controlled by the account.
func (c *Client) ListOwnedPools(ctx context.Context, account uint32) ([]OwnedPool, error) {
	var result []OwnedPool
	params := struct {
		Account uint32 `json:"account"`
	}{Account: account}
	if err := c.call(ctx, "staking_list_pools", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetPoolBalance returns the current balance of a specific staking pool.
// The account parameter is accepted for API consistency but not forwarded to the daemon.
func (c *Client) GetPoolBalance(ctx context.Context, account uint32, poolID string) (*Amount, error) {
	var result struct {
		Balance *Amount `json:"balance"`
	}
	params := struct {
		PoolID string `json:"pool_id"`
	}{PoolID: poolID}
	if err := c.call(ctx, "staking_pool_balance", params, &result); err != nil {
		return nil, err
	}
	return result.Balance, nil
}

// StartStaking starts producing blocks with the pools in the selected account.
func (c *Client) StartStaking(ctx context.Context, account uint32) error {
	params := struct {
		Account uint32 `json:"account"`
	}{Account: account}
	return c.call(ctx, "staking_start", params, nil)
}

// StopStaking stops block production. Does not affect pools or delegations.
func (c *Client) StopStaking(ctx context.Context, account uint32) error {
	params := struct {
		Account uint32 `json:"account"`
	}{Account: account}
	return c.call(ctx, "staking_stop", params, nil)
}

// GetStakingStatus returns whether staking is currently active for the account.
// Returns StakingStatusActive ("Staking") or StakingStatusInactive ("NotStaking").
func (c *Client) GetStakingStatus(ctx context.Context, account uint32) (*StakingStatus, error) {
	var result StakingStatus
	params := struct {
		Account uint32 `json:"account"`
	}{Account: account}
	if err := c.call(ctx, "staking_status", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateDelegation creates a delegation to a pool.
// The Address in params is the owner — the key authorized to withdraw from the delegation.
func (c *Client) CreateDelegation(ctx context.Context, params CreateDelegationParams) (*CreateDelegationResult, error) {
	var result CreateDelegationResult
	if err := c.call(ctx, "delegation_create", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DelegateStaking sends coins to a delegation id to begin staking them.
func (c *Client) DelegateStaking(ctx context.Context, params DelegateParams) (*SendResult, error) {
	var result SendResult
	if err := c.call(ctx, "delegation_stake", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// WithdrawFromDelegation withdraws coins from a delegation.
// Withdrawn coins have a lock period before they become spendable.
func (c *Client) WithdrawFromDelegation(ctx context.Context, params WithdrawParams) (*SendResult, error) {
	var result SendResult
	if err := c.call(ctx, "delegation_withdraw", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListDelegations lists delegation ids controlled by the account with their pool and balance.
func (c *Client) ListDelegations(ctx context.Context, account uint32) ([]DelegationInfo, error) {
	var result []DelegationInfo
	params := struct {
		Account uint32 `json:"account"`
	}{Account: account}
	if err := c.call(ctx, "delegation_list_ids", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}
