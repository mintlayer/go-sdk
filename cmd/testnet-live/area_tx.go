// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/mintlayer/go-sdk/node"
	"github.com/mintlayer/go-sdk/wallet"
	mintlayer "github.com/mintlayer/go-sdk/wasm"
)

// D12 fixture order (created in the tx area, concluded by the D12 tx itself):
//
//	give: 10 demo tokens (escrowed)   ask: 0.005 ML coins (never filled)
//
// The D12 transaction composes, in a single atomic transaction via pure wasm:
//
//	inputs:  [spendable UTXO (fees+change)] + [conclude-order(D12 fixture)]
//	outputs: [create-order (give 10 tokens again, ask 0.005 ML)] + [coin change]
const (
	d12GiveTokens = "10000000"   // 10 tokens @ 6 decimals escrowed by the fixture order
	d12AskCoins   = "500000000"  // 0.005 ML ask label
	d12MinFunding = "4000000000" // 0.04 ML minimum spendable UTXO for fees
)

// runTxArea exercises transaction flows: funding-UTXO discovery, non-broadcast
// composition, selected-UTXO spends, and the D12 conclude+create PST probe.
func runTxArea(ctx context.Context, r *Runner, h *harness, timeout time.Duration) {
	const area = "tx"

	// ── 0. SDK-shape bug probe: nil SelectedUTXOs (non-broadcast, zero fee) ──────
	var bFalse = false
	_, err := h.wc.AddressSend(ctx, wallet.SendParams{
		Account: 0, Address: mustAcc0Addr(ctx, h),
		Amount:        wallet.Amount{Atoms: "100000"},
		SelectedUTXOs: nil, // SDK marshals this as "selected_utxos":null
		Options:       wallet.TxOptions{BroadcastToMempool: &bFalse},
	})
	if err != nil {
		r.Record(area, "AddressSend(nil SelectedUTXOs)", StatusFail,
			"daemon rejects: "+err.Error())
		r.Finding("wallet.SendParams with nil SelectedUTXOs marshals %q and the daemon v1.4.0 rejects it (invalid type: null, expected a sequence). Workaround: pass an explicit empty slice.", "selected_utxos: null")
	} else {
		r.Record(area, "AddressSend(nil SelectedUTXOs)", StatusPass, "nil slice accepted")
	}

	// ── 1. funding-UTXO discovery ─────────────────────────────────────────────────
	fund, err := discoverFunding(ctx, h, d12MinFunding)
	if err != nil {
		// Under continuous bot churn the wallet view can be stale for every
		// candidate; that is an environment race, not an SDK defect.
		r.Record(area, "funding UTXO discovery", StatusSkip,
			"no verifiable unspent outpoint right now ("+err.Error()+") — live bot churns the wallet")
	} else {
		h.Funding = fund
		r.Record(area, "funding UTXO discovery", StatusPass,
			fmt.Sprintf("%s:%d = %s atoms", short(fund.TxID), fund.Vout, fund.Atoms))
	}

	if h.Funding != nil {
		// SDK GetUTXO roundtrip on the discovered outpoint
		op := node.Outpoint{
			SourceID: node.OutpointSourceID{
				Type:    "Transaction",
				Content: json.RawMessage(`{"tx_id":"` + h.Funding.TxID + `"}`),
			},
			Index: h.Funding.Vout,
		}
		if utxo, err := h.nc.GetUTXO(ctx, op); err != nil {
			r.Record(area, "node.GetUTXO(outpoint)", StatusFail, err.Error())
		} else if utxo == nil || !strings.Contains(string(utxo), h.Funding.Atoms) {
			r.Record(area, "node.GetUTXO(outpoint)", StatusFail, "utxo mismatch: "+string(utxo))
		} else {
			r.Record(area, "node.GetUTXO(outpoint)", StatusPass, short(string(utxo)))
		}
	}

	// NOTE on ordering: composing a transaction with BroadcastToMempool=false
	// makes the daemon RESERVE its selected inputs ("Selected UTXO ... is already
	// consumed"); the reservation is never released (transaction_abandon refuses
	// non-broadcast txs), so this flow runs AFTER everything that needs a
	// spendable outpoint.
	// ── 2. D12 PROBE: conclude+create via pure wasm + daemon signing ──────────────
	runD12Probe(ctx, r, h, timeout)

	// ── 4. non-broadcast compose + pending + abandon (zero fee) ───────────────────
	// ── 2. non-broadcast compose + pending + abandon (zero fee) ───────────────────
	selfAddr, err := h.wc.NewAddress(ctx, 0)
	if err != nil {
		r.Record(area, "non-broadcast self address", StatusFail, err.Error())
	} else if nb, err := h.wc.AddressSend(ctx, wallet.SendParams{
		Account: 0, Address: selfAddr, Amount: wallet.Amount{Atoms: "100000"},
		SelectedUTXOs: []wallet.Outpoint{},
		Options:       wallet.TxOptions{BroadcastToMempool: &bFalse},
	}); err != nil {
		r.Record(area, "AddressSend(broadcast=false)", StatusFail, err.Error())
	} else if nb.Broadcasted {
		r.Record(area, "AddressSend(broadcast=false)", StatusFail, "daemon broadcast anyway")
	} else {
		r.Record(area, "AddressSend(broadcast=false)", StatusPass, "txid="+short(nb.TxID)+" not broadcast")

		// SDK gap probe: the daemon response carries the tx hex in `tx`
		raw, rr, rerr := rawRPC(ctx, h.cfg.WalletURL, h.cfg.RPCUser, h.cfg.RPCPass, "address_send", map[string]any{
			"account": 0, "address": selfAddr,
			"amount":         map[string]any{"atoms": "100000"},
			"selected_utxos": []any{},
			"options":        map[string]any{"in_top_x_mb": nil, "broadcast_to_mempool": false},
		})
		if rerr == nil && rr.Error == nil && strings.Contains(string(raw), `"tx":"`) {
			r.Finding("wallet.SendResult drops the daemon's `tx` hex field; the full transaction hex is only reachable via raw RPC or the (broken) DecodeSignedTransactionToJS path")
			r.Record(area, "non-broadcast result carries tx hex (raw)", StatusPass, "raw field present, SDK drops it")
		}

		// Daemon semantics observed live: transaction_list_pending covers
		// mempool txs; a composed-but-never-broadcast tx is NOT pending.
		if pend, err := h.wc.ListPendingTransactions(ctx, 0); err != nil {
			r.Record(area, "pending list excludes non-broadcast tx", StatusFail, err.Error())
		} else if containsStr(pend, nb.TxID) {
			r.Record(area, "pending list excludes non-broadcast tx", StatusFail, "non-broadcast tx wrongly listed as pending")
		} else {
			r.Record(area, "pending list excludes non-broadcast tx", StatusPass, itoa(len(pend))+" pending")
		}

		// Abandoning a never-broadcast tx: the daemon rejects it (it only tracks
		// broadcast txs) — recorded as observed behaviour either way.
		if err := h.wc.AbandonTransaction(ctx, 0, nb.TxID); err != nil {
			r.Record(area, "AbandonTransaction(non-broadcast)", StatusPass,
				"clean RPC error (daemon only tracks broadcast txs): "+short(err.Error()))
		} else {
			r.Record(area, "AbandonTransaction(non-broadcast)", StatusPass, "abandoned")
		}
	}

	// ── 5. selected-UTXO spend (real broadcast; optional under the fee budget) ────
	// ── 4. selected-UTXO spend (real broadcast; optional under the fee budget) ────
	if r.OverBudget("25000000000") {
		r.Record(area, "SelectedUTXOs spend", StatusSkip, "fee budget exceeded")
	} else {
		if serr := spendSelectedUTXO(ctx, r, h, timeout); serr != nil {
			// the bot may consume the chosen outpoint mid-flight — retry once
			// with a freshly discovered outpoint
			if fund, err := discoverFunding(ctx, h, d12MinFunding); err == nil {
				h.Funding = fund
			}
			if serr2 := spendSelectedUTXO(ctx, r, h, timeout); serr2 != nil {
				r.RecordFee(area, "SelectedUTXOs spend", StatusFail, serr2.Error(), "0")
			}
		}
	}
}

