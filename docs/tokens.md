# Tokens and NFTs

Mintlayer supports on-chain fungible tokens and NFTs. The wallet daemon manages the full lifecycle. The `wasm` package provides low-level encoding for manual transaction flows.

---

## Fungible tokens

### Supply policies

When issuing a token you choose one of three supply policies:

| Policy | `TokenSupply.Type` | Description |
|--------|--------------------|-------------|
| Lockable | `"Lockable"` | Unlimited minting until `LockTokenSupply` is called, after which the supply is frozen permanently |
| Unlimited | `"Unlimited"` | Minting is always allowed with no cap |
| Fixed | `"Fixed"` | A cap is set at issuance; supply cannot exceed it |

### Issuing a token

```go
import "github.com/mintlayer/go-sdk/wallet"

wc := wallet.New("http://127.0.0.1:3034")

authorityAddr, err := wc.NewAddress(ctx, 0)

result, err := wc.IssueToken(ctx, wallet.IssueTokenParams{
    Account:            0,
    DestinationAddress: authorityAddr,
    Metadata: wallet.TokenMetadata{
        TokenTicker:      "MYTOKEN",
        NumberOfDecimals: 2,
        MetadataURI:      "https://example.com/token-metadata.json",
        TokenSupply:      wallet.TokenSupply{Type: "Lockable"},
        IsFreezable:      true,
    },
})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("token id: %s\n  tx id: %s\n", result.TokenID, result.TxID)
```

The `DestinationAddress` becomes the **authority address**: the key that controls future token operations (minting, freezing, authority transfer). Keep it secure.

For a `Fixed` supply cap, set the `Content` field:

```go
TokenSupply: wallet.TokenSupply{
    Type:    "Fixed",
    Content: &wallet.Amount{Atoms: "1000000"}, // hard cap
},
```

### Minting

Wait for the issuance transaction to confirm before minting.

```go
mintResult, err := wc.MintTokens(ctx, wallet.MintParams{
    Account: 0,
    TokenID: result.TokenID,
    Address: recipientAddr,
    Amount:  wallet.Amount{Atoms: "100000"}, // in smallest token units
})
fmt.Printf("minted tx id: %s\n", mintResult.TxID)
```

### Unminting

Returns tokens to an unminted state (removes them from circulation without burning).

```go
unmintResult, err := wc.UnmintTokens(ctx, wallet.UnmintParams{
    Account: 0,
    TokenID: "ttml1...",
    Amount:  wallet.Amount{Atoms: "50000"},
})
```

### Locking supply

After locking, the supply policy becomes `Fixed` at the current circulating supply. This operation is irreversible.

```go
lockResult, err := wc.LockTokenSupply(ctx, wallet.LockSupplyParams{
    AccountIndex: 0,
    TokenID:      "ttml1...",
})
```

Note: the field is `AccountIndex`, not `Account`.

### Freezing and unfreezing

Freezing prevents all transfers. If `IsUnfreezable` is `true`, the authority can unfreeze later.

```go
freezeResult, err := wc.FreezeToken(ctx, wallet.FreezeParams{
    Account:       0,
    TokenID:       "ttml1...",
    IsUnfreezable: true,
})

unfreezeResult, err := wc.UnfreezeToken(ctx, wallet.UnfreezeParams{
    Account: 0,
    TokenID: "ttml1...",
})
```

### Transferring authority

```go
changeResult, err := wc.ChangeTokenAuthority(ctx, wallet.ChangeAuthorityParams{
    Account: 0,
    TokenID: "ttml1...",
    Address: newAuthorityAddr,
})
```

### Sending tokens

```go
sendResult, err := wc.SendToken(ctx, wallet.TokenSendParams{
    Account: 0,
    TokenID: "ttml1...",
    Address: recipientAddr,
    Amount:  wallet.Amount{Atoms: "10000"},
})
```

---

## NFTs

NFTs are non-fungible tokens. Each has unique on-chain metadata.

### Issuing an NFT

