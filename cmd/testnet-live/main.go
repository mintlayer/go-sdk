// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mintlayer/go-sdk/node"
	"github.com/mintlayer/go-sdk/wallet"
	mintlayer "github.com/mintlayer/go-sdk/wasm"
)

// Config carries the deployment endpoints and run options.
type Config struct {
	WalletURL     string
	NodeURL       string
	RPCUser       string
	RPCPass       string
	HostWalletDir string // host-side mount of the daemon's /home/mintlayer
	ThrowWallet   string // daemon-side path of the throwaway wallet file
	Timeout       time.Duration
}

// fundingUTXO is a discovered spendable outpoint for compose flows.
type fundingUTXO struct {
	TxID  string
	Vout  uint32
	Atoms string
	Dest  string
}

// harness wires the clients, provisioned fixtures and shared state across
// the area checks.
type harness struct {
	cfg  Config
	wc   *wallet.Client
	nc   *node.Client
	wasm *mintlayer.Client
	Net  mintlayer.Network

	Funding             *fundingUTXO
	TokenID             string // live demo token (USDT)
	ThrowTokenID        string // throwaway freezable token minted by this run
	OrderID             string // D12 fixture order
	OrderTxID           string
	D12SignedPST        string
	D12ComposedTx       []byte
	CounterpartyAccount uint32
	restore             func(string)
}

// Join joins path elements (daemon-side paths).
func (h *harness) Join(elems ...string) string { return filepath.Join(elems...) }

func main() {
	var (
		cfg      Config
		areas    string
		include  string
		budget   string
		dryAreas []string
	)
	flag.StringVar(&cfg.WalletURL, "wallet-url", envOr("WALLET_URL", "http://127.0.0.1:13040"), "wallet daemon RPC")
	flag.StringVar(&cfg.NodeURL, "node-url", envOr("NODE_URL", "http://127.0.0.1:13030"), "node daemon RPC")
	flag.StringVar(&cfg.RPCUser, "rpc-user", envOr("RPC_USER", "mm"), "RPC user")
	flag.StringVar(&cfg.RPCPass, "rpc-pass", envOr("RPC_PASS", "mm-testnet-demo"), "RPC password")
	flag.StringVar(&cfg.HostWalletDir, "host-wallet-dir", envOr("HOST_WALLET_DIR", "../market_maker/run/docker-data"), "host mount of daemon /home/mintlayer")
	flag.StringVar(&cfg.ThrowWallet, "throw-wallet", "/home/mintlayer/live-test-wallet.dat", "daemon-side throwaway wallet path")
	flag.StringVar(&areas, "areas", "wallet,tx,orders,tokens,node,wasm", "comma list of areas to run")
	flag.StringVar(&include, "include", "", "comma list: slow,destructive")
	flag.StringVar(&budget, "budget", "150000000000", "fee budget in atoms (default 1.5 ML)")
	flag.Parse()
	cfg.Timeout = 20 * time.Minute

	want := map[string]bool{}
	for _, a := range strings.Split(areas, ",") {
		want[strings.TrimSpace(a)] = true
	}
	slow := strings.Contains(include, "slow")
	destructive := strings.Contains(include, "destructive")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	fmt.Println("== testnet-live: SDK surface validation against live testnet")
	fmt.Printf("   wallet=%s node=%s areas=%s include=%q\n", cfg.WalletURL, cfg.NodeURL, areas, include)

	h := &harness{cfg: cfg, Net: mintlayer.Testnet, TokenID: demoTokenID()}
	h.wc = wallet.New(cfg.WalletURL, wallet.WithBasicAuth(cfg.RPCUser, cfg.RPCPass), wallet.WithTimeout(60*time.Second))
	h.nc = node.New(cfg.NodeURL, node.WithBasicAuth(cfg.RPCUser, cfg.RPCPass), node.WithTimeout(30*time.Second))

	var err error
	if h.wasm, err = mintlayer.New(ctx); err != nil {
		fmt.Println("FATAL: wasm init:", err)
		os.Exit(1)
	}
	if err := h.wc.OpenWallet(ctx, "/home/mintlayer/wallet.dat", ""); err != nil {
		fmt.Println("note: open bot wallet:", err)
	}
	if err := syncToTip(ctx, h.wc, h.nc, 10*time.Minute); err != nil {
		fmt.Println("FATAL: wallet sync:", err)
		os.Exit(1)
	}

	r := NewRunner(budget)

	// Provision shared fixtures: counterparty account + funding utxo + throwaway token.
	provisioned := runProvision(ctx, r, h, want)

	if want["wallet"] {
		runWalletReadOnly(ctx, r, h)
		runWalletLifecycle(ctx, r, h, 5*time.Minute, slow, destructive)
	}
	if want["wallet"] {
		// lifecycle phase may have left the throwaway open — restore the bot wallet
		if err := h.wc.OpenWallet(ctx, "/home/mintlayer/wallet.dat", ""); err != nil {
			r.Record("wallet", "reopen bot wallet", StatusFail, err.Error())
		}
		if err := syncToTip(ctx, h.wc, h.nc, 5*time.Minute); err != nil {
			r.Record("wallet", "resync bot wallet", StatusFail, err.Error())
		}
	}
	if want["tx"] {
		runTxArea(ctx, r, h, 10*time.Minute)
	}
	if want["tokens"] {
		if provisioned {
			runTokensProvision(ctx, r, h, 10*time.Minute)
			runTokensLifecycle(ctx, r, h, 10*time.Minute)
			runTokensLifecycleReal(ctx, r, h, 10*time.Minute)
		} else {
			r.Record("tokens", "provision", StatusSkip, "provisioning failed upstream")
		}
	}
	if want["orders"] {
		runOrdersArea(ctx, r, h, 10*time.Minute)
	}
	if want["node"] {
		runNodeArea(ctx, r, h)
	}
	if want["wasm"] {
		runWasmArea(ctx, r, h)
	}
	_ = dryAreas

	code := r.Summary()
	fmt.Printf("\nfee spend: %s atoms (%s ML)\n", r.SpentAtoms, atomsToML(r.SpentAtoms))
	for _, f := range r.Findings {
		fmt.Println("FINDING:", f)
	}
	os.Exit(code)
}

