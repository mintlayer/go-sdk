// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/mintlayer/go-sdk/node"
	"github.com/mintlayer/go-sdk/wallet"
)

// ── amount helpers (atoms are decimal strings; 1 ML = 1e11 atoms) ─────────────

func mustAtoms(s string) *big.Int {
	n, ok := new(big.Int).SetString(strings.TrimSpace(s), 10)
	if !ok {
		panic("bad atom amount: " + s)
	}
	return n
}

func addAtoms(a, b string) string {
	return new(big.Int).Add(mustAtoms(a), mustAtoms(b)).String()
}

func subAtoms(a, b string) string {
	return new(big.Int).Sub(mustAtoms(a), mustAtoms(b)).String()
}

func cmpAtoms(a, b string) int { return mustAtoms(a).Cmp(mustAtoms(b)) }

func atomsToML(atoms string) string {
	n := mustAtoms(atoms)
	whole := new(big.Int).Div(n, big.NewInt(1e11))
	frac := new(big.Int).Mod(n, big.NewInt(1e11))
	s := frac.String()
	for len(s) < 11 {
		s = "0" + s
	}
	s = strings.TrimRight(s, "0")
	if s == "" {
		return whole.String()
	}
	return whole.String() + "." + s
}

// feeForSize estimates the fee for a transaction of size bytes at rate atoms/KB,
// padded by 30% so the manually built D12 transaction clears the minimum rate.
func feeForSize(size uint32, ratePerKB string) string {
	fee := new(big.Int).Mul(big.NewInt(int64(size)), mustAtoms(ratePerKB))
	fee.Div(fee, big.NewInt(1000))
	fee.Mul(fee, big.NewInt(130)) // 30% padding
	fee.Div(fee, big.NewInt(100))
	return fee.String()
}

// ── raw JSON-RPC helper (used to inspect daemon responses the SDK drops) ──────

type rawResp struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    any    `json:"data"`
	} `json:"error"`
}

func rawRPC(ctx context.Context, url, user, pass, method string, params any) (json.RawMessage, *rawResp, error) {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(user, pass)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	var rr rawResp
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return nil, nil, err
	}
	return rr.Result, &rr, nil
}

// ── chain sync helpers ────────────────────────────────────────────────────────

// nodeHeight returns the current best block height.
func nodeHeight(ctx context.Context, nc *node.Client) (uint64, error) {
	return nc.BestBlockHeight(ctx)
}

// waitTxConfirmed polls until txid is no longer in the mempool (i.e. included
// in a block or evicted), then syncs the wallet so balances/history update.
func waitTxConfirmed(ctx context.Context, nc *node.Client, txid string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		mt, err := nc.GetTransaction(ctx, txid)
		if err != nil || mt == nil {
			break // left the mempool
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for %s to confirm", short(txid))
		}
		time.Sleep(3 * time.Second)
	}
	return nil
}

// waitForNewBlock waits until the node tip advances past `from`.
func waitForNewBlock(ctx context.Context, nc *node.Client, from uint64, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		h, err := nc.BestBlockHeight(ctx)
		if err == nil && h > from {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for block after height %d", from)
		}
		time.Sleep(2 * time.Second)
	}
}

// syncToTip syncs the wallet and waits until its best block reaches the node tip.
func syncToTip(ctx context.Context, wc *wallet.Client, nc *node.Client, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		tip, err := nc.BestBlockHeight(ctx)
		if err == nil {
			if serr := wc.SyncWallet(ctx); serr == nil {
				bb, berr := wc.BestBlock(ctx)
				if berr == nil && bb.Height >= tip-1 {
					return nil
				}
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout syncing wallet to tip")
		}
		time.Sleep(2 * time.Second)
	}
}

func short(s string) string {
	if len(s) > 16 {
		return s[:16] + "…"
	}
	return s
}

func uitoa(u uint64) string         { return strconv.FormatUint(u, 10) }
func itoa(i int) string             { return strconv.Itoa(i) }
func btoa(b bool) string            { return strconv.FormatBool(b) }
func hexs(b []byte) string          { return hex.EncodeToString(b) }
func osExecutable() (string, error) { return os.Executable() }

