# Mintlayer Go SDK

A Go SDK for the [Mintlayer](https://www.mintlayer.org/) blockchain.

```
go get github.com/mintlayer/go-sdk
```

Requires Go 1.21+. No CGO. The WASM cryptography runtime is embedded in the binary.

---

## Overview

The SDK is organised as four independent sub-clients plus a top-level `Client` that wires them together.

| Package | Purpose | Default port |
|---|---|---|
| `github.com/mintlayer/go-sdk/node` | JSON-RPC 2.0 client for the node daemon | 3030 (mainnet) |
| `github.com/mintlayer/go-sdk/indexer` | REST client for the indexer (api-web-server) | 3000 |
| `github.com/mintlayer/go-sdk/wallet` | JSON-RPC 2.0 client for the wallet daemon | 3034 (mainnet) |
| `github.com/mintlayer/go-sdk/wasm` | Cryptography & transaction-building via WASM | — |

Use the top-level client when you need multiple sub-clients, or import sub-packages directly when you only need one.

---

## Quick start

```go
package main

import (
    "context"
    "fmt"
    "log"

    sdk "github.com/mintlayer/go-sdk"
)

func main() {
    client := sdk.New(sdk.Config{
        NodeURL:    "http://127.0.0.1:3030",
        IndexerURL: "http://127.0.0.1:3000",
        WalletURL:  "http://127.0.0.1:3034",
    })

    ctx := context.Background()

    // Query the chain tip from the indexer.
    tip, err := client.Indexer.GetTip(ctx)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("chain tip: height=%d id=%s\n", tip.BlockHeight, tip.BlockID)

    // Optionally initialise the embedded WASM cryptography runtime (~400 ms).
    if err := client.InitWASM(ctx); err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    privKey, err := client.WASM.MakePrivateKey()
    if err != nil {
        log.Fatal(err)
    }
    pubKey, err := client.WASM.PublicKeyFromPrivateKey(privKey)
    if err != nil {
        log.Fatal(err)
    }
    addr, err := client.WASM.PubkeyToPubkeyHashAddress(pubKey, sdk.Mainnet)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("address:", addr)
}
```

---

## Node client (`node/`)

JSON-RPC 2.0 client for the Mintlayer node daemon. Supports Basic Auth for nodes with authentication enabled.

```go
import "github.com/mintlayer/go-sdk/node"

c := node.New("http://127.0.0.1:3030",
    node.WithBasicAuth("user", "pass"), // optional
    node.WithTimeout(10*time.Second),   // optional
)

ctx := context.Background()

// Chain state
info, err := c.ChainstateInfo(ctx)
height, err := c.BestBlockHeight(ctx)
blockID, err := c.BestBlockID(ctx)

// Look up a block
blockHex, err := c.GetBlock(ctx, blockID)
blockJSON, err := c.GetBlockJSON(ctx, blockID)

// Token / order info
tokenInfo, err := c.TokenInfo(ctx, "ttml1...")
orderInfo, err := c.OrderInfo(ctx, "order1...")

// Mempool
err = c.SubmitTransaction(ctx, signedTxHex, node.TrustPolicyUntrusted)
feeRate, err := c.GetFeeRate(ctx, 1)

// P2P
peerCount, err := c.GetPeerCount(ctx)
peers, err := c.GetConnectedPeers(ctx)
```

Errors from the daemon are returned as `*node.RPCError` with a numeric `Code` and `Message`.

---

## Indexer client (`indexer/`)

REST client for `api-web-server`. All paths are relative to `/api/v2/`.

```go
import "github.com/mintlayer/go-sdk/indexer"

c := indexer.New("http://127.0.0.1:3000",
    indexer.WithTimeout(15*time.Second), // optional
)

ctx := context.Background()

// Chain
tip, err := c.GetTip(ctx)
blockIDAtHeight, err := c.GetBlockIDAtHeight(ctx, 100_000)

// Block
block, err := c.GetBlock(ctx, "00000000...")
txIDs, err := c.GetBlockTransactionIDs(ctx, "00000000...")

// Transaction
tx, err := c.GetTransaction(ctx, "aabbcc...")
txID, err := c.SubmitTransaction(ctx, signedTxHex) // requires --enable-post-routes

// Address
utxos, err := c.GetSpendableUTXOs(ctx, "mxtc1...")
info, err := c.GetAddressInfo(ctx, "mxtc1...")

// Pool / staking
pools, err := c.ListPools(ctx, indexer.PoolListOpts{Sort: "by_pledge"})
pool, err := c.GetPool(ctx, "pool1...")

// Tokens
token, err := c.GetToken(ctx, "ttml1...")
tokens, err := c.FindTokensByTicker(ctx, "MYTOKEN", indexer.PageOpts{Items: 10})

// Orders
orders, err := c.ListOrders(ctx, indexer.PageOpts{Offset: 0, Items: 20})
order, err := c.GetOrder(ctx, "order1...")

// Statistics
stats, err := c.GetCoinStatistics(ctx)
```

Non-2xx responses are returned as `*indexer.HTTPError` with a `StatusCode` field.

---

## Wallet client (`wallet/`)

JSON-RPC 2.0 client for `wallet-rpc-daemon`. The wallet daemon manages key storage, signing, and broadcasting.

```go
import "github.com/mintlayer/go-sdk/wallet"

c := wallet.New("http://127.0.0.1:3034",
    wallet.WithBasicAuth("user", "pass"), // optional
)

ctx := context.Background()

// Wallet lifecycle
err = c.OpenWallet(ctx, "/path/to/wallet.dat", "")
defer c.CloseWallet(ctx)
err = c.SyncWallet(ctx)

// Accounts and addresses
info, err := c.GetWalletInfo(ctx)
addr, err := c.NewAddress(ctx, 0 /*account*/)
balance, err := c.GetBalance(ctx, 0)

// Send coins (account 0, auto fee)
result, err := c.AddressSend(ctx, wallet.SendParams{
    Account:         0,
    Address:         "mxtc1...",
    Amount:          wallet.Amount{Atoms: "100000000000"}, // 1 ML
    SelectedUtxos:   nil,
    ChangeAddress:   nil,
    Options:         nil,
})
fmt.Println("tx id:", result.TxID)

// Token operations
issueResult, err := c.IssueToken(ctx, wallet.IssueTokenParams{
    Account:            0,
    Ticker:             "MYTOKEN",
    NumberOfDecimals:   2,
    MetadataUri:        "https://example.com/token",
    DestinationAddress: addr,
    TotalSupply:        wallet.TokenTotalSupplyParams{Type: "Lockable"},
    IsFreezable:        "No",
    Options:            nil,
})
mintResult, err := c.MintTokens(ctx, wallet.MintParams{
    Account: 0,
    TokenID: issueResult.TokenID,
    Address: addr,
    Amount:  wallet.Amount{Atoms: "1000"},
    Options: nil,
})

// Staking
err = c.StartStaking(ctx, 0)
pools, err := c.ListOwnedPools(ctx, 0)

// Compose and sign a raw transaction (cold wallet flow)
composed, err := c.ComposeTransaction(ctx, wallet.ComposeParams{...})
signed, err := c.SignRawTransaction(ctx, 0, composed.Hex)
submitResult, err := c.SubmitTransaction(ctx, signed.Hex, false)
```

---

## WASM client (`wasm/`)

Cryptographic primitives and binary transaction encoding via an embedded WebAssembly module.
Initialisation takes ~400 ms; use `sync.Once` or `Client.InitWASM` to call it once per process.

```go
import (
    "context"
    mintlayer "github.com/mintlayer/go-sdk/wasm"
)

ctx := context.Background()
c, err := mintlayer.New(ctx)
if err != nil {
    log.Fatal(err)
}
defer c.Close()

// Key derivation (BIP-44 path 44'/mintlayer_coin_type'/0')
mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
accountKey, err := c.MakeDefaultAccountPrivkey(mnemonic, mintlayer.Mainnet)
recvKey, err := c.MakeReceivingAddress(accountKey, 0)
pubKey, err := c.PublicKeyFromPrivateKey(recvKey)
addr, err := c.PubkeyToPubkeyHashAddress(pubKey, mintlayer.Mainnet)

// Transaction building
srcID, err := c.EncodeOutpointSourceId(txIDBytes, mintlayer.SourceTransaction)
input, err := c.EncodeInputForUtxo(srcID, outputIndex)

output, err := c.EncodeOutputTransfer(
    mintlayer.NewAmount("100000000000"), // 1 ML
    destAddr,
    mintlayer.Mainnet,
)

tx, err := c.EncodeTransaction(input, output, 0 /*flags*/)
txID, err := c.GetTransactionID(tx, true)

// Signing
witness, err := c.EncodeWitness(
    mintlayer.SigHashAll,
    recvKey,
    addr,
    tx,
    utxoBytes, // see examples/send-coins for encoding details
    0,         // input index
    mintlayer.TxAdditionalInfo{},
    blockHeight,
    mintlayer.Mainnet,
)
signedTx, err := c.EncodeSignedTransaction(tx, witness)
```

See [examples/send-coins/](examples/send-coins/main.go) for a complete end-to-end transaction flow.

---

## Top-level client

`sdk.New` constructs only the sub-clients whose URL is non-empty.

```go
// Node + WASM only — no wallet or indexer client is created.
client := sdk.New(sdk.Config{
    NodeURL:  "http://127.0.0.1:3030",
    Username: "user",
    Password: "pass",
})

// Call InitWASM before using client.WASM.
if err := client.InitWASM(ctx); err != nil {
    log.Fatal(err)
}
defer client.Close()
```

Convenience type aliases (`sdk.Amount`, `sdk.Network`, `sdk.Mainnet`, …) are re-exported so callers
that only import the top-level package do not need to also import `github.com/mintlayer/go-sdk/wasm`.

---

## Examples

| Example | Description |
|---|---|
| [examples/send-coins/](examples/send-coins/main.go) | Derive key → fetch UTXOs → build, sign, and submit a transaction |
| [examples/issue-token/](examples/issue-token/main.go) | Issue a fungible token and mint an initial supply via the wallet daemon |

---

## Amounts

All coin and token amounts use the `Amount` type, which stores the value as a decimal string of
_atoms_ — the smallest indivisible unit. **1 ML = 100,000,000,000 atoms** (11 decimal places).

```go
one := mintlayer.NewAmount("100000000000") // 1 ML
zero := mintlayer.NewAmountZero()
fmt.Println(one.Atoms()) // "100000000000"
```

The indexer and wallet clients use their own `Amount` struct with both `Atoms` and `Decimal` fields
populated by the server.

---

## Networks

| Constant | Value | Use |
|---|---|---|
| `Mainnet` | 0 | Production network |
| `Testnet` | 1 | Public test network |
| `Regtest` | 2 | Local regression testing |
| `Signet` | 3 | Signet |

Pass the network constant to any function that derives addresses or encodes transactions.

---

## Error handling

- `node.RPCError` — JSON-RPC error from the node daemon (`Code`, `Message`)
- `wallet.RPCError` — JSON-RPC error from the wallet daemon
- `indexer.HTTPError` — non-2xx HTTP response from the indexer (`StatusCode`, `Body`)
- WASM errors are plain `error` values with a descriptive message prefixed by `mintlayer:`.