// runProvision sets up the counterparty account, the funding utxo and the
// live demo token id; returns false when the environment is unusable.
func runProvision(ctx context.Context, r *Runner, h *harness, want map[string]bool) bool {
	// counterparty account (idempotent; reuse when present)
	info, err := h.wc.GetWalletInfo(ctx)
	if err != nil {
		r.Record("provision", "GetWalletInfo", StatusFail, err.Error())
		return false
	}
	_ = info
	if acc, err := h.wc.CreateAccount(ctx, "testnet-live"); err == nil {
		h.CounterpartyAccount = acc.Account
		r.Record("provision", "CreateAccount(counterparty)", StatusPass, fmt.Sprintf("account %d", acc.Account))
	} else {
		r.Record("provision", "CreateAccount(counterparty)", StatusSkip, "exists or refused: "+err.Error())
	}
	// funding utxo for compose flows
	fund, err := h.wc.NewAddress(ctx, 0)
	if err != nil {
		r.Record("provision", "NewAddress(funding)", StatusFail, err.Error())
		return false
	}
	send, err := h.wc.AddressSend(ctx, wallet.SendParams{
		Account: 0, Address: fund, Amount: wallet.Amount{Atoms: "300000000000"}, // 3 ML
	})
	if err != nil {
		r.Record("provision", "fundAddress", StatusFail, err.Error())
		return false
	}
	h.Funding = &fundingUTXO{TxID: send.TxID, Atoms: "295000000000", Dest: fund}
	r.Record("provision", "fundAddress", StatusPass, "tx "+short(send.TxID))
	return true
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
