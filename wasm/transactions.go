package mintlayer

import "encoding/json"

// EncodeTransaction encodes an unsigned transaction from its inputs and outputs.
// inputs and outputs must be concatenated encoded bytes from the respective Encode* functions.
func (c *Client) EncodeTransaction(inputs, outputs []byte, flags uint64) ([]byte, error) {
	inPtr, inLen, err := c.writeBytes(inputs)
	if err != nil {
		return nil, err
	}
	outPtr, outLen, err := c.writeBytes(outputs)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_transaction",
		uint64(inPtr), uint64(inLen), uint64(outPtr), uint64(outLen), flags)
}

// EncodeOutpointSourceId encodes a source ID (transaction hash or block reward ID) together
// with a SourceId discriminant into binary form for use in outpoints.
func (c *Client) EncodeOutpointSourceId(id []byte, sourceId SourceId) ([]byte, error) {
	ptr, length, err := c.writeBytes(id)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytesNoErr("encode_outpoint_source_id",
		uint64(ptr), uint64(length), uint64(sourceId))
}

// GetTransactionID returns the transaction ID (as a hex string) for the given encoded transaction.
// Set strictByteSize to true to require the bytes to represent exactly one Transaction object.
func (c *Client) GetTransactionID(transaction []byte, strictByteSize bool) (string, error) {
	ptr, length, err := c.writeBytes(transaction)
	if err != nil {
		return "", err
	}
	return c.callReturnString("get_transaction_id",
		uint64(ptr), uint64(length), encodeBool(strictByteSize))
}

// EstimateTransactionSize estimates the encoded size of a signed transaction in bytes.
// inputUtxosDests must contain one address string per input (the spending destination of each UTXO).
func (c *Client) EstimateTransactionSize(inputs []byte, inputUtxosDests []string, outputs []byte, network Network) (uint32, error) {
	inPtr, inLen, err := c.writeBytes(inputs)
	if err != nil {
		return 0, err
	}
	destsPtr, destsLen, err := c.writeStringArray(inputUtxosDests)
	if err != nil {
		return 0, err
	}
	defer c.freeStringArray(destsPtr, destsLen)
	outPtr, outLen, err := c.writeBytes(outputs)
	if err != nil {
		return 0, err
	}
	return c.callReturnU32("estimate_transaction_size",
		uint64(inPtr), uint64(inLen),
		uint64(destsPtr), uint64(destsLen),
		uint64(outPtr), uint64(outLen),
		uint64(network))
}

// EncodeSignedTransaction combines an unsigned transaction with its witness signatures into
// a fully signed transaction.
func (c *Client) EncodeSignedTransaction(transaction, signatures []byte) ([]byte, error) {
	txPtr, txLen, err := c.writeBytes(transaction)
	if err != nil {
		return nil, err
	}
	sigPtr, sigLen, err := c.writeBytes(signatures)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("encode_signed_transaction",
		uint64(txPtr), uint64(txLen), uint64(sigPtr), uint64(sigLen))
}

// EncodePartiallySignedTransaction creates a PartiallySignedTransaction object.
// additionalInfo provides pool/order data required for signing.
func (c *Client) EncodePartiallySignedTransaction(
	transaction, signatures, inputUtxos, inputDestinations, htlcSecrets []byte,
	additionalInfo TxAdditionalInfo,
	network Network,
) ([]byte, error) {
	txPtr, txLen, err := c.writeBytes(transaction)
	if err != nil {
		return nil, err
	}
	sigPtr, sigLen, err := c.writeBytes(signatures)
	if err != nil {
		return nil, err
	}
	utxosPtr, utxosLen, err := c.writeBytes(inputUtxos)
	if err != nil {
		return nil, err
	}
	destsPtr, destsLen, err := c.writeBytes(inputDestinations)
	if err != nil {
		return nil, err
	}
	htlcPtr, htlcLen, err := c.writeBytes(htlcSecrets)
	if err != nil {
		return nil, err
	}

	infoKey := c.allocExternRef(additionalInfo)
	defer c.freeExternRef(infoKey)

	return c.callReturnBytes("encode_partially_signed_transaction",
		uint64(txPtr), uint64(txLen),
		uint64(sigPtr), uint64(sigLen),
		uint64(utxosPtr), uint64(utxosLen),
		uint64(destsPtr), uint64(destsLen),
		uint64(htlcPtr), uint64(htlcLen),
		uint64(infoKey), uint64(network))
}

// DecodePartiallySignedTransactionToJS decodes a partially signed transaction into a JSON object.
func (c *Client) DecodePartiallySignedTransactionToJS(transaction []byte, network Network) (json.RawMessage, error) {
	ptr, length, err := c.writeBytes(transaction)
	if err != nil {
		return nil, err
	}
	return c.callReturnJSON("decode_partially_signed_transaction_to_js",
		uint64(ptr), uint64(length), uint64(network))
}

// DecodeSignedTransactionToJS decodes a signed transaction into a JSON object.
func (c *Client) DecodeSignedTransactionToJS(transaction []byte, network Network) (json.RawMessage, error) {
	ptr, length, err := c.writeBytes(transaction)
	if err != nil {
		return nil, err
	}
	return c.callReturnJSON("decode_signed_transaction_to_js",
		uint64(ptr), uint64(length), uint64(network))
}

// ExtractHTLCSecret extracts the pre-image secret from a signed HTLC-spend transaction.
func (c *Client) ExtractHTLCSecret(signedTx []byte, strictByteSize bool, htlcOutpointSourceId []byte, htlcOutputIndex uint32) ([]byte, error) {
	txPtr, txLen, err := c.writeBytes(signedTx)
	if err != nil {
		return nil, err
	}
	srcPtr, srcLen, err := c.writeBytes(htlcOutpointSourceId)
	if err != nil {
		return nil, err
	}
	return c.callReturnBytes("extract_htlc_secret",
		uint64(txPtr), uint64(txLen),
		encodeBool(strictByteSize),
		uint64(srcPtr), uint64(srcLen),
		uint64(htlcOutputIndex))
}

// InternalVerifyWitness verifies an input witness against the transaction.
// inputOwnerDest may be nil for inputs where the destination is not required.
func (c *Client) InternalVerifyWitness(
	sighashType SignatureHashType,
	inputOwnerDest *string,
	witness, transaction, inputUtxos []byte,
	inputIndex uint32,
	additionalInfo TxAdditionalInfo,
	blockHeight uint64,
	network Network,
) error {
	destPtr, destLen, err := c.writeOptionalString(inputOwnerDest)
	if err != nil {
		return err
	}
	witPtr, witLen, err := c.writeBytes(witness)
	if err != nil {
		return err
	}
	txPtr, txLen, err := c.writeBytes(transaction)
	if err != nil {
		return err
	}
	utxosPtr, utxosLen, err := c.writeBytes(inputUtxos)
	if err != nil {
		return err
	}

	infoKey := c.allocExternRef(additionalInfo)
	defer c.freeExternRef(infoKey)

	return c.callVoidFallible("internal_verify_witness",
		uint64(sighashType),
		uint64(destPtr), uint64(destLen),
		uint64(witPtr), uint64(witLen),
		uint64(txPtr), uint64(txLen),
		uint64(utxosPtr), uint64(utxosLen),
		uint64(inputIndex),
		uint64(infoKey),
		blockHeight, uint64(network))
}
