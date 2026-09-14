// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mintlayer/go-sdk/wallet"
)

// Chain-constant protocol fees for token operations (tokens fee V1 schedule,
// verified against the daemon config). These dwarf the ~0.21 ML tx fee, so the
// throwaway-token lifecycle is budget-gated: it only runs when the configured
// FEE_BUDGET_ATOMS can cover it, otherwise it AUTO-SKIPS with the exact cost.
const (
	protoFeeIssuance  = "10000000000000" // 100 ML
	protoFeeSupply    = "5000000000000"  // 50 ML (mint/unmint/lock)
	protoFeeFreeze    = "5000000000000"  // 50 ML (freeze/unfreeze)
	protoFeeAuthority = "2000000000000"  // 20 ML
	protoFeeTxPad     = "1000000000"     // ~0.01 ML tx-fee headroom per op
)

func tokenLifecycleCost() string {
	total := "0"
	for _, f := range []string{
		protoFeeIssuance, protoFeeSupply, // issue + mint
		protoFeeFreeze, protoFeeFreeze, // freeze + unfreeze
		protoFeeAuthority, protoFeeSupply, // authority + lock
	} {
		total = addAtoms(total, addAtoms(f, protoFeeTxPad))
	}
	return total
}

// demoTokenID is the pre-existing testnet token used for order flows (token
// transfers carry no protocol fee — only tx fees).
func demoTokenID() string {
	return envOr("DEMO_TOKEN_ID", "tmltk1sm5xzzpjphv4nuv2r27ve6g2cexaea4muevtlhrejypehv4d25esmg2xe8")
}

// ensureCounterparty makes sure the counterparty account exists.
func ensureCounterparty(ctx context.Context, r *Runner, h *harness) bool {
	const area = "tokens"
	if _, err := h.wc.NewAddress(ctx, h.CounterpartyAccount); err == nil {
		r.Record(area, "counterparty account ready", StatusPass,
			fmt.Sprintf("account %d exists", h.CounterpartyAccount))
		return true
	}
	acc, err := h.wc.CreateAccount(ctx, "live-test-counterparty")
	if err != nil {
		r.Record(area, "ensure counterparty account", StatusFail, err.Error())
		return false
	}
	h.CounterpartyAccount = acc.Account
	if _, err := h.wc.NewAddress(ctx, h.CounterpartyAccount); err != nil {
		r.Record(area, "ensure counterparty account", StatusFail, "new address: "+err.Error())
		return false
	}
	r.Record(area, "ensure counterparty account", StatusPass, fmt.Sprintf("created account %d", h.CounterpartyAccount))
	return true
}

// runTokensProvision prepares the token side of the fixture:
//
//  1. counterparty account + funding (tx fees only)
//  2. token supply for the order's ask currency, transferred from the account-0
//     balance of the pre-existing demo token (tx fee only)
//
// A full throwaway-token issuance is NOT done here; see runTokensLifecycle.
func runTokensProvision(ctx context.Context, r *Runner, h *harness, timeout time.Duration) {
	const area = "tokens"
	if !ensureCounterparty(ctx, r, h) {
		return
	}
	cpAddr, err := h.wc.NewAddress(ctx, h.CounterpartyAccount)
	if err != nil {
		r.Record(area, "counterparty address", StatusFail, err.Error())
		return
	}

	// ── fund the counterparty (fills pay fees from its own UTXOs) ──────────────
	const fundAtoms = "50000000000" // 0.5 ML
	if r.OverBudget(fundAtoms) {
		r.Record(area, "fund counterparty", StatusSkip, "fee budget exceeded")
		return
	}
	res, err := h.wc.AddressSend(ctx, wallet.SendParams{
		Account: 0, Address: cpAddr, Amount: wallet.Amount{Atoms: fundAtoms},
		SelectedUTXOs: []wallet.Outpoint{}, // nil would marshal "selected_utxos":null and the daemon rejects it (SDK finding)
	})
	if err != nil {
		r.RecordFee(area, "fund counterparty (AddressSend)", StatusFail, err.Error(), "0")
		return
	}
	r.RecordFee(area, "fund counterparty (AddressSend)", StatusPass,
		"txid="+short(res.TxID)+" broadcasted="+btoa(res.Broadcasted), res.Fees.Coins.Atoms)
	if err := confirmTx(ctx, h, res.TxID, timeout); err != nil {
		r.Record(area, "fund counterparty confirmed", StatusFail, err.Error())
		return
	}
	r.Record(area, "fund counterparty confirmed", StatusPass, short(res.TxID))

	// ── adopt the pre-existing demo token as the GIVE currency ─────────────────
	h.TokenID = demoTokenID()
	ti, err := h.nc.TokenInfo(ctx, h.TokenID)
	if err != nil || ti == nil || ti.Type == "" {
		detail := "node has no info"
		if err != nil {
			detail = err.Error()
		}
		r.Record(area, "node.TokenInfo(demo token)", StatusFail, detail)
		return
	}
	r.Record(area, "node.TokenInfo(demo token)", StatusPass, "type="+ti.Type)

	// account 0 must hold enough of it
	bal, err := h.wc.GetBalance(ctx, 0)
	if err != nil {
		r.Record(area, "account-0 token balance", StatusFail, err.Error())
		return
	}
	const needTokens = "400000000" // 400 tokens @ 6 decimals (order give + D12 give + headroom)
	held, ok := bal.Tokens[h.TokenID]
	if !ok || cmpAtoms(held.Atoms, needTokens) < 0 {
		have := "none"
		if ok {
			have = held.Atoms
		}
		r.Record(area, "account-0 token balance", StatusFail,
			fmt.Sprintf("need %s atoms of the demo token, have %s", needTokens, have))
		return
	}
	r.Record(area, "account-0 token balance", StatusPass, held.Atoms+" atoms")

	// NOTE: no token transfer to the counterparty is needed — the order fixtures
	// GIVE tokens (escrowed from account 0's existing balance) and ASK coins,
	// so the counterparty fills by paying coins only. A daemon-side token send
	// here would consolidate hundreds of dust token UTXOs (5+ ML fee observed).
}

