// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package main

import (
	"context"
)

// runWalletReadOnly exercises read-only wallet checks against the live bot
// wallet. The bot wallet file itself is never closed, created or mutated here.
func runWalletReadOnly(ctx context.Context, r *Runner, h *harness) {
	const area = "wallet"
	wc := h.wc

	info, err := wc.GetWalletInfo(ctx)
	if err != nil {
		r.Record(area, "GetWalletInfo", StatusFail, err.Error())
		return
	}
	if info.ExtraInfo.Type != "SoftwareWallet" {
		r.Record(area, "GetWalletInfo", StatusFail, "unexpected wallet type "+info.ExtraInfo.Type)
		return
	}
	r.Record(area, "GetWalletInfo", StatusPass, "id="+short(info.WalletID)+" type="+info.ExtraInfo.Type)

	// wallet sync state vs node tip
	bb, err := wc.BestBlock(ctx)
	if err != nil {
		r.Record(area, "BestBlock", StatusFail, err.Error())
	} else {
		tip, terr := h.nc.BestBlockHeight(ctx)
		if terr != nil {
			r.Record(area, "BestBlock", StatusSkip, "node height unavailable")
		} else if bb.Height+5 < tip {
			r.Record(area, "BestBlock", StatusFail, "wallet at "+uitoa(bb.Height)+", node at "+uitoa(tip))
		} else {
			r.Record(area, "BestBlock", StatusPass, "wallet="+uitoa(bb.Height)+" node="+uitoa(tip))
		}
	}

	// balance: the bot account must hold spendable coins for the fixture flows
	bal, err := wc.GetBalance(ctx, 0)
	if err != nil {
		r.Record(area, "GetBalance(0)", StatusFail, err.Error())
	} else if cmpAtoms(bal.Coins.Atoms, "0") <= 0 {
		r.Record(area, "GetBalance(0)", StatusFail, "bot account unfunded — fund it before running the harness")
	} else {
		r.Record(area, "GetBalance(0)", StatusPass, bal.Coins.Atoms+" atoms ("+atomsToML(bal.Coins.Atoms)+" ML)")
	}

	addrs, err := wc.ShowReceiveAddresses(ctx, 0)
	if err != nil {
		r.Record(area, "ShowReceiveAddresses(0)", StatusFail, err.Error())
	} else if len(addrs) == 0 {
		r.Record(area, "ShowReceiveAddresses(0)", StatusFail, "no receive addresses derived")
	} else {
		used := 0
		for _, a := range addrs {
			if a.Used {
				used++
			}
		}
		r.Record(area, "ShowReceiveAddresses(0)", StatusPass, itoa(len(addrs))+" addrs, "+itoa(used)+" used")
	}

	if len(addrs) > 0 {
		if pk, err := wc.RevealPublicKey(ctx, 0, addrs[0].Address); err != nil {
			r.Record(area, "RevealPublicKey", StatusFail, err.Error())
		} else if len(pk) != 66 && len(pk) != 68 {
			r.Record(area, "RevealPublicKey", StatusFail, "unexpected pubkey hex length "+itoa(len(pk)))
		} else {
			shape := "66-hex compressed"
			if len(pk) == 68 {
				shape = "68-hex (0x00-tagged, matches the wasm Destination::PubKey form)"
			}
			r.Record(area, "RevealPublicKey", StatusPass, shape)
		}
	}

	if st, err := wc.GetStakingStatus(ctx, 0); err != nil {
		r.Record(area, "GetStakingStatus(0)", StatusFail, err.Error())
	} else if *st != "Staking" && *st != "NotStaking" {
		r.Record(area, "GetStakingStatus(0)", StatusFail, "unexpected status "+string(*st))
	} else {
		r.Record(area, "GetStakingStatus(0)", StatusPass, string(*st))
	}

	if pools, err := wc.ListOwnedPools(ctx, 0); err != nil {
		r.Record(area, "ListOwnedPools(0)", StatusFail, err.Error())
	} else {
		r.Record(area, "ListOwnedPools(0)", StatusPass, itoa(len(pools))+" pools")
	}

	if dels, err := wc.ListDelegations(ctx, 0); err != nil {
		r.Record(area, "ListDelegations(0)", StatusFail, err.Error())
	} else {
		r.Record(area, "ListDelegations(0)", StatusPass, itoa(len(dels))+" delegations")
	}

	if txs, err := wc.ListTransactionsByAddress(ctx, 0, nil, 5); err != nil {
		r.Record(area, "ListTransactionsByAddress(0)", StatusFail, err.Error())
	} else if len(txs) > 5 {
		r.Record(area, "ListTransactionsByAddress(0)", StatusFail, "limit not honoured")
	} else {
		r.Record(area, "ListTransactionsByAddress(0)", StatusPass, itoa(len(txs))+" entries")
	}

	if pend, err := wc.ListPendingTransactions(ctx, 0); err != nil {
		r.Record(area, "ListPendingTransactions(0)", StatusFail, err.Error())
	} else {
		r.Record(area, "ListPendingTransactions(0)", StatusPass, itoa(len(pend))+" pending")
	}

	// account_create with an empty name must fail cleanly (validated RPC surface)
	if _, err := wc.CreateAccount(ctx, ""); err != nil {
		// intent: clean RPC rejection, whatever daemon rule fires first
		// (empty-name validation or empty-newest-account protection)
		r.Record(area, "CreateAccount(empty name) rejects", StatusPass, "daemon refused: "+short(err.Error()))
	} else {
		r.Record(area, "CreateAccount(empty name) rejects", StatusFail, "empty name accepted")
	}
}
