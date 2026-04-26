// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// Command issue-token demonstrates issuing a new fungible token and minting an
// initial supply using the Mintlayer wallet daemon.
//
// The flow:
//
//  1. Open the wallet (or create one from a mnemonic).
//  2. Sync the wallet with the chain.
//  3. Derive a new receiving address to be the token authority.
//  4. Issue the token — the wallet signs, pays fees, and broadcasts the tx.
//  5. Mint an initial supply to the same address.
//
// Usage:
//
//	go run ./examples/issue-token \
//	  -wallet   /path/to/wallet.dat \
//	  -ticker   MYTOKEN \
//	  -decimals 2 \
//	  -supply   1000000 \
//	  -uri      "https://example.com/token-metadata.json" \
//	  -wallet-rpc http://127.0.0.1:3034
//
// Requirements:
//   - wallet-rpc-daemon must be running and have a wallet open (or use -create).
//   - The wallet account must hold enough TML to pay issuance fees.
//   - The indexer must have --enable-post-routes if you want to broadcast via it.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/mintlayer/go-sdk/wallet"
)

func main() {
	walletPath := flag.String("wallet", "", "path to wallet file (required unless already open)")
	walletPass := flag.String("password", "", "wallet password (empty for unencrypted)")
	ticker := flag.String("ticker", "", "token ticker symbol, e.g. MYTOKEN (required)")
	decimals := flag.Uint("decimals", 2, "number of decimal places (0–18)")
	supplyStr := flag.String("supply", "1000000", "initial mint supply in the token's smallest unit")
	metadataURI := flag.String("uri", "", "URL pointing to token metadata JSON")
	walletRPC := flag.String("wallet-rpc", "http://127.0.0.1:3034", "wallet daemon RPC endpoint")
	account := flag.Uint("account", 0, "wallet account index")
	flag.Parse()

	if *ticker == "" {
		flag.Usage()
		log.Fatal("required: -ticker")
	}

	ctx := context.Background()

	// ── 1. Connect to the wallet daemon ──────────────────────────────────────
	wc := wallet.New(*walletRPC)

	// Open the wallet if a path was provided. Skip if the daemon already has
	// a wallet open (e.g. from a previous session).
	if *walletPath != "" {
		if err := wc.OpenWallet(ctx, *walletPath, *walletPass); err != nil {
			log.Fatalf("open wallet: %v", err)
		}
		log.Printf("wallet opened: %s", *walletPath)
	}

	// ── 2. Sync the wallet ────────────────────────────────────────────────────
	if err := wc.SyncWallet(ctx); err != nil {
		// Non-fatal: the daemon may already be syncing.
		log.Printf("sync wallet: %v (continuing)", err)
	}

	// ── 3. Derive a fresh address to act as the token authority ───────────────
	//
	// The authority address is the address whose private key can later mint,
	// burn, freeze, or transfer the authority of the token.
	authorityAddr, err := wc.NewAddress(ctx, uint32(*account))
	if err != nil {
		log.Fatalf("new address: %v", err)
	}
	log.Printf("authority address: %s", authorityAddr)

	// Show current balance so the user can confirm there are enough funds.
	balance, err := wc.GetBalance(ctx, uint32(*account))
	if err != nil {
		log.Printf("get balance: %v (continuing)", err)
	} else {
		log.Printf("account balance: %s atoms (%s ML)", balance.Coins.Atoms, balance.Coins.Decimal)
	}

	// ── 4. Issue the token ────────────────────────────────────────────────────
	//
	// Token supply type "Lockable" means the supply is unlimited until you
	// explicitly call LockTokenSupply. Use "Fixed" (with a cap) or "Unlimited"
	// to change the supply policy.
	issueResult, err := wc.IssueToken(ctx, wallet.IssueTokenParams{
		Account:            uint32(*account),
		DestinationAddress: authorityAddr,
		Metadata: wallet.TokenMetadata{
			TokenTicker:      *ticker,
			NumberOfDecimals: uint8(*decimals),
			MetadataURI:      *metadataURI,
			TokenSupply:      wallet.TokenSupply{Type: "Lockable"},
			IsFreezable:      false,
		},
	})
	if err != nil {
		log.Fatalf("issue token: %v", err)
	}

	fmt.Printf("token issued\n")
	fmt.Printf("  token id: %s\n", issueResult.TokenID)
	fmt.Printf("  tx id:    %s\n", issueResult.TxID)

	// ── 5. Mint initial supply ─────────────────────────────────────────────────
	//
	// MintTokens creates new tokens and sends them to the given address.
	// The wallet must control the authority key.
	//
	// Note: wait for the issuance transaction to confirm before minting.
	// In production, poll indexer.GetTransaction until confirmations > 0.
	mintResult, err := wc.MintTokens(ctx, wallet.MintParams{
		Account: uint32(*account),
		TokenID: issueResult.TokenID,
		Address: authorityAddr,
		Amount:  wallet.Amount{Atoms: *supplyStr},
	})
	if err != nil {
		log.Fatalf("mint tokens: %v\n\n"+
			"Tip: the issuance tx may not have confirmed yet.\n"+
			"Wait for confirmation and re-run with -token-id flag.", err)
	}

	fmt.Printf("tokens minted\n")
	fmt.Printf("  tx id: %s\n", mintResult.TxID)
	fmt.Printf("  fees:  %s atoms\n", mintResult.Fees.Coins.Atoms)
}