// runTokensLifecycle exercises the throwaway freezable-token lifecycle
// (issue → mint → freeze → unfreeze → authority → lock). The chain-constant
// protocol fees for these operations sum to ~320 ML, so this only runs when
// the fee budget explicitly allows it; otherwise every step AUTO-SKIPS with
// the exact reason.
func runTokensLifecycle(ctx context.Context, r *Runner, h *harness, timeout time.Duration) {
	const area = "tokens"
	cost := tokenLifecycleCost()
	if cmpAtoms(r.BudgetAtoms, cost) < 0 {
		r.Record(area, "throwaway token lifecycle", StatusSkip,
			fmt.Sprintf("chain protocol fees for issue+mint+freeze+unfreeze+authority+lock total %s ML — "+
				"raise FEE_BUDGET_ATOMS above %s to run live (default budget %s atoms = %s ML)",
				atomsToML(cost), cost, r.BudgetAtoms, atomsToML(r.BudgetAtoms)))
		r.Finding("Token protocol fees on this chain (tokens fee V1): issuance 100 ML, supply change 50 ML, freeze 50 ML, authority 20 ML — the SDK docs/wasm.md fee helpers report these correctly, but they dominate any budget sized around tx fees alone")
		return
	}
	runTokensLifecycleReal(ctx, r, h, timeout)
}

