package mintlayer

// EncodeOutputTransfer creates a Transfer output sending coins to an address.
func (c *Client) EncodeOutputTransfer(amount Amount, address string, network Network) ([]byte, error) {
	amtPtr, err := c.newWASMAmount(amount)
	if err != nil {
		return nil, err
	}
	addrPtr, addrLen, err := c.writeString(address)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_output_transfer",
		uint64(amtPtr), uint64(addrPtr), uint64(addrLen), uint64(network))
}

// EncodeOutputTokenTransfer creates a Transfer output sending tokens to an address.
func (c *Client) EncodeOutputTokenTransfer(amount Amount, address, tokenID string, network Network) ([]byte, error) {
	amtPtr, err := c.newWASMAmount(amount)
	if err != nil {
		return nil, err
	}
	addrPtr, addrLen, err := c.writeString(address)
	if err != nil {
		return nil, err
	}
	tidPtr, tidLen, err := c.writeString(tokenID)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_output_token_transfer",
		uint64(amtPtr), uint64(addrPtr), uint64(addrLen),
		uint64(tidPtr), uint64(tidLen), uint64(network))
}

// EncodeOutputLockThenTransfer creates a LockThenTransfer output for coins.
// lock is the encoded timelock (see EncodeLockFor* functions).
func (c *Client) EncodeOutputLockThenTransfer(amount Amount, address string, lock []byte, network Network) ([]byte, error) {
	amtPtr, err := c.newWASMAmount(amount)
	if err != nil {
		return nil, err
	}
	addrPtr, addrLen, err := c.writeString(address)
	if err != nil {
		return nil, err
	}
	lockPtr, lockLen, err := c.writeBytes(lock)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_output_lock_then_transfer",
		uint64(amtPtr), uint64(addrPtr), uint64(addrLen),
		uint64(lockPtr), uint64(lockLen), uint64(network))
}

// EncodeOutputTokenLockThenTransfer creates a LockThenTransfer output for tokens.
func (c *Client) EncodeOutputTokenLockThenTransfer(amount Amount, address, tokenID string, lock []byte, network Network) ([]byte, error) {
	amtPtr, err := c.newWASMAmount(amount)
	if err != nil {
		return nil, err
	}
	addrPtr, addrLen, err := c.writeString(address)
	if err != nil {
		return nil, err
	}
	tidPtr, tidLen, err := c.writeString(tokenID)
	if err != nil {
		return nil, err
	}
	lockPtr, lockLen, err := c.writeBytes(lock)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_output_token_lock_then_transfer",
		uint64(amtPtr), uint64(addrPtr), uint64(addrLen),
		uint64(tidPtr), uint64(tidLen),
		uint64(lockPtr), uint64(lockLen), uint64(network))
}

// EncodeOutputCoinBurn creates a Burn output for coins.
func (c *Client) EncodeOutputCoinBurn(amount Amount) ([]byte, error) {
	amtPtr, err := c.newWASMAmount(amount)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_output_coin_burn", uint64(amtPtr))
}

// EncodeOutputTokenBurn creates a Burn output for tokens.
func (c *Client) EncodeOutputTokenBurn(amount Amount, tokenID string, network Network) ([]byte, error) {
	amtPtr, err := c.newWASMAmount(amount)
	if err != nil {
		return nil, err
	}
	tidPtr, tidLen, err := c.writeString(tokenID)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_output_token_burn",
		uint64(amtPtr), uint64(tidPtr), uint64(tidLen), uint64(network))
}

// EncodeOutputCreateDelegation creates an output that creates a staking delegation.
func (c *Client) EncodeOutputCreateDelegation(poolID, ownerAddress string, network Network) ([]byte, error) {
	pidPtr, pidLen, err := c.writeString(poolID)
	if err != nil {
		return nil, err
	}
	addrPtr, addrLen, err := c.writeString(ownerAddress)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_output_create_delegation",
		uint64(pidPtr), uint64(pidLen), uint64(addrPtr), uint64(addrLen), uint64(network))
}

