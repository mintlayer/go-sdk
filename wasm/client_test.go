package mintlayer_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"testing"

	mintlayer "github.com/mintlayer/go-sdk/wasm"
)

func newClient(t *testing.T) *mintlayer.Client {
	t.Helper()
	c, err := mintlayer.New(context.Background())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// ── key derivation ────────────────────────────────────────────────────────────

func TestMakePrivateKey(t *testing.T) {
	c := newClient(t)
	key, err := c.MakePrivateKey()
	if err != nil {
		t.Fatalf("MakePrivateKey: %v", err)
	}
	if len(key) == 0 {
		t.Fatal("expected non-empty private key")
	}
	// Keys should not be all-zero.
	allZero := true
	for _, b := range key {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Fatal("private key is all zeros")
	}
}

func TestMakePrivateKeyIsRandom(t *testing.T) {
	c := newClient(t)
	k1, err := c.MakePrivateKey()
	if err != nil {
		t.Fatalf("MakePrivateKey 1: %v", err)
	}
	k2, err := c.MakePrivateKey()
	if err != nil {
		t.Fatalf("MakePrivateKey 2: %v", err)
	}
	if bytes.Equal(k1, k2) {
		t.Fatal("two consecutive keys are identical — RNG broken")
	}
}

func TestMakeDefaultAccountPrivkey(t *testing.T) {
	c := newClient(t)
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	key, err := c.MakeDefaultAccountPrivkey(mnemonic, mintlayer.Mainnet)
	if err != nil {
		t.Fatalf("MakeDefaultAccountPrivkey: %v", err)
	}
	if len(key) == 0 {
		t.Fatal("expected non-empty account key")
	}
	// Deterministic: same mnemonic must produce the same key.
	key2, err := c.MakeDefaultAccountPrivkey(mnemonic, mintlayer.Mainnet)
	if err != nil {
		t.Fatalf("MakeDefaultAccountPrivkey (2nd): %v", err)
	}
	if !bytes.Equal(key, key2) {
		t.Fatal("key derivation is not deterministic")
	}
}

func TestPublicKeyFromPrivateKey(t *testing.T) {
	c := newClient(t)
	privKey, err := c.MakePrivateKey()
	if err != nil {
		t.Fatalf("MakePrivateKey: %v", err)
	}
	pubKey, err := c.PublicKeyFromPrivateKey(privKey)
	if err != nil {
		t.Fatalf("PublicKeyFromPrivateKey: %v", err)
	}
	if len(pubKey) == 0 {
		t.Fatal("expected non-empty public key")
	}
}

func TestKeyDerivationChain(t *testing.T) {
	c := newClient(t)
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	accountKey, err := c.MakeDefaultAccountPrivkey(mnemonic, mintlayer.Mainnet)
	if err != nil {
		t.Fatalf("MakeDefaultAccountPrivkey: %v", err)
	}
	recvKey, err := c.MakeReceivingAddress(accountKey, 0)
	if err != nil {
		t.Fatalf("MakeReceivingAddress: %v", err)
	}
	pubKey, err := c.PublicKeyFromPrivateKey(recvKey)
	if err != nil {
		t.Fatalf("PublicKeyFromPrivateKey: %v", err)
	}
	addr, err := c.PubkeyToPubkeyHashAddress(pubKey, mintlayer.Mainnet)
	if err != nil {
		t.Fatalf("PubkeyToPubkeyHashAddress: %v", err)
	}
	if addr == "" {
		t.Fatal("expected non-empty bech32m address")
	}
	t.Logf("address: %s", addr)

	// Extended public key path should yield the same public key.
	extPubKey, err := c.ExtendedPublicKeyFromExtendedPrivateKey(accountKey)
	if err != nil {
		t.Fatalf("ExtendedPublicKeyFromExtendedPrivateKey: %v", err)
	}
	pubKeyFromExt, err := c.MakeReceivingAddressPublicKey(extPubKey, 0)
	if err != nil {
		t.Fatalf("MakeReceivingAddressPublicKey: %v", err)
	}
	addrFromExt, err := c.PubkeyToPubkeyHashAddress(pubKeyFromExt, mintlayer.Mainnet)
	if err != nil {
		t.Fatalf("PubkeyToPubkeyHashAddress (ext): %v", err)
	}
	if addr != addrFromExt {
		t.Fatalf("address mismatch: private path=%q, public path=%q", addr, addrFromExt)
	}
}

// ── timelocks ─────────────────────────────────────────────────────────────────

func TestTimelocks(t *testing.T) {
	c := newClient(t)
	tests := []struct {
		name string
		fn   func() ([]byte, error)
	}{
		{"ForBlockCount", func() ([]byte, error) { return c.EncodeLockForBlockCount(100) }},
		{"ForSeconds", func() ([]byte, error) { return c.EncodeLockForSeconds(86400) }},
		{"UntilHeight", func() ([]byte, error) { return c.EncodeLockUntilHeight(500000) }},
		{"UntilTime", func() ([]byte, error) { return c.EncodeLockUntilTime(1700000000) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lock, err := tt.fn()
			if err != nil {
				t.Fatalf("lock: %v", err)
			}
			if len(lock) == 0 {
				t.Fatal("expected non-empty lock bytes")
			}
		})
	}
}

// ── fees ──────────────────────────────────────────────────────────────────────

func TestFees(t *testing.T) {
	c := newClient(t)
	height := uint64(500_000)
	network := mintlayer.Mainnet

	fees := []struct {
		name string
		fn   func() (mintlayer.Amount, error)
	}{
		{"FungibleTokenIssuance", func() (mintlayer.Amount, error) {
			return c.FungibleTokenIssuanceFee(height, network)
		}},
		{"NftIssuance", func() (mintlayer.Amount, error) {
			return c.NftIssuanceFee(height, network)
		}},
		{"DataDeposit", func() (mintlayer.Amount, error) {
			return c.DataDepositFee(height, network)
		}},
		{"TokenSupplyChange", func() (mintlayer.Amount, error) {
			return c.TokenSupplyChangeFee(height, network)
		}},
		{"TokenFreeze", func() (mintlayer.Amount, error) {
			return c.TokenFreezeFee(height, network)
		}},
		{"TokenChangeAuthority", func() (mintlayer.Amount, error) {
			return c.TokenChangeAuthorityFee(height, network)
		}},
	}
	for _, tt := range fees {
		t.Run(tt.name, func(t *testing.T) {
			fee, err := tt.fn()
			if err != nil {
				t.Fatalf("fee: %v", err)
			}
			if fee.Atoms() == "" || fee.Atoms() == "0" {
				t.Fatalf("expected non-zero fee, got %q", fee.Atoms())
			}
			t.Logf("%s fee: %s atoms", tt.name, fee.Atoms())
		})
	}
}

// ── staking ───────────────────────────────────────────────────────────────────

func TestStakingPoolSpendMaturityBlockCount(t *testing.T) {
	c := newClient(t)
	count, err := c.StakingPoolSpendMaturityBlockCount(500_000, mintlayer.Mainnet)
	if err != nil {
		t.Fatalf("StakingPoolSpendMaturityBlockCount: %v", err)
	}
	if count == 0 {
		t.Fatal("expected non-zero maturity block count")
	}
	t.Logf("maturity block count: %d", count)
}

// ── transaction roundtrip ─────────────────────────────────────────────────────

// fakeInputBytes builds a minimal fake input: encodes an all-zero tx hash as a UTXO source ID,
// then encodes input index 0.
func buildFakeInput(t *testing.T, c *mintlayer.Client) []byte {
	t.Helper()
	txIDBytes := make([]byte, 32)
	srcID, err := c.EncodeOutpointSourceId(txIDBytes, mintlayer.SourceTransaction)
	if err != nil {
		t.Fatalf("EncodeOutpointSourceId: %v", err)
	}
	input, err := c.EncodeInputForUtxo(srcID, 0)
	if err != nil {
		t.Fatalf("EncodeInputForUtxo: %v", err)
	}
	return input
}

func TestEncodeTransaction(t *testing.T) {
	c := newClient(t)

	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	accountKey, _ := c.MakeDefaultAccountPrivkey(mnemonic, mintlayer.Mainnet)
	recvKey, _ := c.MakeReceivingAddress(accountKey, 0)
	pubKey, _ := c.PublicKeyFromPrivateKey(recvKey)
	toAddr, _ := c.PubkeyToPubkeyHashAddress(pubKey, mintlayer.Mainnet)

	input := buildFakeInput(t, c)
	output, err := c.EncodeOutputTransfer(mintlayer.NewAmount("100000000000"), toAddr, mintlayer.Mainnet)
	if err != nil {
		t.Fatalf("EncodeOutputTransfer: %v", err)
	}

	tx, err := c.EncodeTransaction(input, output, 0)
	if err != nil {
		t.Fatalf("EncodeTransaction: %v", err)
	}
	if len(tx) == 0 {
		t.Fatal("expected non-empty transaction bytes")
	}

	txID, err := c.GetTransactionID(tx, true)
	if err != nil {
		t.Fatalf("GetTransactionID: %v", err)
	}
	if txID == "" {
		t.Fatal("expected non-empty transaction ID")
	}
	// Verify it looks like a hex hash.
	decoded, err := hex.DecodeString(txID)
	if err != nil {
		t.Fatalf("transaction ID is not hex: %v", err)
	}
	if len(decoded) == 0 {
		t.Fatal("decoded transaction ID is empty")
	}
	t.Logf("tx id: %s", txID)
}

func TestSignChallengeRoundtrip(t *testing.T) {
	c := newClient(t)

	privKey, err := c.MakePrivateKey()
	if err != nil {
		t.Fatalf("MakePrivateKey: %v", err)
	}
	pubKey, err := c.PublicKeyFromPrivateKey(privKey)
	if err != nil {
		t.Fatalf("PublicKeyFromPrivateKey: %v", err)
	}
	addr, err := c.PubkeyToPubkeyHashAddress(pubKey, mintlayer.Mainnet)
	if err != nil {
		t.Fatalf("PubkeyToPubkeyHashAddress: %v", err)
	}

	message := []byte("hello mintlayer")
	sig, err := c.SignChallenge(privKey, message)
	if err != nil {
		t.Fatalf("SignChallenge: %v", err)
	}
	ok, err := c.VerifyChallenge(addr, mintlayer.Mainnet, sig, message)
	if err != nil {
		t.Fatalf("VerifyChallenge: %v", err)
	}
	if !ok {
		t.Fatal("VerifyChallenge returned false")
	}
}

func TestSignMessageForSpendingRoundtrip(t *testing.T) {
	c := newClient(t)

	privKey, err := c.MakePrivateKey()
	if err != nil {
		t.Fatalf("MakePrivateKey: %v", err)
	}
	pubKey, err := c.PublicKeyFromPrivateKey(privKey)
	if err != nil {
		t.Fatalf("PublicKeyFromPrivateKey: %v", err)
	}

	message := []byte("spending message test")
	sig, err := c.SignMessageForSpending(privKey, message)
	if err != nil {
		t.Fatalf("SignMessageForSpending: %v", err)
	}
	ok, err := c.VerifySignatureForSpending(pubKey, sig, message)
	if err != nil {
		t.Fatalf("VerifySignatureForSpending: %v", err)
	}
	if !ok {
		t.Fatal("VerifySignatureForSpending returned false")
	}
}

func TestEncodeWitnessNoSignature(t *testing.T) {
	c := newClient(t)
	w, err := c.EncodeWitnessNoSignature()
	if err != nil {
		t.Fatalf("EncodeWitnessNoSignature: %v", err)
	}
	if len(w) == 0 {
		t.Fatal("expected non-empty witness bytes")
	}
}
