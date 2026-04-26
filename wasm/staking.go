// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mintlayer

// EncodeStakePoolData encodes the parameters of a staking pool into binary form
// suitable for use in EncodeOutputCreateStakePool.
//
// staker is the bech32m address allowed to produce blocks.
// vrfPublicKey is the bech32m-encoded VRF public key for the pool.
// decommissionKey is the bech32m address that can decommission the pool.
// marginRatioPerThousand is the percentage of block rewards kept by the staker (0–1000).
// costPerBlock is a fixed amount subtracted from block rewards before margin calculation.
func (c *Client) EncodeStakePoolData(
	value Amount,
	staker, vrfPublicKey, decommissionKey string,
	marginRatioPerThousand uint32,
	costPerBlock Amount,
	network Network,
) ([]byte, error) {
	valPtr, err := c.newWASMAmount(value)
	if err != nil {
		return nil, err
	}
	stakerPtr, stakerLen, err := c.writeString(staker)
	if err != nil {
		return nil, err
	}
	vrfPtr, vrfLen, err := c.writeString(vrfPublicKey)
	if err != nil {
		return nil, err
	}
	decommPtr, decommLen, err := c.writeString(decommissionKey)
	if err != nil {
		return nil, err
	}
	cpbPtr, err := c.newWASMAmount(costPerBlock)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_stake_pool_data",
		uint64(valPtr),
		uint64(stakerPtr), uint64(stakerLen),
		uint64(vrfPtr), uint64(vrfLen),
		uint64(decommPtr), uint64(decommLen),
		uint64(marginRatioPerThousand),
		uint64(cpbPtr),
		uint64(network))
}

// EffectivePoolBalance computes the effective balance of a staking pool used for
// stake selection, given the pledge and total pool balance.
func (c *Client) EffectivePoolBalance(network Network, pledgeAmount, poolBalance Amount) (Amount, error) {
	pledgePtr, err := c.newWASMAmount(pledgeAmount)
	if err != nil {
		return Amount{}, err
	}
	poolPtr, err := c.newWASMAmount(poolBalance)
	if err != nil {
		return Amount{}, err
	}
	return c.callReturnAmountFallible("effective_pool_balance",
		uint64(network), uint64(pledgePtr), uint64(poolPtr))
}

// StakingPoolSpendMaturityBlockCount returns the number of blocks that must pass after a pool
// decommissions before its funds become spendable.
func (c *Client) StakingPoolSpendMaturityBlockCount(currentBlockHeight uint64, network Network) (uint64, error) {
	return c.callReturnU64("staking_pool_spend_maturity_block_count",
		currentBlockHeight, uint64(network))
}
