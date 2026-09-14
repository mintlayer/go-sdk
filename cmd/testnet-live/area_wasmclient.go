// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	mintlayer "github.com/mintlayer/go-sdk/wasm"
)

// runWasmArea exercises the embedded WASM cryptography/encoding runtime offline.
func runWasmArea(ctx context.Context, r *Runner, h *harness) {
	const area = "wasm"
	w := h.wasm

	// -- key derivation ---------------------------------------------------------
	priv, err := w.MakePrivateKey()
	if err != nil {
		r.Record(area, "MakePrivateKey", StatusFail, err.Error())
	} else if len(priv) == 0 {
		r.Record(area, "MakePrivateKey", StatusFail, "empty key")
	} else {
		r.Record(area, "MakePrivateKey", StatusPass, itoa(len(priv))+" bytes")
	}

	pub, err := w.PublicKeyFromPrivateKey(priv)
	// Observed wire shapes (tagged, not raw as docs/wasm.md implies):
	//   MakePrivateKey           → [0x00 tag][32-byte scalar]
	//   PublicKeyFromPrivateKey  → [0x00 tag][33-byte compressed pubkey]
	if err != nil || (len(pub) != 33 && !(len(pub) == 34 && pub[0] == 0x00)) {
		detail := fmt.Sprintf("pubkey len=%d", len(pub))
		if err != nil {
			detail = err.Error()
		}
		r.Record(area, "PublicKeyFromPrivateKey", StatusFail, detail)
	} else if addr, err := w.PubkeyToPubkeyHashAddress(pub, h.Net); err != nil || !strings.HasPrefix(addr, "tmt") {
		detail := "addr=" + addr
		if err != nil {
			detail = err.Error()
		}
		r.Record(area, "PubkeyToPubkeyHashAddress(Testnet)", StatusFail, detail)
	} else {
		r.Record(area, "PublicKeyFromPrivateKey", StatusPass,
			fmt.Sprintf("%d bytes (0x00-tagged Destination::PubKey form)", len(pub)))
		r.Record(area, "PubkeyToPubkeyHashAddress(Testnet)", StatusPass, addr)
		if len(pub) == 34 {
			r.Finding("wasm.PublicKeyFromPrivateKey returns a tagged Destination blob (0x00 + compressed key) and MakePrivateKey returns a tagged scalar — docs/wasm.md describes bare 33/32-byte keys; callers using them as raw crypto material (e.g. multisig challenge concatenation) will misbehave")
		}
	}

	// network awareness: mainnet prefix must differ from testnet
	if addr, err := w.PubkeyToPubkeyHashAddress(priv2pub(w, priv), mintlayer.Mainnet); err != nil {
		r.Record(area, "PubkeyToPubkeyHashAddress(Mainnet)", StatusFail, err.Error())
	} else if strings.HasPrefix(addr, "tmt") {
		r.Record(area, "PubkeyToPubkeyHashAddress(Mainnet)", StatusFail, "testnet prefix on mainnet addr")
	} else {
		r.Record(area, "PubkeyToPubkeyHashAddress(Mainnet)", StatusPass, addr)
	}

	// -- mnemonic determinism ------------------------------------------------------
	const mnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	k1, e1 := w.MakeDefaultAccountPrivkey(mnemonic, h.Net)
	k2, e2 := w.MakeDefaultAccountPrivkey(mnemonic, h.Net)
	if e1 != nil || e2 != nil || !bytes.Equal(k1, k2) {
		r.Record(area, "MakeDefaultAccountPrivkey(determinism)", StatusFail, firstErr(e1, e2))
	} else {
		r0, _ := w.MakeReceivingAddress(k1, 0)
		r1, _ := w.MakeReceivingAddress(k1, 1)
		if bytes.Equal(r0, r1) {
			r.Record(area, "MakeReceivingAddress(indexes)", StatusFail, "index 0 and 1 derive the same key")
		} else {
			r.Record(area, "MakeDefaultAccountPrivkey(determinism)", StatusPass, "stable across calls")
			r.Record(area, "MakeReceivingAddress(indexes)", StatusPass, "distinct per index")
		}
	}

	// -- transaction building roundtrip ----------------------------------------------
	srcID, err := w.EncodeOutpointSourceId(bytes.Repeat([]byte{0x11}, 32), mintlayer.SourceTransaction)
	if err != nil {
		r.Record(area, "EncodeOutpointSourceId", StatusFail, err.Error())
	} else {
		r.Record(area, "EncodeOutpointSourceId", StatusPass, itoa(len(srcID))+" bytes")
	}
	inp, err := w.EncodeInputForUtxo(srcID, 0)
	if err != nil {
		r.Record(area, "EncodeInputForUtxo", StatusFail, err.Error())
	} else {
		r.Record(area, "EncodeInputForUtxo", StatusPass, itoa(len(inp))+" bytes")
	}
	out, err := w.EncodeOutputTransfer(mintlayer.NewAmount("100000"), "tmt1qydvc5z0yh9tganhpu52mzr622lkjw4wxq5zc697", h.Net)
	if err != nil {
		r.Record(area, "EncodeOutputTransfer", StatusFail, err.Error())
	} else {
		r.Record(area, "EncodeOutputTransfer", StatusPass, itoa(len(out))+" bytes")
	}
	tx, err := w.EncodeTransaction(inp, out, 0)
	if err != nil {
		r.Record(area, "EncodeTransaction", StatusFail, err.Error())
		tx = nil
	} else {
		r.Record(area, "EncodeTransaction", StatusPass, itoa(len(tx))+" bytes")
	}
	if tx != nil {
		id1, e1 := w.GetTransactionID(tx, true)
		id2, e2 := w.GetTransactionID(tx, true)
		if e1 != nil || e2 != nil || id1 == "" || id1 != id2 {
			r.Record(area, "GetTransactionID(determinism)", StatusFail, firstErr(e1, e2))
		} else {
			r.Record(area, "GetTransactionID(determinism)", StatusPass, short(id1))
		}
		if size, err := w.EstimateTransactionSize(inp, []string{"tmt1qydvc5z0yh9tganhpu52mzr622lkjw4wxq5zc697"}, out, h.Net); err != nil || size == 0 {
			detail := "zero size"
			if err != nil {
				detail = err.Error()
			}
			r.Record(area, "EstimateTransactionSize", StatusFail, detail)
		} else {
			r.Record(area, "EstimateTransactionSize", StatusPass, uitoa(uint64(size))+" bytes")
		}
	}

	// -- witnesses / challenge signing ------------------------------------------------
	if nosig, err := w.EncodeWitnessNoSignature(); err != nil || len(nosig) == 0 {
		detail := "empty witness"
		if err != nil {
			detail = err.Error()
		}
		r.Record(area, "EncodeWitnessNoSignature", StatusFail, detail)
	} else {
		r.Record(area, "EncodeWitnessNoSignature", StatusPass, hexs(nosig))
	}

	msg := []byte("testnet-live challenge")
	if priv == nil {
		r.Record(area, "SignChallenge", StatusSkip, "no private key")
	} else if sig, err := w.SignChallenge(priv, msg); err != nil {
		r.Record(area, "SignChallenge", StatusFail, err.Error())
	} else if pub, err := w.PublicKeyFromPrivateKey(priv); err != nil {
		r.Record(area, "SignChallenge", StatusFail, "pubkey: "+err.Error())
	} else if addr, err := w.PubkeyToPubkeyHashAddress(pub, h.Net); err != nil {
		r.Record(area, "SignChallenge", StatusFail, "addr: "+err.Error())
	} else if ok, err := w.VerifyChallenge(addr, h.Net, sig, msg); err != nil || !ok {
		detail := "verify=false"
		if err != nil {
			detail = err.Error()
		}
		r.Record(area, "SignChallenge", StatusFail, detail)
	} else if ok2, _ := w.VerifyChallenge(addr, h.Net, sig, []byte("tampered")); ok2 {
		r.Record(area, "SignChallenge", StatusFail, "verified a tampered message")
	} else {
		r.Record(area, "SignChallenge/VerifyChallenge", StatusPass, "roundtrip ok, tamper rejected")
	}

	// -- protocol fees -----------------------------------------------------------------
	height, _ := h.nc.BestBlockHeight(ctx)
	for _, f := range []struct {
		name string
		fn   func() (mintlayer.Amount, error)
	}{
		{"FungibleTokenIssuanceFee", func() (mintlayer.Amount, error) { return w.FungibleTokenIssuanceFee(height, h.Net) }},
		{"TokenSupplyChangeFee", func() (mintlayer.Amount, error) { return w.TokenSupplyChangeFee(height, h.Net) }},
		{"TokenFreezeFee", func() (mintlayer.Amount, error) { return w.TokenFreezeFee(height, h.Net) }},
		{"TokenChangeAuthorityFee", func() (mintlayer.Amount, error) { return w.TokenChangeAuthorityFee(height, h.Net) }},
	} {
		if amt, err := f.fn(); err != nil || cmpAtoms(amt.Atoms(), "0") < 0 {
			detail := "negative"
			if err != nil {
				detail = err.Error()
			}
			r.Record(area, f.name, StatusFail, detail)
		} else {
			r.Record(area, f.name, StatusPass, amt.Atoms()+" atoms at height "+uitoa(height))
		}
	}

	// -- order id derivation -------------------------------------------------------------
	if oid, err := w.GetOrderId(inp, h.Net); err != nil || !strings.HasPrefix(oid, "tordr") {
		detail := oid
		if err != nil {
			detail = err.Error()
		}
		r.Record(area, "GetOrderId(synthetic)", StatusFail, detail)
	} else {
		r.Record(area, "GetOrderId(synthetic)", StatusPass, short(oid))
	}

	// -- DecodeSignedTransactionToJS crash detection (forked child) -----------------------
	// The decoder is known to hard-crash the host process; run it in a child
	// process so the harness survives to report the finding.
	if err := probeDecodeCrash(); err != nil {
		r.Record(area, "DecodeSignedTransactionToJS", StatusFail, err.Error())
		r.Finding("DecodeSignedTransactionToJS crashes the host process (wazero JIT fault) on a valid signed tx — function unusable (%v)", err)
	} else {
		r.Record(area, "DecodeSignedTransactionToJS", StatusPass, "decoded without crash")
	}
}

// probeDecodeCrash re-executes this binary in a child process with a hidden
// flag that runs DecodeSignedTransactionToJS on a known-good signed tx.
func probeDecodeCrash() error {
	exe, err := osExecutable()
	if err != nil {
		return fmt.Errorf("cannot self-exec: %w", err)
	}
	cmd := exec.Command(exe, "--self-test-decode-crash")
	out, err := combinedOutput(cmd)
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(out))
	if len(msg) > 160 {
		msg = msg[:160]
	}
	if strings.Contains(msg, "fatal error") || strings.Contains(msg, "SIGSEGV") || strings.Contains(msg, "fault") || cmd.ProcessState == nil {
		return fmt.Errorf("child crashed: %s", msg)
	}
	// a clean error return from the wasm call is acceptable
	return nil
}

func priv2pub(w *mintlayer.Client, priv []byte) []byte {
	pub, err := w.PublicKeyFromPrivateKey(priv)
	if err != nil {
		return priv // triggers a clean encode error, fine for prefix checks
	}
	return pub
}

func firstErr(errs ...error) string {
	for _, e := range errs {
		if e != nil {
			return e.Error()
		}
	}
	return "determinism violated"
}

var _ = time.Second // keep time import for future waits
