// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mintlayer_test

// Regression tests for the externref-array ownership bug (dlmalloc heap
// corruption).
//
// The WASM module's Vec<String>/Vec<Vec<u8>> ABI (wasm-bindgen reference
// types) TRANSFERS OWNERSHIP of the argument array to the guest: during the
// call the guest converts every element, recycles the externref table slots,
// and frees the array buffer itself. The wrapper must therefore never free
// the array buffer (or re-read indices from it) after the call — doing so is
// a double free that corrupts the dlmalloc heap, and the next malloc-heavy
// export traps out-of-bounds.
//
// These tests are canaries: they perform a vec-parameter call followed by a
// malloc-heavy export and assert the latter still works.

import (
	"testing"

	mintlayer "github.com/mintlayer/go-sdk/wasm"
)

// deriveTestAddress returns a valid bech32m address on the given network.
func deriveTestAddress(t *testing.T, c *mintlayer.Client, index uint32, network mintlayer.Network) string {
	t.Helper()
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	accountKey, err := c.MakeDefaultAccountPrivkey(mnemonic, network)
	if err != nil {
		t.Fatalf("MakeDefaultAccountPrivkey: %v", err)
	}
	recvKey, err := c.MakeReceivingAddress(accountKey, index)
	if err != nil {
		t.Fatalf("MakeReceivingAddress: %v", err)
	}
	pubKey, err := c.PublicKeyFromPrivateKey(recvKey)
	if err != nil {
		t.Fatalf("PublicKeyFromPrivateKey: %v", err)
	}
	addr, err := c.PubkeyToPubkeyHashAddress(pubKey, network)
	if err != nil {
		t.Fatalf("PubkeyToPubkeyHashAddress: %v", err)
	}
	return addr
}

// heapCanary runs malloc-heavy exports and fails if the heap has been
// corrupted by an earlier double free.
func heapCanary(t *testing.T, c *mintlayer.Client) {
	t.Helper()
	if _, err := c.EncodeWitnessNoSignature(); err != nil {
		t.Fatalf("heap corruption canary: EncodeWitnessNoSignature trapped: %v", err)
	}
	input := buildFakeInput(t, c)
	output, err := c.EncodeOutputTransfer(mintlayer.NewAmount("100000000000"), deriveTestAddress(t, c, 99, mintlayer.Mainnet), mintlayer.Mainnet)
	if err != nil {
		t.Fatalf("heap corruption canary: EncodeOutputTransfer: %v", err)
	}
	if _, err := c.EncodeTransaction(input, output, 0); err != nil {
		t.Fatalf("heap corruption canary: EncodeTransaction trapped: %v", err)
	}
}

// TestEstimateTransactionSizeThenEncodeCanary mirrors the fee fixed-point
// loop: several EstimateTransactionSize calls with owned string-vec args,
// followed by encode-heavy exports. Regression for the double free of the
// estimate_transaction_size destinations array.
func TestEstimateTransactionSizeThenEncodeCanary(t *testing.T) {
	c := newClient(t)

	input := buildFakeInput(t, c)
	output, err := c.EncodeOutputTransfer(mintlayer.NewAmount("100000000000"), deriveTestAddress(t, c, 1, mintlayer.Mainnet), mintlayer.Mainnet)
	if err != nil {
		t.Fatalf("EncodeOutputTransfer: %v", err)
	}
	dests := []string{
		deriveTestAddress(t, c, 2, mintlayer.Mainnet),
		deriveTestAddress(t, c, 3, mintlayer.Mainnet),
	}

	// 3× like the fee fixed-point loop before the first PST encode.
	for i := 0; i < 3; i++ {
		size, err := c.EstimateTransactionSize(input, dests, output, mintlayer.Mainnet)
		if err != nil {
			t.Fatalf("EstimateTransactionSize (round %d): %v", i, err)
		}
		if size == 0 {
			t.Fatalf("EstimateTransactionSize (round %d): returned 0", i)
		}
		// A canary after every estimate: pre-fix, the deferred free of the
		// destinations array corrupts the heap and this traps immediately.
		heapCanary(t, c)
	}
}

// TestEncodeSignedTransactionIntentCanary covers the Vec<Vec<u8>> path
// (writeUint8ArrayArray): the guest frees the signatures array buffer during
// encode_signed_transaction_intent, so the wrapper must not free it again.
func TestEncodeSignedTransactionIntentCanary(t *testing.T) {
	c := newClient(t)

	message, err := c.MakeTransactionIntentMessageToSign("intent-1", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	if err != nil {
		t.Fatalf("MakeTransactionIntentMessageToSign: %v", err)
	}
	privKey, err := c.MakePrivateKey()
	if err != nil {
		t.Fatalf("MakePrivateKey: %v", err)
	}
	sig, err := c.SignChallenge(privKey, message)
	if err != nil {
		t.Fatalf("SignChallenge: %v", err)
	}
	sigs := [][]byte{sig, sig, sig}

	if _, err := c.EncodeSignedTransactionIntent(message, sigs); err != nil {
		t.Logf("EncodeSignedTransactionIntent: %v (domain error is fine; the call still consumes the vec)", err)
	}
	heapCanary(t, c)
}

// TestVerifyTransactionIntentCanary covers the second Vec<String> path
// (VerifyTransactionIntent input destinations).
func TestVerifyTransactionIntentCanary(t *testing.T) {
	c := newClient(t)

	message, err := c.MakeTransactionIntentMessageToSign("intent-1", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	if err != nil {
		t.Fatalf("MakeTransactionIntentMessageToSign: %v", err)
	}
	dests := []string{
		deriveTestAddress(t, c, 4, mintlayer.Mainnet),
		deriveTestAddress(t, c, 5, mintlayer.Mainnet),
	}

	if err := c.VerifyTransactionIntent(message, []byte{0x01, 0x02, 0x03}, dests, mintlayer.Mainnet); err != nil {
		t.Logf("VerifyTransactionIntent: %v (domain error is fine; the call still consumes the vec)", err)
	}
	heapCanary(t, c)
}