```go
ownerAddr, err := wc.NewAddress(ctx, 0)

result, err := wc.IssueNFT(ctx, wallet.IssueNFTParams{
    Account:            0,
    DestinationAddress: ownerAddr,
    Metadata: wallet.NFTMetadata{
        Name:        "My NFT",
        Description: "A unique digital collectible",
        Ticker:      "MYNFT",
        MediaHash:   "sha256hexhash...",
        MediaURI:    stringPtr("https://example.com/media.png"),
        IconURI:     stringPtr("https://example.com/icon.png"),
    },
})
fmt.Printf("nft id: %s\n", result.TokenID)
```

NFTs cannot be minted after issuance: each issuance transaction creates exactly one NFT.

---

## Reading token state from the indexer

```go
import "github.com/mintlayer/go-sdk/indexer"

idxClient := indexer.New("http://127.0.0.1:3000")

// Find by ticker
ids, err := idxClient.FindTokensByTicker(ctx, "MYTOKEN", indexer.PageOpts{Items: 10})

// Full token info
token, err := idxClient.GetToken(ctx, "ttml1...")
fmt.Printf("ticker:    %s\n", token.TokenTicker)
fmt.Printf("supply:    %s\n", token.CirculatingSupply.Decimal)
fmt.Printf("locked:    %v\n", token.IsLocked)
fmt.Printf("frozen:    %v\n", token.Frozen)

// Transaction history
txs, err := idxClient.GetTokenTransactions(ctx, "ttml1...", indexer.PageOpts{Items: 20})

// NFT
nft, err := idxClient.GetNFT(ctx, "nftid1...")
fmt.Printf("owner: %s\n", nft.Owner)
fmt.Printf("name:  %s\n", nft.Metadata.Name)

// Tokens where address is authority
tokenIDs, err := idxClient.GetTokenAuthority(ctx, "mxtc1authority...")
```

---

## Building token transactions manually

Use the `wasm` package for full control over token transactions.

### Issue a token

```go
import mintlayer "github.com/mintlayer/go-sdk/wasm"

c, _ := mintlayer.New(ctx)

tip, _ := idxClient.GetTip(ctx)

issuanceFee, err := c.FungibleTokenIssuanceFee(tip.BlockHeight, mintlayer.Mainnet)

// Predict the token ID
tokenIDStr, err := c.GetTokenId(encodedInputs, tip.BlockHeight, mintlayer.Mainnet)

issueOutput, err := c.EncodeOutputIssueFungibleToken(
    authorityAddr,
    "MYTOKEN",
    "https://example.com/metadata.json",
    2,                         // decimals
    mintlayer.TotalSupplyLockable,
    nil,                       // supplyAmount: only required for TotalSupplyFixed
    mintlayer.FreezableYes,
    tip.BlockHeight,
    mintlayer.Mainnet,
)
```

Include the issuance fee as a separate output or subtract it from an input UTXO.

### Mint tokens

```go
// Get the current nonce from the indexer
tokenInfo, err := idxClient.GetToken(ctx, tokenIDStr)

mintInput, err := c.EncodeInputForMintTokens(
    tokenIDStr,
    mintlayer.NewAmount("100000"),
    tokenInfo.NextNonce,
    mintlayer.Mainnet,
)

mintOutput, err := c.EncodeOutputTokenTransfer(
    mintlayer.NewAmount("100000"),
    recipientAddr,
    tokenIDStr,
    mintlayer.Mainnet,
)
```

### Freeze a token

```go
freezeInput, err := c.EncodeInputForFreezeToken(
    tokenIDStr,
    mintlayer.TokenUnfreezableYes, // can be unfrozen later
    tokenInfo.NextNonce,
    mintlayer.Mainnet,
)
```

---

## Token fees

Protocol fees apply to many token operations. Query them from the WASM client before building transactions:

```go
tip, _ := idxClient.GetTip(ctx)

issuanceFee, _ := c.FungibleTokenIssuanceFee(tip.BlockHeight, mintlayer.Mainnet)
mintFee, _      := c.TokenSupplyChangeFee(tip.BlockHeight, mintlayer.Mainnet)
freezeFee, _    := c.TokenFreezeFee(tip.BlockHeight, mintlayer.Mainnet)
authorityFee, _ := c.TokenChangeAuthorityFee(tip.BlockHeight, mintlayer.Mainnet)
```

These fees must be included as coin inputs in the transaction (or deducted from change).