// EncodeOutputDelegateStaking creates an output that delegates coins to a staking pool.
func (c *Client) EncodeOutputDelegateStaking(amount Amount, delegationID string, network Network) ([]byte, error) {
	amtPtr, err := c.newWASMAmount(amount)
	if err != nil {
		return nil, err
	}
	didPtr, didLen, err := c.writeString(delegationID)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_output_delegate_staking",
		uint64(amtPtr), uint64(didPtr), uint64(didLen), uint64(network))
}

// EncodeOutputCreateStakePool creates an output that creates a staking pool.
// poolData is the encoded stake pool data (see EncodeStakePoolData).
func (c *Client) EncodeOutputCreateStakePool(poolID string, poolData []byte, network Network) ([]byte, error) {
	pidPtr, pidLen, err := c.writeString(poolID)
	if err != nil {
		return nil, err
	}
	pdPtr, pdLen, err := c.writeBytes(poolData)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_output_create_stake_pool",
		uint64(pidPtr), uint64(pidLen), uint64(pdPtr), uint64(pdLen), uint64(network))
}

// EncodeOutputProduceBlockFromStake creates a ProduceBlockFromStake output.
// This UTXO is consumed when decommissioning a pool (if the pool has staked at least once).
func (c *Client) EncodeOutputProduceBlockFromStake(poolID, staker string, network Network) ([]byte, error) {
	pidPtr, pidLen, err := c.writeString(poolID)
	if err != nil {
		return nil, err
	}
	stkPtr, stkLen, err := c.writeString(staker)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_output_produce_block_from_stake",
		uint64(pidPtr), uint64(pidLen), uint64(stkPtr), uint64(stkLen), uint64(network))
}

// EncodeOutputDataDeposit creates a DataDeposit output for arbitrary on-chain data.
func (c *Client) EncodeOutputDataDeposit(data []byte) ([]byte, error) {
	ptr, length, err := c.writeBytes(data)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_output_data_deposit", uint64(ptr), uint64(length))
}

// EncodeOutputHTLC creates a hash time-lock contract (HTLC) output for coins or tokens.
// tokenID may be nil for coin HTLCs. refundTimelock is an encoded timelock.
func (c *Client) EncodeOutputHTLC(amount Amount, tokenID *string, secretHash, spendAddress, refundAddress string, refundTimelock []byte, network Network) ([]byte, error) {
	amtPtr, err := c.newWASMAmount(amount)
	if err != nil {
		return nil, err
	}
	tidPtr, tidLen, err := c.writeOptionalString(tokenID)
	if err != nil {
		return nil, err
	}
	shPtr, shLen, err := c.writeString(secretHash)
	if err != nil {
		return nil, err
	}
	saPtr, saLen, err := c.writeString(spendAddress)
	if err != nil {
		return nil, err
	}
	raPtr, raLen, err := c.writeString(refundAddress)
	if err != nil {
		return nil, err
	}
	tlPtr, tlLen, err := c.writeBytes(refundTimelock)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_output_htlc",
		uint64(amtPtr), uint64(tidPtr), uint64(tidLen),
		uint64(shPtr), uint64(shLen),
		uint64(saPtr), uint64(saLen),
		uint64(raPtr), uint64(raLen),
		uint64(tlPtr), uint64(tlLen),
		uint64(network))
}

// EncodeOutputIssueFungibleToken creates an output that issues a new fungible token.
// supplyAmount may be nil unless totalSupply is TotalSupplyFixed.
func (c *Client) EncodeOutputIssueFungibleToken(
	authority, tokenTicker, metadataURI string,
	numberOfDecimals uint32,
	totalSupply TotalSupply,
	supplyAmount *Amount,
	isTokenFreezable FreezableToken,
	currentBlockHeight uint64,
	network Network,
) ([]byte, error) {
	authPtr, authLen, err := c.writeString(authority)
	if err != nil {
		return nil, err
	}
	tkrPtr, tkrLen, err := c.writeString(tokenTicker)
	if err != nil {
		return nil, err
	}
	uriPtr, uriLen, err := c.writeString(metadataURI)
	if err != nil {
		return nil, err
	}

	var saPtr uint32
	if supplyAmount != nil {
		saPtr, err = c.newWASMAmount(*supplyAmount)
		if err != nil {
			return nil, err
		}
	}

	return c.callReturnBytes("encode_output_issue_fungible_token",
		uint64(authPtr), uint64(authLen),
		uint64(tkrPtr), uint64(tkrLen),
		uint64(uriPtr), uint64(uriLen),
		uint64(numberOfDecimals), uint64(totalSupply),
		uint64(saPtr), uint64(isTokenFreezable),
		currentBlockHeight, uint64(network))
}