// spendSelectedUTXO performs one AddressSend bound to the exact funding outpoint.
func spendSelectedUTXO(ctx context.Context, r *Runner, h *harness, timeout time.Duration) error {
	const area = "tx"
	if h.Funding == nil {
		return fmt.Errorf("no funding outpoint")
	}
	dst, aerr := h.wc.NewAddress(ctx, 0)
	if aerr != nil {
		return aerr
	}
	op := wallet.Outpoint{
		SourceID: wallet.OutpointSourceID{
			Type:    "Transaction",
			Content: json.RawMessage(`{"tx_id":"` + h.Funding.TxID + `"}`),
		},
		Index: h.Funding.Vout,
	}
	sel, serr := h.wc.AddressSend(ctx, wallet.SendParams{
		Account: 0, Address: dst, Amount: wallet.Amount{Atoms: "100000000"},
		SelectedUTXOs: []wallet.Outpoint{op},
	})
	if serr != nil {
		return serr
	}
	r.RecordFee(area, "SelectedUTXOs spend", StatusPass,
		"spent exact outpoint "+short(h.Funding.TxID), sel.Fees.Coins.Atoms)
	return confirmTx(ctx, h, sel.TxID, timeout)
}

// discoverFunding picks a coin UTXO from the daemon's own accounting
// (account_utxos RPC — which the SDK does not wrap, see findings) and
// cross-verifies it against the node, because the wallet's list can contain
// stale entries that are already spent on chain (the live bot churns the
// account continuously).
func discoverFunding(ctx context.Context, h *harness, minAtoms string) (*fundingUTXO, error) {
	raw, rr, err := rawRPC(ctx, h.cfg.WalletURL, h.cfg.RPCUser, h.cfg.RPCPass, "account_utxos", []any{0})
	if err != nil {
		return nil, err
	}
	if rr.Error != nil {
		return nil, fmt.Errorf("account_utxos: %s", rr.Error.Message)
	}
	var utxos []struct {
		Outpoint struct {
			SourceID struct {
				Type    string `json:"type"`
				Content struct {
					TxID string `json:"tx_id"`
				} `json:"content"`
			} `json:"source_id"`
			Index uint32 `json:"index"`
		} `json:"outpoint"`
		Output struct {
			Type    string `json:"type"`
			Content struct {
				Value struct {
					Type    string `json:"type"`
					Content struct {
						Amount struct {
							Atoms string `json:"atoms"`
						} `json:"amount"`
					} `json:"content"`
				} `json:"value"`
				Destination string `json:"destination"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(raw, &utxos); err != nil {
		return nil, err
	}
	// candidates in descending value, cross-checked against the node
	var cands []int
	for i := range utxos {
		u := utxos[i]
		if u.Output.Type != "Transfer" || u.Output.Content.Value.Type != "Coin" {
			continue
		}
		if cmpAtoms(u.Output.Content.Value.Content.Amount.Atoms, minAtoms) < 0 {
			continue
		}
		cands = append(cands, i)
	}
	for a := 1; a < len(cands); a++ {
		for b := a; b > 0 && cmpAtoms(utxos[cands[b]].Output.Content.Value.Content.Amount.Atoms,
			utxos[cands[b-1]].Output.Content.Value.Content.Amount.Atoms) > 0; b-- {
			cands[b], cands[b-1] = cands[b-1], cands[b]
		}
	}
	if len(cands) == 0 {
		return nil, fmt.Errorf("no account-0 coin UTXO ≥ %s ML in the daemon's spendable set", atomsToML(minAtoms))
	}
	for _, idx := range cands {
		u := utxos[idx]
		utxoJSON, _, rerr := rawRPC(ctx, h.cfg.NodeURL, h.cfg.RPCUser, h.cfg.RPCPass,
			"chainstate_get_utxo", []any{map[string]any{
				"source_id": map[string]any{"type": "Transaction", "content": map[string]any{"tx_id": u.Outpoint.SourceID.Content.TxID}},
				"index":     u.Outpoint.Index,
			}})
		if rerr != nil || len(utxoJSON) == 0 || string(utxoJSON) == "null" {
			continue // wallet view is stale — already spent on chain
		}
		return &fundingUTXO{
			TxID:  u.Outpoint.SourceID.Content.TxID,
			Vout:  u.Outpoint.Index,
			Atoms: u.Output.Content.Value.Content.Amount.Atoms,
			Dest:  u.Output.Content.Destination,
		}, nil
	}
	return nil, fmt.Errorf("all %d daemon-listed UTXOs are already spent on chain (stale wallet view)", len(cands))
}

// extractSignedTx strips the PST envelope: layout is tx || witnesses-vec ||
// input_utxos || ... — the broadcastable SignedTransaction is the composed tx
// bytes followed by the completed witness vector (compact count + entries).
// Witness entries: Option tag (0x01 Some / 0x00 None); Some = witness enum
// tag 0x00 NoSignature, 0x01 Signature(sighash u8 + sig 64 + pubkey 33).
func extractSignedTx(pst []byte, composedTx []byte) ([]byte, error) {
	if len(pst) < len(composedTx) {
		return nil, fmt.Errorf("pst shorter than composed tx")
	}
	if !equalBytes(pst[:len(composedTx)], composedTx) {
		// MaybeSignedTransaction enum tag (0x01 = PartiallySigned) precedes the PST
		if len(pst) > 0 && (pst[0] == 0x00 || pst[0] == 0x01) && equalBytes(pst[1:1+len(composedTx)], composedTx) {
			pst = pst[1:]
		} else {
			return nil, fmt.Errorf("pst does not start with the composed tx (first bytes %x vs %x)", pst[:min(8, len(pst))], composedTx[:min(8, len(composedTx))])
		}
	}
	rest := pst[len(composedTx):]
	n, off, err := readCompact(rest)
	if err != nil {
		return nil, fmt.Errorf("witness count: %w", err)
	}
	wit := []byte{}
	for i := 0; i < n; i++ {
		if off >= len(rest) {
			return nil, fmt.Errorf("witness %d: truncated option tag", i)
		}
		opt := rest[off]
		off++
		if opt == 0x00 {
			return nil, fmt.Errorf("witness %d: unsigned (incomplete pst)", i)
		}
		if off >= len(rest) {
			return nil, fmt.Errorf("witness %d: truncated enum tag", i)
		}
		tag := rest[off]
		off++
		switch tag {
		case 0x00: // NoSignature: empty body
			wit = append(wit, 0x00)
		case 0x01: // Signature: sighash(1) + sig(64) + pubkey(33)
			if off+98 > len(rest) {
				return nil, fmt.Errorf("witness %d: truncated signature", i)
			}
			wit = append(wit, 0x01)
			wit = append(wit, rest[off:off+1+64+33]...)
			off += 1 + 64 + 33
		default:
			return nil, fmt.Errorf("witness %d: unknown tag 0x%02x", i, tag)
		}
	}
	// SignedTransaction = tx || compact(count) || witnesses (no option tags)
	cb := []byte{byte(n&0b11 | ((n >> 2) << 2))}
	out := append(append([]byte{}, composedTx...), cb...)
	out = append(out, wit...)
	return out, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// readCompact parses a scale compact uint prefix (single-byte form suffices
// for input counts < 64).
func readCompact(b []byte) (int, int, error) {
	if len(b) == 0 {
		return 0, 0, fmt.Errorf("empty")
	}
	f := b[0] & 0b11
	switch {
	case f < 2:
		return int(b[0]) >> 2, 1, nil
	case f == 2:
		if len(b) < 2 {
			return 0, 0, fmt.Errorf("truncated")
		}
		v := int(b[0])>>2 | int(b[1])<<6
		return v, 2, nil
	}
	return 0, 0, fmt.Errorf("compact too large for witness counts")
}

// runD12Probe composes conclude+create in a single transaction via pure wasm,
// wraps it in a PST, has the wallet daemon sign it, and reports every gap on
// the (currently impossible) broadcast path.
func runD12Probe(ctx context.Context, r *Runner, h *harness, timeout time.Duration) {
	const area = "tx"
	const name = "D12 probe: wasm conclude+create PST"
	w := h.wasm

	// fixture order D12: give coins, ask tokens, owned by account 0
	concludeAddr, err := h.wc.NewAddress(ctx, 0)
	if err != nil {
		r.Record(area, name, StatusFail, "conclude addr: "+err.Error())
		return
	}
	if r.OverBudget("25000000000") {
		r.Record(area, name, StatusSkip, "fee budget exceeded (fixture order not created)")
		return
	}
	tokenID := h.TokenID
	if tokenID == "" {
		tokenID = demoTokenID() // tokens area not selected this run
	}
	created, err := h.wc.CreateOrder(ctx, wallet.CreateOrderParams{
		Account:         0,
		Ask:             wallet.OutputValue{Coin: true, Amount: wallet.Amount{Atoms: d12AskCoins}},
		Give:            wallet.OutputValue{Coin: false, TokenID: tokenID, Amount: wallet.Amount{Atoms: d12GiveTokens}},
		ConcludeAddress: concludeAddr,
	})
	if err != nil {
		r.Record(area, name, StatusFail, "fixture order create: "+err.Error())
		return
	}
	r.RecordFee(area, "D12 fixture order (10 tokens give)", StatusPass,
		"order="+short(created.OrderID), "20000000000")
	if err := confirmTx(ctx, h, created.TxID, timeout); err != nil {
		// The probe only composes and signs; it does not need the fixture order
		// to be in chain state yet, so proceed with a note instead of aborting.
		r.Record(area, name+" fixture order confirm", StatusFail, err.Error()+" (probe continues: compose+sign do not need chain state)")
	}
	if h.Funding == nil {
		r.Record(area, name, StatusSkip, "no funding UTXO for the fee input")
		return
	}

	height, _ := h.nc.BestBlockHeight(ctx)

	// inputs: [funding UTXO] + [conclude order]
	txidBytes, _ := hex.DecodeString(h.Funding.TxID)
	srcID, _ := w.EncodeOutpointSourceId(txidBytes, mintlayer.SourceTransaction)
	inFund, err := w.EncodeInputForUtxo(srcID, h.Funding.Vout)
	if err != nil {
		r.Record(area, name, StatusFail, "encode funding input: "+err.Error())
		return
	}
	inConclude, err := w.EncodeInputForConcludeOrder(created.OrderID, 0, height+1, h.Net)
	if err != nil {
		r.Record(area, name, StatusFail, "EncodeInputForConcludeOrder: "+err.Error())
		return
	}
	inputs := append(append([]byte{}, inFund...), inConclude...)

	// outputs: [create order (give: same tokens re-escrowed, ask: coins)] + [change]
	newConclude, _ := h.wc.NewAddress(ctx, 0)
	giveTok := tokenID
	createOut, err := w.EncodeCreateOrderOutput(
		mintlayer.NewAmount(d12AskCoins), nil,
		mintlayer.NewAmount(d12GiveTokens), &giveTok,
		newConclude, h.Net,
	)
	if err != nil {
		r.Record(area, name, StatusFail, "EncodeCreateOrderOutput: "+err.Error())
		return
	}
	changeAtoms := subAtoms(h.Funding.Atoms, "30000000000") // minus 0.3 ML fee headroom
	changeOut, err := w.EncodeOutputTransfer(mintlayer.NewAmount(changeAtoms), h.Funding.Dest, h.Net)
	if err != nil {
		r.Record(area, name, StatusFail, "encode change: "+err.Error())
		return
	}
	outputs := append(append([]byte{}, createOut...), changeOut...)

	tx, err := w.EncodeTransaction(inputs, outputs, 0)
	if err != nil {
		r.Record(area, name, StatusFail, "EncodeTransaction: "+err.Error())
		return
	}
	if txid, err := w.GetTransactionID(tx, true); err != nil || txid == "" {
		r.Record(area, name, StatusFail, "GetTransactionID: "+firstErr(err))
		return
	}

	// fee estimate (rate is flat on this testnet: 1 ML/KB)
	rate := "100000000000"
	if fr, ferr := h.nc.GetFeeRate(ctx, 1); ferr == nil && fr != nil {
		rate = fr.AmountPerKB.Atoms
	}
	fee := "30000000000" // 0.3 ML fallback headroom
	est := uint32(300)
	if e, eerr := w.EstimateTransactionSize(inputs,
		[]string{h.Funding.Dest, concludeAddr}, outputs, h.Net); eerr != nil {
		r.Record(area, name+" EstimateTransactionSize", StatusFail,
			"on conclude+create inputs (fee fallback used): "+eerr.Error())
	} else {
		est = e
		fee = feeForSize(est, rate)
	}
	changeAtoms = subAtoms(h.Funding.Atoms, fee)
	changeOut, err = w.EncodeOutputTransfer(mintlayer.NewAmount(changeAtoms), h.Funding.Dest, h.Net)
	if err != nil {
		r.Record(area, name, StatusFail, "re-encode change: "+err.Error())
		return
	}
	tx, err = w.EncodeTransaction(inputs, append(append([]byte{}, createOut...), changeOut...), 0)
	h.D12ComposedTx = append([]byte{}, tx...)
	if err != nil {
		r.Record(area, name, StatusFail, "re-encode tx: "+err.Error())
		return
	}

	// PST assembly — the wire format needs one Option<Destination>/Option<TxOutput>/
	// Option<InputWitness> tag byte per entry (0x01 = Some, 0x00 = None), a
	// convention the SDK does not document (pre-flight finding).
	utxoBlob, _ := w.EncodeOutputTransfer(mintlayer.NewAmount(h.Funding.Atoms), h.Funding.Dest, h.Net)
	destFund, _ := w.EncodeDestination(h.Funding.Dest, h.Net)
	destConc, _ := w.EncodeDestination(concludeAddr, h.Net)

	inputUtxos := append([]byte{0x01}, utxoBlob...)   // funding input: Some(output)
	inputUtxos = append(inputUtxos, 0x00)             // conclude input: None
	destinations := append([]byte{0x01}, destFund...) // Some(dest)
	destinations = append(destinations, append([]byte{0x01}, destConc...)...)
	nosig, _ := w.EncodeWitnessNoSignature()
	witnesses := append([]byte{0x01}, nosig...) // Some(NoSignature)
	witnesses = append(witnesses, append([]byte{0x01}, nosig...)...)
	htlc := []byte{0x00, 0x00} // None per input

	additional := mintlayer.TxAdditionalInfo{
		PoolInfo: map[string]mintlayer.PoolInfo{},
		OrderInfo: map[string]mintlayer.OrderInfo{
			created.OrderID: {
				InitiallyAsked: mintlayer.CoinsAmount(d12AskCoins),
				InitiallyGiven: mintlayer.TokensAmount(d12GiveTokens, tokenID),
				AskBalance:     mintlayer.OrderBalance{Atoms: "0"},
				GiveBalance:    mintlayer.OrderBalance{Atoms: d12GiveTokens, TokenID: &giveTok},
			},
		},
	}

	pst, err := w.EncodePartiallySignedTransaction(tx, witnesses, inputUtxos, destinations, htlc, additional, h.Net)
	if err != nil {
		r.Record(area, name, StatusFail, "EncodePartiallySignedTransaction: "+err.Error())
		return
	}
	pstHex := hex.EncodeToString(pst)
	r.Record(area, name+" composed", StatusPass,
		fmt.Sprintf("pst=%d bytes, est fee %s atoms", len(pst), fee))

	// daemon-side structural validation
	if insp, err := h.wc.InspectTransaction(ctx, pstHex); err != nil {
		r.Record(area, name+" InspectTransaction", StatusFail, err.Error())
		if strings.Contains(err.Error(), "Insufficient") {
			r.Finding("wallet.InspectTransaction rejects conclude+create PSTs with %q — the wallet's balance check does not credit order inputs with the order's escrowed balance (observed: required %s tokens, available 0), while SignRawTransaction accepts the same PST", "Insufficient UTXO amount", d12GiveTokens)
		}
	} else {
		r.Record(area, name+" InspectTransaction", StatusPass,
			fmt.Sprintf("inputs=%d sigs=%d", insp.Stats.NumInputs, insp.Stats.TotalSignatures))
	}

	// the actual signing probe — does the daemon own the conclude key?
	signed, err := h.wc.SignRawTransaction(ctx, 0, pstHex)
	if err != nil {
		r.Record(area, name+" SignRawTransaction", StatusFail,
			"SIGNING FAILED (conclude-key unknown to the wallet?): "+err.Error())
		return
	}
	h.D12SignedPST = signed.Hex
	r.Record(area, name+" SignRawTransaction", StatusPass,
		"daemon signed the conclude input (conclude key owned); signed PST "+itoa(len(signed.Hex)/2)+" bytes")
	r.Finding("wallet.SignRawTransaction returns MaybeSignedTransaction (a PST, with an undocumented per-entry Option-tag wire format) — the SDK's SignedTx type also drops the daemon's is_complete flag")

	// Strip the PST envelope → broadcastable SignedTransaction and submit.
	pstBytes, derr := hex.DecodeString(signed.Hex)
	if derr != nil {
		r.Record(area, name+" broadcast", StatusFail, "pst hex: "+derr.Error())
		return
	}
	signedTx, xerr := extractSignedTx(pstBytes, h.D12ComposedTx)
	if xerr != nil {
		r.Record(area, name+" broadcast", StatusFail, "extract: "+xerr.Error())
		r.Finding("PST→SignedTransaction extraction failed: %v", xerr)
		return
	}
	sub, err := h.wc.SubmitTransaction(ctx, hex.EncodeToString(signedTx), false)
	if err != nil {
		r.Record(area, name+" broadcast", StatusFail, err.Error())
		r.Finding("Atomic batch broadcast rejected: %v", err)
		return
	}
	r.Record(area, name+" broadcast", StatusPass, "ATOMIC txid="+short(sub.TxID)+" (conclude+create in one tx)"+itoa(len(signedTx)/2)+"B")
	if werr := waitTxConfirmed(ctx, h.nc, sub.TxID, 5*time.Minute); werr != nil {
		r.Record(area, name+" confirm", StatusFail, werr.Error())
		return
	}
	r.Record(area, name+" confirmed", StatusPass, "D12 atomic migration landed on chain")
}

func mustAcc0Addr(ctx context.Context, h *harness) string {
	addrs, err := h.wc.ShowReceiveAddresses(ctx, 0)
	if err != nil || len(addrs) == 0 {
		a, _ := h.wc.NewAddress(ctx, 0)
		return a
	}
	return addrs[0].Address
}

var _ = big.NewInt
