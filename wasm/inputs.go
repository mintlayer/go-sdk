package mintlayer

// EncodeInputForUtxo encodes a UTXO input from an outpoint source ID and output index.
func (c *Client) EncodeInputForUtxo(outpointSourceID []byte, outputIndex uint32) ([]byte, error) {
	ptr, length, err := c.writeBytes(outpointSourceID)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_input_for_utxo",
		uint64(ptr), uint64(length), uint64(outputIndex))
}

// EncodeInputForWithdrawFromDelegation creates an input that withdraws from a delegation.
func (c *Client) EncodeInputForWithdrawFromDelegation(delegationID string, amount Amount, nonce uint64, network Network) ([]byte, error) {
	idPtr, idLen, err := c.writeString(delegationID)
	if err != nil {
		return nil, err
	}
	amtPtr, err := c.newWASMAmount(amount)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_input_for_withdraw_from_delegation",
		uint64(idPtr), uint64(idLen), uint64(amtPtr), nonce, uint64(network))
}

// EncodeInputForMintTokens creates an input to mint tokens.
func (c *Client) EncodeInputForMintTokens(tokenID string, amount Amount, nonce uint64, network Network) ([]byte, error) {
	idPtr, idLen, err := c.writeString(tokenID)
	if err != nil {
		return nil, err
	}
	amtPtr, err := c.newWASMAmount(amount)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_input_for_mint_tokens",
		uint64(idPtr), uint64(idLen), uint64(amtPtr), nonce, uint64(network))
}

// EncodeInputForUnmintTokens creates an input to unmint tokens.
func (c *Client) EncodeInputForUnmintTokens(tokenID string, nonce uint64, network Network) ([]byte, error) {
	idPtr, idLen, err := c.writeString(tokenID)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_input_for_unmint_tokens",
		uint64(idPtr), uint64(idLen), nonce, uint64(network))
}

// EncodeInputForLockTokenSupply creates an input to lock the token supply.
func (c *Client) EncodeInputForLockTokenSupply(tokenID string, nonce uint64, network Network) ([]byte, error) {
	idPtr, idLen, err := c.writeString(tokenID)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_input_for_lock_token_supply",
		uint64(idPtr), uint64(idLen), nonce, uint64(network))
}

// EncodeInputForFreezeToken creates an input to freeze a token.
func (c *Client) EncodeInputForFreezeToken(tokenID string, isTokenUnfreezable TokenUnfreezable, nonce uint64, network Network) ([]byte, error) {
	idPtr, idLen, err := c.writeString(tokenID)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_input_for_freeze_token",
		uint64(idPtr), uint64(idLen), uint64(isTokenUnfreezable), nonce, uint64(network))
}

// EncodeInputForUnfreezeToken creates an input to unfreeze a token.
func (c *Client) EncodeInputForUnfreezeToken(tokenID string, nonce uint64, network Network) ([]byte, error) {
	idPtr, idLen, err := c.writeString(tokenID)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_input_for_unfreeze_token",
		uint64(idPtr), uint64(idLen), nonce, uint64(network))
}

// EncodeInputForChangeTokenAuthority creates an input to change the token authority.
func (c *Client) EncodeInputForChangeTokenAuthority(tokenID, newAuthority string, nonce uint64, network Network) ([]byte, error) {
	idPtr, idLen, err := c.writeString(tokenID)
	if err != nil {
		return nil, err
	}
	authPtr, authLen, err := c.writeString(newAuthority)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_input_for_change_token_authority",
		uint64(idPtr), uint64(idLen), uint64(authPtr), uint64(authLen), nonce, uint64(network))
}

// EncodeInputForChangeTokenMetadataURI creates an input to change the token metadata URI.
func (c *Client) EncodeInputForChangeTokenMetadataURI(tokenID, newMetadataURI string, nonce uint64, network Network) ([]byte, error) {
	idPtr, idLen, err := c.writeString(tokenID)
	if err != nil {
		return nil, err
	}
	uriPtr, uriLen, err := c.writeString(newMetadataURI)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_input_for_change_token_metadata_uri",
		uint64(idPtr), uint64(idLen), uint64(uriPtr), uint64(uriLen), nonce, uint64(network))
}

// EncodeInputForConcludeOrder creates an input that concludes an order.
func (c *Client) EncodeInputForConcludeOrder(orderID string, nonce, currentBlockHeight uint64, network Network) ([]byte, error) {
	idPtr, idLen, err := c.writeString(orderID)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_input_for_conclude_order",
		uint64(idPtr), uint64(idLen), nonce, currentBlockHeight, uint64(network))
}

// EncodeInputForFillOrder creates an input that fills an order.
// FillOrder inputs should not be signed (use EncodeWitnessNoSignature).
func (c *Client) EncodeInputForFillOrder(orderID string, fillAmount Amount, destination string, nonce, currentBlockHeight uint64, network Network) ([]byte, error) {
	idPtr, idLen, err := c.writeString(orderID)
	if err != nil {
		return nil, err
	}
	amtPtr, err := c.newWASMAmount(fillAmount)
	if err != nil {
		return nil, err
	}
	destPtr, destLen, err := c.writeString(destination)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_input_for_fill_order",
		uint64(idPtr), uint64(idLen), uint64(amtPtr),
		uint64(destPtr), uint64(destLen), nonce, currentBlockHeight, uint64(network))
}

// EncodeInputForFreezeOrder creates an input that freezes an order.
// Order freezing is available only after the orders V1 fork.
func (c *Client) EncodeInputForFreezeOrder(orderID string, currentBlockHeight uint64, network Network) ([]byte, error) {
	idPtr, idLen, err := c.writeString(orderID)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_input_for_freeze_order",
		uint64(idPtr), uint64(idLen), currentBlockHeight, uint64(network))
}