// combinedOutput runs a command with a hard timeout and returns its output.
func combinedOutput(cmd *exec.Cmd) ([]byte, error) {
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Start(); err != nil {
		return buf.Bytes(), err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return buf.Bytes(), err
	case <-time.After(60 * time.Second):
		_ = cmd.Process.Kill()
		return buf.Bytes(), fmt.Errorf("child timed out")
	}
}

// ── transaction JSON parsing (wallet transaction_get shape) ───────────────────
//
// The wallet returns: [{"V1":{"flags":0,"inputs":[...],"outputs":[...],"version":1}},{...status...}]
// inputs are externally tagged: {"Utxo":{"id":{"Transaction":"<hex>"},"index":n}}

type txJSON struct {
	V1 struct {
		Flags   uint64            `json:"flags"`
		Inputs  []json.RawMessage `json:"inputs"`
		Outputs []json.RawMessage `json:"outputs"`
		Version uint64            `json:"version"`
	} `json:"V1"`
}

// unwrapTxJSON accepts the [tx, status] array shape and the bare object shape.
func unwrapTxJSON(raw json.RawMessage) (json.RawMessage, error) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr[0], nil
	}
	return raw, nil
}

type utxoInputJSON struct {
	Utxo *struct {
		ID    map[string]string `json:"id"` // {"Transaction":"hex"} or {"BlockReward":"hex"}
		Index uint32            `json:"index"`
	} `json:"Utxo"`
}

// decodeTransfer unwraps {"Transfer":[value, destination]} outputs.
// The wallet transaction JSON renders the destination as a bech32 string;
// the node renders it as a debug struct — callers must not assume either.
func decodeTransfer(o json.RawMessage) ([]json.RawMessage, bool) {
	var obj struct {
		Transfer []json.RawMessage `json:"Transfer"`
	}
	if err := json.Unmarshal(o, &obj); err != nil || len(obj.Transfer) < 2 {
		return nil, false
	}
	return obj.Transfer, true
}

// parseTxInputs extracts (sourceIDHex, isBlockReward, outputIndex) for each input.
func parseTxInputs(raw json.RawMessage) ([]struct {
	Source   string
	IsReward bool
	Index    uint32
}, error) {
	inner, err := unwrapTxJSON(raw)
	if err != nil {
		return nil, err
	}
	var t txJSON
	if err := json.Unmarshal(inner, &t); err != nil {
		return nil, fmt.Errorf("parse tx json: %w", err)
	}
	var out []struct {
		Source   string
		IsReward bool
		Index    uint32
	}
	for _, in := range t.V1.Inputs {
		var u utxoInputJSON
		if err := json.Unmarshal(in, &u); err != nil {
			return nil, fmt.Errorf("parse input %q: %w", short(string(in)), err)
		}
		if u.Utxo == nil {
			return nil, fmt.Errorf("unsupported input kind: %s", short(string(in)))
		}
		if hexID, ok := u.Utxo.ID["Transaction"]; ok {
			out = append(out, struct {
				Source   string
				IsReward bool
				Index    uint32
			}{hexID, false, u.Utxo.Index})
			continue
		}
		if blockID, ok := u.Utxo.ID["BlockReward"]; ok {
			out = append(out, struct {
				Source   string
				IsReward bool
				Index    uint32
			}{blockID, true, u.Utxo.Index})
			continue
		}
		return nil, fmt.Errorf("input id has neither Transaction nor BlockReward: %s", short(string(in)))
	}
	return out, nil
}

// transferOutputs extracts coin/token transfer outputs as (amountAtoms, tokenID, destinationBech32).
// The node returns the internal shape {"Transfer":[value,destination]} where the
// destination renders as a debug string; callers that re-encode UTXOs must pass
// the bech32 address they know owns the output instead.
type txOutput struct {
	Atoms    string
	TokenID  string // empty for coins
	Dest     string // bech32 destination as rendered by the wallet tx JSON
	RawShape string
}