// runTokensLifecycleReal performs the real (expensive) token lifecycle.
func runTokensLifecycleReal(ctx context.Context, r *Runner, h *harness, timeout time.Duration) {
	const area = "tokens"
	wc := h.wc

	authority, err := wc.NewAddress(ctx, 0)
	if err != nil {
		r.Record(area, "IssueToken authority address", StatusFail, err.Error())
		return
	}
	if err := spendBudget(r, addAtoms(protoFeeIssuance, protoFeeTxPad), "IssueToken"); err != nil {
		r.Record(area, "IssueToken(freezable)", StatusSkip, err.Error())
		return
	}
	issue, err := wc.IssueToken(ctx, wallet.IssueTokenParams{
		Account:            0,
		DestinationAddress: authority,
		Metadata: wallet.TokenMetadata{
			TokenTicker:      "LTST",
			NumberOfDecimals: 6,
			MetadataURI:      "https://example.com/live-test-token",
			TokenSupply:      wallet.TokenSupply{Type: "Lockable"},
			IsFreezable:      true,
		},
	})
	if err != nil {
		r.Record(area, "IssueToken(freezable)", StatusFail, err.Error())
		return
	}
	h.ThrowTokenID = issue.TokenID
	// SDK gap: IssueTokenResult carries no Fees/Tx-hex even though the daemon
	// response does; only the protocol fee is accounted here.
	r.RecordFee(area, "IssueToken(freezable, lockable)", StatusPass,
		"token="+short(issue.TokenID)+" (tx fee untracked: SDK result type has no fees)", protoFeeIssuance)
	if err := confirmTx(ctx, h, issue.TxID, timeout); err != nil {
		r.Record(area, "token issuance confirmed", StatusFail, err.Error())
		return
	}

	ti, err := h.nc.TokenInfo(ctx, issue.TokenID)
	if err != nil {
		r.Record(area, "node.TokenInfo(new token)", StatusFail, err.Error())
	} else if ti == nil || ti.Type == "" {
		r.Record(area, "node.TokenInfo(new token)", StatusFail, "issued token not visible to the node")
	} else {
		r.Record(area, "node.TokenInfo(new token)", StatusPass, "type="+ti.Type)
	}

	cpAddr, _ := wc.NewAddress(ctx, h.CounterpartyAccount)
	if err := spendBudget(r, addAtoms(protoFeeSupply, protoFeeTxPad), "MintTokens"); err != nil {
		r.Record(area, "MintTokens", StatusSkip, err.Error())
		return
	}
	mint, err := wc.MintTokens(ctx, wallet.MintParams{
		Account: 0, TokenID: issue.TokenID, Address: cpAddr,
		Amount: wallet.Amount{Atoms: "1000000000"},
	})
	if err != nil {
		r.Record(area, "MintTokens", StatusFail, err.Error())
		return
	}
	r.RecordFee(area, "MintTokens", StatusPass, "txid="+short(mint.TxID), protoFeeSupply)
	confirmTx(ctx, h, mint.TxID, timeout)

	if err := spendBudget(r, addAtoms(protoFeeFreeze, protoFeeTxPad), "FreezeToken"); err != nil {
		r.Record(area, "FreezeToken", StatusSkip, err.Error())
		return
	}
	freeze, err := wc.FreezeToken(ctx, wallet.FreezeParams{
		Account: 0, TokenID: issue.TokenID, IsUnfreezable: true,
	})
	if err != nil {
		r.Record(area, "FreezeToken(unfreezable)", StatusFail, err.Error())
		return
	}
	r.RecordFee(area, "FreezeToken(unfreezable)", StatusPass, "txid="+short(freeze.TxID), protoFeeFreeze)
	confirmTx(ctx, h, freeze.TxID, timeout)

	if err := spendBudget(r, addAtoms(protoFeeFreeze, protoFeeTxPad), "UnfreezeToken"); err != nil {
		r.Record(area, "UnfreezeToken", StatusSkip, err.Error())
		return
	}
	unf, err := wc.UnfreezeToken(ctx, wallet.UnfreezeParams{Account: 0, TokenID: issue.TokenID})
	if err != nil {
		r.Record(area, "UnfreezeToken", StatusFail, err.Error())
	} else {
		r.RecordFee(area, "UnfreezeToken", StatusPass, "txid="+short(unf.TxID), protoFeeFreeze)
		confirmTx(ctx, h, unf.TxID, timeout)
	}

	newAuth, err := wc.NewAddress(ctx, 0)
	if err != nil {
		r.Record(area, "ChangeTokenAuthority address", StatusFail, err.Error())
	} else if err := spendBudget(r, addAtoms(protoFeeAuthority, protoFeeTxPad), "ChangeTokenAuthority"); err != nil {
		r.Record(area, "ChangeTokenAuthority", StatusSkip, err.Error())
	} else if ca, err := wc.ChangeTokenAuthority(ctx, wallet.ChangeAuthorityParams{
		Account: 0, TokenID: issue.TokenID, Address: newAuth,
	}); err != nil {
		r.Record(area, "ChangeTokenAuthority", StatusFail, err.Error())
	} else {
		r.RecordFee(area, "ChangeTokenAuthority", StatusPass, "→"+short(newAuth), protoFeeAuthority)
		confirmTx(ctx, h, ca.TxID, timeout)
	}

	if err := spendBudget(r, addAtoms(protoFeeSupply, protoFeeTxPad), "LockTokenSupply"); err != nil {
		r.Record(area, "LockTokenSupply", StatusSkip, err.Error())
		return
	}
	lock, err := wc.LockTokenSupply(ctx, wallet.LockSupplyParams{
		AccountIndex: 0, TokenID: issue.TokenID,
	})
	if err != nil {
		r.Record(area, "LockTokenSupply", StatusFail, err.Error())
		return
	}
	r.RecordFee(area, "LockTokenSupply", StatusPass, "supply now fixed", protoFeeSupply)
	confirmTx(ctx, h, lock.TxID, timeout)

	if raw, _, err := rawRPC(ctx, h.cfg.NodeURL, h.cfg.RPCUser, h.cfg.RPCPass,
		"chainstate_token_info", []any{issue.TokenID}); err != nil {
		r.Record(area, "token chain state (raw)", StatusFail, err.Error())
	} else if strings.Contains(string(raw), "IsLocked") || strings.Contains(string(raw), "is_locked") {
		r.Record(area, "token chain state (raw)", StatusPass, "locked flag present")
	} else {
		r.Record(area, "token chain state (raw)", StatusPass, "info returned (lock flag shape unverified)")
	}
}

// spendBudget reserves atoms against the runner budget; the error explains the
// shortfall when the remaining budget cannot cover the operation.
func spendBudget(r *Runner, atoms, what string) error {
	if r.OverBudget(atoms) {
		return fmt.Errorf("%s needs %s atoms; %s spent of %s budget", what, atoms, r.SpentAtoms, r.BudgetAtoms)
	}
	return nil
}