// EncodeOutputIssueNFT creates an output that issues a new NFT.
// creator, mediaURI, iconURI, and additionalMetadataURI may be nil.
func (c *Client) EncodeOutputIssueNFT(
	tokenID, authority, name, ticker, description string,
	mediaHash []byte,
	creator []byte,
	mediaURI, iconURI, additionalMetadataURI *string,
	currentBlockHeight uint64,
	network Network,
) ([]byte, error) {
	tidPtr, tidLen, err := c.writeString(tokenID)
	if err != nil {
		return nil, err
	}
	authPtr, authLen, err := c.writeString(authority)
	if err != nil {
		return nil, err
	}
	namePtr, nameLen, err := c.writeString(name)
	if err != nil {
		return nil, err
	}
	tkrPtr, tkrLen, err := c.writeString(ticker)
	if err != nil {
		return nil, err
	}
	descPtr, descLen, err := c.writeString(description)
	if err != nil {
		return nil, err
	}
	mhPtr, mhLen, err := c.writeBytes(mediaHash)
	if err != nil {
		return nil, err
	}
	crPtr, crLen, err := c.writeOptionalBytes(creator)
	if err != nil {
		return nil, err
	}
	muPtr, muLen, err := c.writeOptionalString(mediaURI)
	if err != nil {
		return nil, err
	}
	iuPtr, iuLen, err := c.writeOptionalString(iconURI)
	if err != nil {
		return nil, err
	}
	amPtr, amLen, err := c.writeOptionalString(additionalMetadataURI)
	if err != nil {
		return nil, err
	}

	return c.callReturnBytes("encode_output_issue_nft",
		uint64(tidPtr), uint64(tidLen),
		uint64(authPtr), uint64(authLen),
		uint64(namePtr), uint64(nameLen),
		uint64(tkrPtr), uint64(tkrLen),
		uint64(descPtr), uint64(descLen),
		uint64(mhPtr), uint64(mhLen),
		uint64(crPtr), uint64(crLen),
		uint64(muPtr), uint64(muLen),
		uint64(iuPtr), uint64(iuLen),
		uint64(amPtr), uint64(amLen),
		currentBlockHeight, uint64(network))
}

// EncodeCreateOrderOutput creates an output that creates an order for token exchange.
// askTokenID and giveTokenID may be nil for coin amounts.
func (c *Client) EncodeCreateOrderOutput(
	askAmount Amount, askTokenID *string,
	giveAmount Amount, giveTokenID *string,
	concludeAddress string,
	network Network,
) ([]byte, error) {
	askAmtPtr, err := c.newWASMAmount(askAmount)
	if err != nil {
		return nil, err
	}
	askTidPtr, askTidLen, err := c.writeOptionalString(askTokenID)
	if err != nil {
		return nil, err
	}
	giveAmtPtr, err := c.newWASMAmount(giveAmount)
	if err != nil {
		return nil, err
	}
	giveTidPtr, giveTidLen, err := c.writeOptionalString(giveTokenID)
	if err != nil {
		return nil, err
	}
	caPtr, caLen, err := c.writeString(concludeAddress)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_create_order_output",
		uint64(askAmtPtr), uint64(askTidPtr), uint64(askTidLen),
		uint64(giveAmtPtr), uint64(giveTidPtr), uint64(giveTidLen),
		uint64(caPtr), uint64(caLen), uint64(network))
}
