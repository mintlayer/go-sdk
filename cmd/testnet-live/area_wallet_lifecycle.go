// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mintlayer/go-sdk/wallet"
)

// lifecycle password for the destructive encryption phase
const harnessPassword = "live-test-harness"

// runWalletLifecycle closes the bot wallet, runs lifecycle checks against a
// throwaway wallet file at the CONTAINER path (the daemon's filesystem), and
// re-opens the bot wallet afterwards. main() registers restore() on SIGINT too.
func runWalletLifecycle(ctx context.Context, r *Runner, h *harness, timeout time.Duration, slow, destructive bool) {
	const area = "wallet-lifecycle"
	wc := h.wc
	cfg := h.cfg

	// ── close the bot wallet ───────────────────────────────────────────────────
	if err := wc.CloseWallet(ctx); err != nil {
		r.Record(area, "CloseWallet(bot)", StatusFail, err.Error())
		h.restore("lifecycle phase could not close the bot wallet")
		return
	}
	r.Record(area, "CloseWallet(bot)", StatusPass, "bot wallet closed for lifecycle phase")

	// ── fresh throwaway file (host-side cleanup when the mapped dir is known) ──
	if cfg.HostWalletDir != "" {
		host := filepath.Join(cfg.HostWalletDir, filepath.Base(cfg.ThrowWallet))
		if _, err := os.Stat(host); err == nil {
			if rmErr := os.Remove(host); rmErr != nil {
				r.Record(area, "clear throwaway file", StatusSkip, rmErr.Error())
			} else {
				r.Record(area, "clear throwaway file", StatusPass, host)
			}
		}
	}

	created, err := wc.CreateWallet(ctx, wallet.CreateWalletParams{
		Path:            cfg.ThrowWallet,
		StoreSeedPhrase: false,
	})
	if err != nil {
		// leftover from an earlier run: try opening it (password then none)
		if oerr := wc.OpenWallet(ctx, cfg.ThrowWallet, harnessPassword); oerr == nil {
			r.Record(area, "OpenWallet(throwaway, encrypted)", StatusPass, "reused encrypted leftover")
		} else if oerr2 := wc.OpenWallet(ctx, cfg.ThrowWallet, ""); oerr2 != nil {
			r.Record(area, "OpenWallet(throwaway)", StatusFail,
				"leftover wallet unopenable: "+firstErr(oerr, oerr2)+" — remove the file and rerun")
			h.restore("throwaway unusable")
			return
		} else {
			r.Record(area, "OpenWallet(throwaway)", StatusPass, "reused unencrypted leftover")
		}
	} else if created.Mnemonic == nil || created.Mnemonic.Content == nil || created.Mnemonic.Content.Mnemonic == "" {
		r.Record(area, "CreateWallet(throwaway)", StatusFail, "no mnemonic returned for store_seed_phrase=false")
		h.restore("throwaway unusable")
		return
	} else {
		words := len(splitWords(created.Mnemonic.Content.Mnemonic))
		r.Record(area, "CreateWallet(throwaway)", StatusPass,
			fmt.Sprintf("container path %s, %d-word mnemonic returned (not stored)", cfg.ThrowWallet, words))
	}

	info, err := wc.GetWalletInfo(ctx)
	if err != nil {
		r.Record(area, "GetWalletInfo(throwaway)", StatusFail, err.Error())
		h.restore("throwaway info failed")
		return
	}
	if info.ExtraInfo.Type != "SoftwareWallet" {
		r.Record(area, "GetWalletInfo(throwaway)", StatusFail, "type "+info.ExtraInfo.Type)
	} else {
		r.Record(area, "GetWalletInfo(throwaway)", StatusPass, "software wallet")
	}

	if acc, err := wc.CreateAccount(ctx, "live-test-throwaway"); err != nil {
		// Daemon rule: the newest account must have transaction history before
		// another one can be created — a fresh wallet can only have account 0.
		if strings.Contains(err.Error(), "Cannot create a new account") {
			r.Record(area, "CreateAccount(throwaway)", StatusSkip,
				"daemon refuses: newest account has no transaction history (expected on a fresh wallet)")
		} else {
			r.Record(area, "CreateAccount(throwaway)", StatusFail, err.Error())
		}
	} else {
		r.Record(area, "CreateAccount(throwaway)", StatusPass, "account "+uitoa(uint64(acc.Account)))
	}

	if addr, err := wc.NewAddress(ctx, 0); err != nil {
		r.Record(area, "NewAddress(throwaway)", StatusFail, err.Error())
	} else if !hasPrefixTMT(addr) {
		r.Record(area, "NewAddress(throwaway)", StatusFail, "bad address "+addr)
	} else {
		r.Record(area, "NewAddress(throwaway)", StatusPass, addr)
	}

	if bal, err := wc.GetBalance(ctx, 0); err != nil {
		r.Record(area, "GetBalance(throwaway)", StatusFail, err.Error())
	} else if bal.Coins.Atoms != "0" {
		r.Record(area, "GetBalance(throwaway)", StatusFail, "fresh wallet has balance?!")
	} else {
		r.Record(area, "GetBalance(throwaway)", StatusPass, "0 atoms (fresh)")
	}

	if err := wc.SyncWallet(ctx); err != nil {
		r.Record(area, "SyncWallet(throwaway)", StatusFail, err.Error())
	} else {
		r.Record(area, "SyncWallet(throwaway)", StatusPass, "synced")
	}

	if bb, err := wc.BestBlock(ctx); err != nil {
		r.Record(area, "BestBlock(throwaway)", StatusFail, err.Error())
	} else {
		r.Record(area, "BestBlock(throwaway)", StatusPass, "height="+uitoa(bb.Height))
	}

	// open/close roundtrip
	if err := wc.CloseWallet(ctx); err != nil {
		r.Record(area, "CloseWallet(throwaway)", StatusFail, err.Error())
	} else if err := wc.OpenWallet(ctx, cfg.ThrowWallet, ""); err != nil {
		r.Record(area, "OpenWallet(throwaway)", StatusFail, err.Error())
	} else {
		r.Record(area, "Open/CloseWallet roundtrip", StatusPass, "reopened without password")
	}

	// ── gated: rescan (slow) ───────────────────────────────────────────────────
	if !slow {
		r.Record(area, "RescanWallet", StatusSkip, "gated: run with --include=slow")
	} else {
		done := make(chan error, 1)
		go func() { done <- wc.RescanWallet(ctx) }()
		select {
		case err := <-done:
			if err != nil {
				r.Record(area, "RescanWallet", StatusFail, err.Error())
			} else {
				r.Record(area, "RescanWallet", StatusPass, "full chain rescan finished (slow gate on)")
			}
		case <-time.After(15 * time.Minute):
			r.Record(area, "RescanWallet", StatusFail, "timed out after 15m")
		}
	}

	// ── gated: destructive ops ─────────────────────────────────────────────────
	if !destructive {
		r.Record(area, "SweepSpendable(throwaway)", StatusSkip, "gated: run with --include=destructive")
		r.Record(area, "key encryption lifecycle", StatusSkip, "gated: run with --include=destructive")
	} else {
		// sweep on the empty throwaway: must return a clean RPC error, not a hang
		done := make(chan error, 1)
		go func() {
			_, err := wc.SweepSpendable(ctx, wallet.SweepParams{
				Account: 0, All: true,
				DestinationAddress: mustAcc0Addr(ctx, h),
			})
			done <- err
		}()
		select {
		case err := <-done:
			if err != nil {
				r.Record(area, "SweepSpendable(throwaway)", StatusPass, "clean error on empty wallet: "+short(err.Error()))
			} else {
				r.Record(area, "SweepSpendable(throwaway)", StatusFail, "empty wallet swept successfully?!")
			}
		case <-time.After(30 * time.Second):
			r.Record(area, "SweepSpendable(throwaway)", StatusFail, "timed out")
		}

		// key encryption lifecycle on the throwaway
		if err := wc.EncryptPrivateKeys(ctx, harnessPassword); err != nil {
			r.Record(area, "EncryptPrivateKeys", StatusFail, err.Error())
		} else {
			r.Record(area, "EncryptPrivateKeys", StatusPass, "keys encrypted")
		}
		if err := wc.UnlockPrivateKeys(ctx, harnessPassword); err != nil {
			r.Record(area, "UnlockPrivateKeys", StatusFail, err.Error())
		} else {
			r.Record(area, "UnlockPrivateKeys", StatusPass, "unlocked with password")
		}
		if err := wc.LockPrivateKeys(ctx); err != nil {
			r.Record(area, "LockPrivateKeys", StatusFail, err.Error())
		} else {
			r.Record(area, "LockPrivateKeys", StatusPass, "locked again")
		}
		// persistence: close + reopen now requires the password
		_ = wc.CloseWallet(ctx)
		if err := wc.OpenWallet(ctx, cfg.ThrowWallet, ""); err == nil {
			r.Record(area, "encrypted wallet rejects empty password", StatusFail, "opened without password")
			_ = wc.CloseWallet(ctx)
		} else {
			r.Record(area, "encrypted wallet rejects empty password", StatusPass, "")
		}
		if err := wc.OpenWallet(ctx, cfg.ThrowWallet, harnessPassword); err != nil {
			r.Record(area, "OpenWallet(encrypted)", StatusFail, err.Error())
		} else {
			r.Record(area, "OpenWallet(encrypted)", StatusPass, "reopened with password")
		}
	}

	// leave the throwaway closed; main() restores the bot wallet
	_ = wc.CloseWallet(ctx)
}

func splitWords(s string) []string {
	var out []string
	for _, w := range splitOnSpace(s) {
		out = append(out, w)
	}
	return out
}

func splitOnSpace(s string) []string {
	var out []string
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' || s[i] == '\n' {
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}

func hasPrefixTMT(addr string) bool {
	return len(addr) > 4 && addr[:4] == "tmt1"
}

var _ = fmt.Sprintf
