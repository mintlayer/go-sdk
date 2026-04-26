// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// Command send-coins demonstrates the full manual transaction flow using the
// Mintlayer Go SDK:
//
//  1. Derive an account key and receiving address from a BIP-39 mnemonic.
//  2. Fetch spendable UTXOs for that address from the indexer.
//  3. Build an unsigned transaction (encode inputs and outputs).
//  4. Sign each input with EncodeWitness.
//  5. Submit the signed transaction to the indexer.
//
// Usage:
//
//	go run ./examples/send-coins \
//	  -mnemonic "word1 word2 ... word12" \
//	  -to      mxtc1qrecipient... \
//	  -amount  100000000000 \
//	  -indexer http://127.0.0.1:3000
//
// NOTE: This example sends ALL spendable UTXOs to the recipient with no change
// output. It is intentionally minimal. Production code should select UTXOs,
// compute fees, add a change output, and handle errors more robustly.
package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/mintlayer/go-sdk/indexer"
	mintlayer "github.com/mintlayer/go-sdk/wasm"
)

func main() {
	mnemonic := flag.String("mnemonic", "", "BIP-39 mnemonic (12 or 24 words)")
	toAddr := flag.String("to", "", "recipient bech32m address")
	amountAtoms := flag.String("amount", "", "amount to send in atoms (e.g. 100000000000 = 1 ML)")
	indexerURL := flag.String("indexer", "http://127.0.0.1:3000", "indexer base URL")
	keyIndex := flag.Uint("key-index", 0, "receiving address key index")
	flag.Parse()

	if *mnemonic == "" || *toAddr == "" || *amountAtoms == "" {
		flag.Usage()
		log.Fatal("required: -mnemonic, -to, -amount")
	}

	ctx := context.Background()

	// ── 1. Initialise the WASM cryptography runtime ───────────────────────────
	wasm, err := mintlayer.New(ctx)
	must(err, "init wasm")
	defer wasm.Close()

	// ── 2. Derive the spending key and address ────────────────────────────────
	accountKey, err := wasm.MakeDefaultAccountPrivkey(*mnemonic, mintlayer.Mainnet)
	must(err, "derive account key")

	spendKey, err := wasm.MakeReceivingAddress(accountKey, uint32(*keyIndex))
	must(err, "derive receiving key")

	pubKey, err := wasm.PublicKeyFromPrivateKey(spendKey)
	must(err, "derive public key")

	fromAddr, err := wasm.PubkeyToPubkeyHashAddress(pubKey, mintlayer.Mainnet)
	must(err, "derive address")
	log.Printf("spending from: %s", fromAddr)

	// ── 3. Fetch spendable UTXOs ──────────────────────────────────────────────
	idxClient := indexer.New(*indexerURL)
	utxos, err := idxClient.GetSpendableUTXOs(ctx, fromAddr)
	must(err, "fetch UTXOs")

	if len(utxos) == 0 {
		log.Fatalf("no spendable UTXOs for %s", fromAddr)
	}
	log.Printf("found %d spendable UTXO(s)", len(utxos))

	// Verify we have enough balance.
	total := totalAtoms(utxos)
	sendAmt := mustParseBig(*amountAtoms)
	if total.Cmp(sendAmt) < 0 {
		log.Fatalf("insufficient balance: have %s atoms, need %s atoms", total, sendAmt)
	}

	// ── 4. Encode inputs and collect per-input UTXO bytes ────────────────────
	//
	// Each input is: EncodeOutpointSourceId → EncodeInputForUtxo.
	// The combined inputs slice is the concatenation of all encoded inputs.
	//
	// We also build the inputUtxos slice used during signing. It is a
	// concatenation of one entry per input, each prefixed by:
	//   0x00  — no UTXO output data (fallback)
	//   0x01  — UTXO output data follows (required for correct sighash)
	var (
		encodedInputs  []byte
		allUtxoBytes   []byte // one entry per input for the witness call
	)

	for _, u := range utxos {
		// Hex-decode the source transaction ID.
		txIDHex := u.Outpoint.SourceID
		txIDBytes, err := hex.DecodeString(txIDHex)
		must(err, "decode tx id hex")

		// EncodeOutpointSourceId packs the tx hash + discriminant.
		srcID, err := wasm.EncodeOutpointSourceId(txIDBytes, mintlayer.SourceTransaction)
		must(err, "encode outpoint source id")

		// EncodeInputForUtxo builds the TxInput binary.
		inp, err := wasm.EncodeInputForUtxo(srcID, u.Outpoint.Index)
		must(err, "encode input for utxo")
		encodedInputs = append(encodedInputs, inp...)

		// Re-encode the UTXO output binary so the sighash covers the correct
		// value. For Transfer outputs we parse the JSON amount and destination.
		utxoEntry := encodeUTXOEntry(wasm, u.Output, mintlayer.Mainnet)
		allUtxoBytes = append(allUtxoBytes, utxoEntry...)
	}

	// ── 5. Encode the transfer output ────────────────────────────────────────
	output, err := wasm.EncodeOutputTransfer(
		mintlayer.NewAmount(*amountAtoms),
		*toAddr,
		mintlayer.Mainnet,
	)
	must(err, "encode output transfer")

	// ── 6. Build the unsigned transaction ────────────────────────────────────
	tx, err := wasm.EncodeTransaction(encodedInputs, output, 0 /*flags*/)
	must(err, "encode transaction")

	txID, err := wasm.GetTransactionID(tx, true)
	must(err, "get transaction id")
	log.Printf("unsigned tx id: %s", txID)

	// ── 7. Sign each input and collect witnesses ──────────────────────────────
	var witnessBytes []byte
	for i := range utxos {
		w, err := wasm.EncodeWitness(
			mintlayer.SigHashAll,
			spendKey,
			fromAddr,
			tx,
			allUtxoBytes,
			uint32(i),
			mintlayer.TxAdditionalInfo{},
			0, // block height (0 = no lock-time constraint)
			mintlayer.Mainnet,
		)
		must(err, fmt.Sprintf("sign input %d", i))
		witnessBytes = append(witnessBytes, w...)
	}

	// ── 8. Assemble the signed transaction ───────────────────────────────────
	signedTx, err := wasm.EncodeSignedTransaction(tx, witnessBytes)
	must(err, "encode signed transaction")

	signedHex := hex.EncodeToString(signedTx)
	log.Printf("signed tx (%d bytes)", len(signedTx))

	// ── 9. Submit ─────────────────────────────────────────────────────────────
	submittedTxID, err := idxClient.SubmitTransaction(ctx, signedHex)
	must(err, "submit transaction")
	fmt.Printf("submitted: %s\n", submittedTxID)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func must(err error, context string) {
	if err != nil {
		log.Fatalf("%s: %v", context, err)
	}
}

// mustParseBig parses a decimal atom string into a *big.Int.
func mustParseBig(s string) *big.Int {
	n := new(big.Int)
	if _, ok := n.SetString(strings.TrimSpace(s), 10); !ok {
		log.Fatalf("invalid atom amount: %q", s)
	}
	return n
}

// totalAtoms sums the coin atoms across all UTXOs.
func totalAtoms(utxos []indexer.UTXO) *big.Int {
	total := new(big.Int)
	for _, u := range utxos {
		var raw struct {
			Type  string `json:"type"`
			Value struct {
				Type   string `json:"type"`
				Amount struct {
					Atoms string `json:"atoms"`
				} `json:"amount"`
			} `json:"value"`
		}
		if err := json.Unmarshal(u.Output, &raw); err != nil {
			continue
		}
		if raw.Type == "Transfer" && raw.Value.Type == "Coin" {
			n := mustParseBig(raw.Value.Amount.Atoms)
			total.Add(total, n)
		}
	}
	return total
}

// encodeUTXOEntry re-encodes a JSON UTXO output into the binary form that
// EncodeWitness expects in its inputUtxos slice.
//
// Format: 0x01 + <encoded output bytes> when the output can be re-encoded,
//         0x00 otherwise (signing will still succeed for many output types).
func encodeUTXOEntry(c *mintlayer.Client, utxoJSON json.RawMessage, network mintlayer.Network) []byte {
	var raw struct {
		Type  string `json:"type"`
		Value struct {
			Type   string `json:"type"`
			Amount struct {
				Atoms string `json:"atoms"`
			} `json:"amount"`
		} `json:"value"`
		Destination string `json:"destination"`
	}
	if err := json.Unmarshal(utxoJSON, &raw); err != nil {
		return []byte{0x00}
	}

	if raw.Type == "Transfer" && raw.Value.Type == "Coin" {
		encoded, err := c.EncodeOutputTransfer(
			mintlayer.NewAmount(raw.Value.Amount.Atoms),
			raw.Destination,
			network,
		)
		if err == nil {
			return append([]byte{0x01}, encoded...)
		}
	}

	// For non-Transfer outputs or encoding errors, omit the UTXO data.
	// The witness will still be computed using the destination passed directly
	// to EncodeWitness; some output types do not require the full UTXO body.
	return []byte{0x00}
}

// ─── EstimatedFee (informational) ─────────────────────────────────────────────
// In production, call wasm.EstimateTransactionSize and multiply by the fee
// rate obtained from node.GetFeeRate or indexer.GetFeeRate to compute the
// exact fee before building outputs. Deduct it from the change output.
var _ = bytes.Compare // suppress unused import
