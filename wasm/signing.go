// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mintlayer

// EncodeWitness signs a transaction input and returns the encoded InputWitness.
//
// privateKey is the raw encoded private key.
// inputOwnerDest is the bech32m address that owns the input being signed.
// transaction is the encoded unsigned transaction.
// inputUtxos is a concatenated set of optional UTXO outputs (one per input; prefix 0 for non-UTXO, 1+encoded-output for UTXO).
// inputIndex is the index of the input being signed.
// additionalInfo provides pool/order data needed for signing.
func (c *Client) EncodeWitness(
	sighashType SignatureHashType,
	privateKey []byte,
	inputOwnerDest string,
	transaction, inputUtxos []byte,
	inputIndex uint32,
	additionalInfo TxAdditionalInfo,
	blockHeight uint64,
	network Network,
) ([]byte, error) {
	pkPtr, pkLen, err := c.writeBytes(privateKey)
	if err != nil {
		return nil, err
	}
	destPtr, destLen, err := c.writeString(inputOwnerDest)
	if err != nil {
		return nil, err
	}
	txPtr, txLen, err := c.writeBytes(transaction)
	if err != nil {
		return nil, err
	}
	utxosPtr, utxosLen, err := c.writeBytes(inputUtxos)
	if err != nil {
		return nil, err
	}

	infoKey := c.allocExternRef(additionalInfo)
	defer c.freeExternRef(infoKey)

	return c.callReturnBytes("encode_witness",
		uint64(sighashType),
		uint64(pkPtr), uint64(pkLen),
		uint64(destPtr), uint64(destLen),
		uint64(txPtr), uint64(txLen),
		uint64(utxosPtr), uint64(utxosLen),
		uint64(inputIndex),
		uint64(infoKey),
		blockHeight, uint64(network))
}

// EncodeWitnessNoSignature returns an InputWitness that carries no signature.
// Used for FillOrder inputs.
func (c *Client) EncodeWitnessNoSignature() ([]byte, error) {
	return c.callReturnBytesNoErr("encode_witness_no_signature")
}

// EncodeWitnessHTLCSpend signs an HTLC input for spending (revealing the secret).
func (c *Client) EncodeWitnessHTLCSpend(
	sighashType SignatureHashType,
	privateKey []byte,
	inputOwnerDest string,
	transaction, inputUtxos []byte,
	inputIndex uint32,
	secret []byte,
	additionalInfo TxAdditionalInfo,
	blockHeight uint64,
	network Network,
) ([]byte, error) {
	pkPtr, pkLen, err := c.writeBytes(privateKey)
	if err != nil {
		return nil, err
	}
	destPtr, destLen, err := c.writeString(inputOwnerDest)
	if err != nil {
		return nil, err
	}
	txPtr, txLen, err := c.writeBytes(transaction)
	if err != nil {
		return nil, err
	}
	utxosPtr, utxosLen, err := c.writeBytes(inputUtxos)
	if err != nil {
		return nil, err
	}
	secPtr, secLen, err := c.writeBytes(secret)
	if err != nil {
		return nil, err
	}

	infoKey := c.allocExternRef(additionalInfo)
	defer c.freeExternRef(infoKey)

	return c.callReturnBytes("encode_witness_htlc_spend",
		uint64(sighashType),
		uint64(pkPtr), uint64(pkLen),
		uint64(destPtr), uint64(destLen),
		uint64(txPtr), uint64(txLen),
		uint64(utxosPtr), uint64(utxosLen),
		uint64(inputIndex),
		uint64(secPtr), uint64(secLen),
		uint64(infoKey),
		blockHeight, uint64(network))
}

// EncodeWitnessHTLCRefundSingleSig signs an HTLC input for refunding via a single-sig address.
func (c *Client) EncodeWitnessHTLCRefundSingleSig(
	sighashType SignatureHashType,
	privateKey []byte,
	inputOwnerDest string,
	transaction, inputUtxos []byte,
	inputIndex uint32,
	additionalInfo TxAdditionalInfo,
	blockHeight uint64,
	network Network,
) ([]byte, error) {
	pkPtr, pkLen, err := c.writeBytes(privateKey)
	if err != nil {
		return nil, err
	}
	destPtr, destLen, err := c.writeString(inputOwnerDest)
	if err != nil {
		return nil, err
	}
	txPtr, txLen, err := c.writeBytes(transaction)
	if err != nil {
		return nil, err
	}
	utxosPtr, utxosLen, err := c.writeBytes(inputUtxos)
	if err != nil {
		return nil, err
	}

	infoKey := c.allocExternRef(additionalInfo)
	defer c.freeExternRef(infoKey)

	return c.callReturnBytes("encode_witness_htlc_refund_single_sig",
		uint64(sighashType),
		uint64(pkPtr), uint64(pkLen),
		uint64(destPtr), uint64(destLen),
		uint64(txPtr), uint64(txLen),
		uint64(utxosPtr), uint64(utxosLen),
		uint64(inputIndex),
		uint64(infoKey),
		blockHeight, uint64(network))
}