func parseTransferOutputs(raw json.RawMessage) ([]txOutput, error) {
	inner, err := unwrapTxJSON(raw)
	if err != nil {
		return nil, err
	}
	var t txJSON
	if err := json.Unmarshal(inner, &t); err != nil {
		return nil, err
	}
	var out []txOutput
	for _, o := range t.V1.Outputs {
		transfer, ok := decodeTransfer(o)
		if !ok {
			continue // non-transfer output
		}
		var coin struct {
			Coin *struct {
				Atoms string `json:"atoms"`
			} `json:"Coin"`
			Token *struct {
				TokenID string `json:"token_id"`
				Amount  struct {
					Atoms string `json:"atoms"`
				} `json:"amount"`
			} `json:"Token"`
		}
		if err := json.Unmarshal(transfer[0], &coin); err != nil {
			continue
		}
		var dest string
		_ = json.Unmarshal(transfer[1], &dest)
		switch {
		case coin.Coin != nil:
			out = append(out, txOutput{Atoms: coin.Coin.Atoms, Dest: dest, RawShape: string(o)})
		case coin.Token != nil:
			out = append(out, txOutput{Atoms: coin.Token.Amount.Atoms, TokenID: coin.Token.TokenID, Dest: dest, RawShape: string(o)})
		}
	}
	return out, nil
}

// findTransferTo locates the first transfer output sending exactly `atoms` of
// coins to `dest` (matched by amount only — the node renders destinations as
// debug strings, so ownership is established by the caller's records).
func findTransferTo(raw json.RawMessage, atoms string) bool {
	outs, err := parseTransferOutputs(raw)
	if err != nil {
		return false
	}
	for _, o := range outs {
		if o.TokenID == "" && o.Atoms == atoms {
			return true
		}
	}
	return false
}

// indexedOutput is one transaction output with its position.
type indexedOutput struct {
	Index   uint32
	Atoms   string
	TokenID string // empty for coins
	Dest    string // bech32 destination as rendered by the wallet tx JSON
}

// parseIndexedOutputs returns all transfer outputs with their indices.
func parseIndexedOutputs(raw json.RawMessage) ([]indexedOutput, error) {
	inner, err := unwrapTxJSON(raw)
	if err != nil {
		return nil, err
	}
	var t txJSON
	if err := json.Unmarshal(inner, &t); err != nil {
		return nil, err
	}
	var out []indexedOutput
	for i, o := range t.V1.Outputs {
		transfer, ok := decodeTransfer(o)
		if !ok {
			continue
		}
		var coin struct {
			Coin *struct {
				Atoms string `json:"atoms"`
			} `json:"Coin"`
			Token *struct {
				TokenID string `json:"token_id"`
				Amount  struct {
					Atoms string `json:"atoms"`
				} `json:"amount"`
			} `json:"Token"`
		}
		if err := json.Unmarshal(transfer[0], &coin); err != nil {
			continue
		}
		var dest string
		_ = json.Unmarshal(transfer[1], &dest)
		switch {
		case coin.Coin != nil:
			out = append(out, indexedOutput{Index: uint32(i), Atoms: coin.Coin.Atoms, Dest: dest})
		case coin.Token != nil:
			out = append(out, indexedOutput{Index: uint32(i), Atoms: coin.Token.Amount.Atoms, TokenID: coin.Token.TokenID, Dest: dest})
		}
	}
	return out, nil
}

// jsonUnmarshal is a tiny indirection so area files need not import encoding/json.
func jsonUnmarshal(raw json.RawMessage, v any) error { return json.Unmarshal(raw, v) }

// confirmTx waits for a broadcast tx to leave the mempool, then syncs the wallet.
func confirmTx(ctx context.Context, h *harness, txid string, timeout time.Duration) error {
	if err := waitTxConfirmed(ctx, h.nc, txid, timeout); err != nil {
		return err
	}
	return syncToTip(ctx, h.wc, h.nc, timeout)
}
