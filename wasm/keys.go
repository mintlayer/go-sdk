// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mintlayer

// MakePrivateKey generates a new random private key.
func (c *Client) MakePrivateKey() ([]byte, error) {
	return c.callReturnBytesNoErr("make_private_key")
}

// MakeDefaultAccountPrivkey derives the extended private key for the default
// account (account 0) from a BIP39 mnemonic and network.
// Derivation path: 44'/mintlayer_coin_type'/0'
func (c *Client) MakeDefaultAccountPrivkey(mnemonic string, network Network) ([]byte, error) {
	ptr, length, err := c.writeString(mnemonic)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("make_default_account_privkey",
		uint64(ptr), uint64(length), uint64(network))
}

// PublicKeyFromPrivateKey derives the compressed public key for a private key.
func (c *Client) PublicKeyFromPrivateKey(privkey []byte) ([]byte, error) {
	ptr, length, err := c.writeBytes(privkey)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("public_key_from_private_key", uint64(ptr), uint64(length))
}

// ExtendedPublicKeyFromExtendedPrivateKey derives the extended public key from
// an extended private key.
func (c *Client) ExtendedPublicKeyFromExtendedPrivateKey(privkey []byte) ([]byte, error) {
	ptr, length, err := c.writeBytes(privkey)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("extended_public_key_from_extended_private_key",
		uint64(ptr), uint64(length))
}

// MakeReceivingAddress derives a receiving (external) address key at the given index.
func (c *Client) MakeReceivingAddress(accountPrivkey []byte, keyIndex uint32) ([]byte, error) {
	ptr, length, err := c.writeBytes(accountPrivkey)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("make_receiving_address",
		uint64(ptr), uint64(length), uint64(keyIndex))
}

// MakeChangeAddress derives a change (internal) address key at the given index.
func (c *Client) MakeChangeAddress(accountPrivkey []byte, keyIndex uint32) ([]byte, error) {
	ptr, length, err := c.writeBytes(accountPrivkey)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("make_change_address",
		uint64(ptr), uint64(length), uint64(keyIndex))
}

// MakeReceivingAddressPublicKey derives the receiving address public key from an
// extended public key at the given index.
func (c *Client) MakeReceivingAddressPublicKey(accountPubkey []byte, keyIndex uint32) ([]byte, error) {
	ptr, length, err := c.writeBytes(accountPubkey)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("make_receiving_address_public_key",
		uint64(ptr), uint64(length), uint64(keyIndex))
}

// MakeChangeAddressPublicKey derives the change address public key from an
// extended public key at the given index.
func (c *Client) MakeChangeAddressPublicKey(accountPubkey []byte, keyIndex uint32) ([]byte, error) {
	ptr, length, err := c.writeBytes(accountPubkey)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("make_change_address_public_key",
		uint64(ptr), uint64(length), uint64(keyIndex))
}