// EncodeWitnessHTLCRefundMultisig adds a partial signature to an HTLC refund witness for a
// multisig refund address.
// keyIndex is the index of privateKey within the multisig challenge.
// inputWitness may be empty (first signer) or a previous partial result.
func (c *Client) EncodeWitnessHTLCRefundMultisig(
	sighashType SignatureHashType,
	privateKey []byte,
	keyIndex uint32,
	inputWitness, multisigChallenge, transaction, inputUtxos []byte,
	inputIndex uint32,
	additionalInfo TxAdditionalInfo,
	blockHeight uint64,
	network Network,
) ([]byte, error) {
	pkPtr, pkLen, err := c.writeBytes(privateKey)
	if err != nil {
		return nil, err
	}
	witPtr, witLen, err := c.writeBytes(inputWitness)
	if err != nil {
		return nil, err
	}
	chalPtr, chalLen, err := c.writeBytes(multisigChallenge)
	if err != nil {
		return nil, err
	}
	txPtr, txLen, err := c.writeBytes(transaction)
	if err != nil {
		return nil, err
	}
	utxosPtr, utxosLen, err := c.writeBytes(inputUtxos)
	if err != nil {
		return nil, err
	}

	infoKey := c.allocExternRef(additionalInfo)
	defer c.freeExternRef(infoKey)

	return c.callReturnBytes("encode_witness_htlc_refund_multisig",
		uint64(sighashType),
		uint64(pkPtr), uint64(pkLen),
		uint64(keyIndex),
		uint64(witPtr), uint64(witLen),
		uint64(chalPtr), uint64(chalLen),
		uint64(txPtr), uint64(txLen),
		uint64(utxosPtr), uint64(utxosLen),
		uint64(inputIndex),
		uint64(infoKey),
		blockHeight, uint64(network))
}

// SignChallenge signs an arbitrary message with the given private key for challenge verification.
// Use VerifyChallenge to verify the result.
func (c *Client) SignChallenge(privateKey, message []byte) ([]byte, error) {
	pkPtr, pkLen, err := c.writeBytes(privateKey)
	if err != nil {
		return nil, err
	}
	msgPtr, msgLen, err := c.writeBytes(message)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("sign_challenge",
		uint64(pkPtr), uint64(pkLen), uint64(msgPtr), uint64(msgLen))
}

// VerifyChallenge verifies a challenge signature produced by SignChallenge.
// address must be a pubkeyhash bech32m address.
func (c *Client) VerifyChallenge(address string, network Network, signedChallenge, message []byte) (bool, error) {
	addrPtr, addrLen, err := c.writeString(address)
	if err != nil {
		return false, err
	}
	sigPtr, sigLen, err := c.writeBytes(signedChallenge)
	if err != nil {
		return false, err
	}
	msgPtr, msgLen, err := c.writeBytes(message)
	if err != nil {
		return false, err
	}
	return c.callReturnBool("verify_challenge",
		uint64(addrPtr), uint64(addrLen),
		uint64(network),
		uint64(sigPtr), uint64(sigLen),
		uint64(msgPtr), uint64(msgLen))
}

// SignMessageForSpending signs a message for use as a transaction input witness.
// Use VerifySignatureForSpending to verify the result.
func (c *Client) SignMessageForSpending(privateKey, message []byte) ([]byte, error) {
	pkPtr, pkLen, err := c.writeBytes(privateKey)
	if err != nil {
		return nil, err
	}
	msgPtr, msgLen, err := c.writeBytes(message)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("sign_message_for_spending",
		uint64(pkPtr), uint64(pkLen), uint64(msgPtr), uint64(msgLen))
}

// VerifySignatureForSpending verifies a spending signature produced by SignMessageForSpending.
func (c *Client) VerifySignatureForSpending(publicKey, signature, message []byte) (bool, error) {
	pkPtr, pkLen, err := c.writeBytes(publicKey)
	if err != nil {
		return false, err
	}
	sigPtr, sigLen, err := c.writeBytes(signature)
	if err != nil {
		return false, err
	}
	msgPtr, msgLen, err := c.writeBytes(message)
	if err != nil {
		return false, err
	}
	return c.callReturnBool("verify_signature_for_spending",
		uint64(pkPtr), uint64(pkLen),
		uint64(sigPtr), uint64(sigLen),
		uint64(msgPtr), uint64(msgLen))
}
