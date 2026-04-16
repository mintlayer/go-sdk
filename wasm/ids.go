package mintlayer

// GetPoolId returns the pool ID derived from a transaction's inputs.
func (c *Client) GetPoolId(inputs []byte, network Network) (string, error) {
	ptr, length, err := c.writeBytes(inputs)
	if err != nil {
		return "", err
	}
	return c.callReturnString("get_pool_id", uint64(ptr), uint64(length), uint64(network))
}

// GetTokenId returns the fungible or NFT token ID derived from a transaction's inputs.
// currentBlockHeight is used to determine the token ID scheme for the active network upgrade.
func (c *Client) GetTokenId(inputs []byte, currentBlockHeight uint64, network Network) (string, error) {
	ptr, length, err := c.writeBytes(inputs)
	if err != nil {
		return "", err
	}
	return c.callReturnString("get_token_id",
		uint64(ptr), uint64(length), currentBlockHeight, uint64(network))
}

// GetDelegationId returns the delegation ID derived from a transaction's inputs.
func (c *Client) GetDelegationId(inputs []byte, network Network) (string, error) {
	ptr, length, err := c.writeBytes(inputs)
	if err != nil {
		return "", err
	}
	return c.callReturnString("get_delegation_id", uint64(ptr), uint64(length), uint64(network))
}

// GetOrderId returns the DEX order ID derived from a transaction's inputs.
func (c *Client) GetOrderId(inputs []byte, network Network) (string, error) {
	ptr, length, err := c.writeBytes(inputs)
	if err != nil {
		return "", err
	}
	return c.callReturnString("get_order_id", uint64(ptr), uint64(length), uint64(network))
}
