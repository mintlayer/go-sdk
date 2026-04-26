// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mintlayer

// EncodeDestination encodes a bech32m address string into a binary destination.
func (c *Client) EncodeDestination(address string, network Network) ([]byte, error) {
	ptr, length, err := c.writeString(address)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_destination",
		uint64(ptr), uint64(length), uint64(network))
}

// PubkeyToPubkeyHashAddress derives a pay-to-public-key-hash bech32m address.
func (c *Client) PubkeyToPubkeyHashAddress(pubkey []byte, network Network) (string, error) {
	ptr, length, err := c.writeBytes(pubkey)
	if err != nil {
		return "", err
	}
	return c.callReturnString("pubkey_to_pubkeyhash_address",
		uint64(ptr), uint64(length), uint64(network))
}

// EncodeMultisigChallenge encodes a multisig challenge (script) into binary.
// pubkeys is the concatenated encoded public keys.
func (c *Client) EncodeMultisigChallenge(pubkeys []byte, minRequiredSignatures uint32, network Network) ([]byte, error) {
	ptr, length, err := c.writeBytes(pubkeys)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_multisig_challenge",
		uint64(ptr), uint64(length), uint64(minRequiredSignatures), uint64(network))
}

// MultisigChallengeToAddress converts a binary multisig challenge into its
// bech32m address.
func (c *Client) MultisigChallengeToAddress(challenge []byte, network Network) (string, error) {
	ptr, length, err := c.writeBytes(challenge)
	if err != nil {
		return "", err
	}
	return c.callReturnString("multisig_challenge_to_address",
		uint64(ptr), uint64(length), uint64(network))
}
