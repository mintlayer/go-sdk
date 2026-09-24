// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mintlayer

// MakeTransactionIntentMessageToSign returns the canonical message that must be signed
// to produce a valid transaction intent.
// transactionID should be a hex-encoded transaction ID returned by GetTransactionID.
func (c *Client) MakeTransactionIntentMessageToSign(intent, transactionID string) ([]byte, error) {
	intPtr, intLen, err := c.writeString(intent)
	if err != nil {
		return nil, err
	}
	txidPtr, txidLen, err := c.writeString(transactionID)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("make_transaction_intent_message_to_sign",
		uint64(intPtr), uint64(intLen), uint64(txidPtr), uint64(txidLen))
}

// EncodeSignedTransactionIntent combines a signed message with individual per-input
// signatures into an encoded SignedTransactionIntent.
// signedMessage must be produced by MakeTransactionIntentMessageToSign.
// signatures is an array of raw signature bytes, one per transaction input, each produced
// by SignChallenge.
func (c *Client) EncodeSignedTransactionIntent(signedMessage []byte, signatures [][]byte) ([]byte, error) {
	msgPtr, msgLen, err := c.writeBytes(signedMessage)
	if err != nil {
		return nil, err
	}
	// The signatures array buffer is owned by the guest once the export is
	// invoked (it converts the elements and frees the buffer during the call);
	// only the Go-side refs entries and the wrapper-owned element backing
	// buffers are released afterwards. Freeing the array buffer here would be
	// a double free corrupting the WASM heap.
	sigs, err := c.writeUint8ArrayArray(signatures)
	if err != nil {
		return nil, err
	}
	defer sigs.release(c)
	return c.callReturnBytes("encode_signed_transaction_intent",
		uint64(msgPtr), uint64(msgLen), uint64(sigs.ptr), uint64(sigs.count))
}

// VerifyTransactionIntent verifies a signed transaction intent.
// expectedSignedMessage must have been produced by MakeTransactionIntentMessageToSign.
// encodedSignedIntent must have been produced by EncodeSignedTransactionIntent.
// inputDestinations contains one bech32m address per transaction input.
func (c *Client) VerifyTransactionIntent(expectedSignedMessage, encodedSignedIntent []byte, inputDestinations []string, network Network) error {
	msgPtr, msgLen, err := c.writeBytes(expectedSignedMessage)
	if err != nil {
		return err
	}
	intentPtr, intentLen, err := c.writeBytes(encodedSignedIntent)
	if err != nil {
		return err
	}
	// The destinations array buffer is owned by the guest once the export is
	// invoked; only the Go-side refs entries are released afterwards.
	dests, err := c.writeStringArray(inputDestinations)
	if err != nil {
		return err
	}
	defer dests.release(c)
	return c.callVoidFallible("verify_transaction_intent",
		uint64(msgPtr), uint64(msgLen),
		uint64(intentPtr), uint64(intentLen),
		uint64(dests.ptr), uint64(dests.count),
		uint64(network))
}
