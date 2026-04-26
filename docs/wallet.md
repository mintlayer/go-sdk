# Wallet client

The `wallet` package is a JSON-RPC 2.0 client for the Mintlayer wallet daemon (`wallet-rpc-daemon`).
The daemon manages key storage, address derivation, signing, and broadcasting.

```go
import "github.com/mintlayer/go-sdk/wallet"

c := wallet.New("http://127.0.0.1:3034",
    wallet.WithBasicAuth("user", "pass"), // optional
    wallet.WithTimeout(30*time.Second),   // optional
)
```

**Default ports:** 3034 (mainnet), 13034 (testnet).

Errors returned by the daemon are wrapped as `*wallet.RPCError`:

```go
type RPCError struct {
    Code    int
    Message string
}
```

---

## Wallet lifecycle

### `CreateWallet`

```go
func (c *Client) CreateWallet(ctx context.Context, params CreateWalletParams) (*CreateWalletResult, error)
```

Creates a new wallet file. If `Mnemonic` is `nil`, the daemon generates a fresh 24-word BIP-39 phrase.

```go
type CreateWalletParams struct {
    Path            string  `json:"path"`
    StoreSeedPhrase bool    `json:"store_seed_phrase"`
    Mnemonic        *string `json:"mnemonic"`
    Passphrase      *string `json:"passphrase"`
    HardwareWallet  *string `json:"hardware_wallet"`
}

type CreateWalletResult struct {
    Mnemonic *MnemonicResult `json:"mnemonic,omitempty"`
}
```

When `StoreSeedPhrase` is `false`, the mnemonic is returned in `CreateWalletResult.Mnemonic` and not persisted to disk. Store it securely.

### `RecoverWallet`

```go
func (c *Client) RecoverWallet(ctx context.Context, params RecoverWalletParams) error
```

Recreates a wallet from an existing mnemonic. The daemon will rescan the chain to recover the balance.

### `OpenWallet`

```go
func (c *Client) OpenWallet(ctx context.Context, path, password string) error
```

Opens an existing wallet file. Pass an empty string for `password` if the wallet is not encrypted.

### `CloseWallet`

```go
func (c *Client) CloseWallet(ctx context.Context) error
```

Closes the currently open wallet.

### `GetWalletInfo`

```go
func (c *Client) GetWalletInfo(ctx context.Context) (*WalletInfo, error)
```

Returns wallet metadata including account names and hardware wallet type.

```go
type WalletInfo struct {
    WalletID     string          `json:"wallet_id"`
    AccountNames []string        `json:"account_names"`
    ExtraInfo    WalletExtraInfo `json:"extra_info"`
}
```

### `SyncWallet`

```go
func (c *Client) SyncWallet(ctx context.Context) error
```

Syncs the wallet to the current chain tip.

### `RescanWallet`

```go
func (c *Client) RescanWallet(ctx context.Context) error
```

Rescans the entire chain from genesis. Use this after importing a wallet or when balances appear incorrect.

### `BestBlock`

```go
func (c *Client) BestBlock(ctx context.Context) (*BestBlock, error)
```

Returns the block the wallet is currently synced to.

---

## Accounts

### `CreateAccount`

```go
func (c *Client) CreateAccount(ctx context.Context, name string) (*AccountInfo, error)
```

Creates a new BIP-44 account within the wallet. Returns the account index.

### `RenameAccount`

```go
func (c *Client) RenameAccount(ctx context.Context, account uint32, name string) error
```

Renames an existing account.

---

## Balances and addresses

### `GetBalance`

```go
func (c *Client) GetBalance(ctx context.Context, account uint32) (*Balance, error)
```

Returns the coin and token balances for an account.

```go
type Balance struct {
    Coins  Amount            `json:"coins"`
    Tokens map[string]Amount `json:"tokens"` // keyed by token ID
}
```

### `NewAddress`

```go
func (c *Client) NewAddress(ctx context.Context, account uint32) (string, error)
```

Derives a fresh receiving address and marks it as used.

### `ShowReceiveAddresses`

```go
func (c *Client) ShowReceiveAddresses(ctx context.Context, account uint32) ([]AddressWithUsage, error)
```

Lists all receiving addresses that have been derived for an account, with their usage status and coin balance.

```go
type AddressWithUsage struct {
    Address string `json:"address"`
    Used    bool   `json:"used"`
    Coins   Amount `json:"coins"`
}
```

### `RevealPublicKey`

```go
func (c *Client) RevealPublicKey(ctx context.Context, account uint32, address string) (string, error)
```

Returns the hex-encoded public key for an address controlled by the wallet.

---

## Key encryption

### `EncryptPrivateKeys`

```go
func (c *Client) EncryptPrivateKeys(ctx context.Context, password string) error
```

Encrypts the wallet's private keys with a password.

### `UnlockPrivateKeys`

```go
func (c *Client) UnlockPrivateKeys(ctx context.Context, password string) error
```

Unlocks an encrypted wallet for signing. The keys remain unlocked until `LockPrivateKeys` is called or the daemon restarts.

### `LockPrivateKeys`

```go
func (c *Client) LockPrivateKeys(ctx context.Context) error
```

Locks the private keys without closing the wallet.

---

## Transactions

### `AddressSend`

```go
func (c *Client) AddressSend(ctx context.Context, params SendParams) (*SendResult, error)
```

Sends coins to an address. The wallet selects UTXOs, computes fees, and broadcasts.

```go
type SendParams struct {
    Account       uint32     `json:"account"`
    Address       string     `json:"address"`
    Amount        Amount     `json:"amount"`
    SelectedUTXOs []Outpoint `json:"selected_utxos"`
    Options       TxOptions  `json:"options"`
}

type SendResult struct {
    TxID        string        `json:"tx_id"`
    Fees        FeesBreakdown `json:"fees"`
    Broadcasted bool          `json:"broadcasted"`
}
```

Set `Options.BroadcastToMempool` to `false` to build and sign without broadcasting.

### `SweepSpendable`

```go
func (c *Client) SweepSpendable(ctx context.Context, params SweepParams) (*SendResult, error)
```

Sweeps all spendable funds from one or more addresses to a destination. Set `All` to `true` to sweep the entire account; set `FromAddresses` to sweep specific addresses only.

### `SpendUTXO`

```go
func (c *Client) SpendUTXO(ctx context.Context, params UTXOSpendParams) (*SendResult, error)
```

Spends a specific UTXO, optionally providing an HTLC secret.

### `ComposeTransaction`

```go
func (c *Client) ComposeTransaction(ctx context.Context, params ComposeParams) (*ComposedTx, error)
```

Composes an unsigned transaction from explicit inputs and outputs. Returns a hex-encoded `PartiallySignedTransaction`. Use this for advanced flows where you construct outputs manually.

```go
type ComposedTx struct {
    Hex  string        `json:"hex"`
    Fees FeesBreakdown `json:"fees"`
}
```

### `SignRawTransaction`

```go
func (c *Client) SignRawTransaction(ctx context.Context, account uint32, rawTx string) (*SignedTx, error)
```

Signs a hex-encoded transaction using keys from the given account. Used in cold-wallet flows where composition and signing happen separately.

```go
type SignedTx struct {
    Hex               string          `json:"hex"`
    CurrentSignatures json.RawMessage `json:"current_signatures"`
}
```

### `InspectTransaction`

```go
func (c *Client) InspectTransaction(ctx context.Context, txHex string) (*TxInspection, error)
```

Inspects a hex-encoded transaction without broadcasting. Returns input count, signature count, and fees.

### `SubmitTransaction`

```go
func (c *Client) SubmitTransaction(ctx context.Context, txHex string, doNotStore bool) (*SubmitResult, error)
```

Broadcasts a signed transaction. Set `doNotStore` to `true` to broadcast without saving the transaction in the wallet history.

### `ListTransactionsByAddress`

```go
func (c *Client) ListTransactionsByAddress(ctx context.Context, account uint32, address *string, limit uint32) ([]WalletTx, error)
```

Lists confirmed transactions for an account. Pass `nil` for `address` to list across all addresses. The most recent transactions are returned first.

### `ListPendingTransactions`

```go
func (c *Client) ListPendingTransactions(ctx context.Context, account uint32) ([]string, error)
```

Lists transaction IDs that are in the mempool but not yet confirmed.

### `GetTransaction`

```go
func (c *Client) GetTransaction(ctx context.Context, account uint32, txID string) (json.RawMessage, error)
```

Returns a transaction as raw JSON.

### `AbandonTransaction`

```go
func (c *Client) AbandonTransaction(ctx context.Context, account uint32, txID string) error
```

Removes an unconfirmed transaction from the wallet. The transaction will no longer be rebroadcast. The UTXOs it spent are returned to the available balance.

### `DepositData`

```go
func (c *Client) DepositData(ctx context.Context, account uint32, dataHex string) (*SendResult, error)
```

Embeds arbitrary hex-encoded data in a transaction output (`DataDeposit` output type).

---

## Transaction options

Most transaction methods accept a `TxOptions` struct in their params:

```go
type TxOptions struct {
    InTopXMb           *uint32 `json:"in_top_x_mb"`
    BroadcastToMempool *bool   `json:"broadcast_to_mempool"`
}
```

`InTopXMb` controls fee priority. Setting it to `1` targets the top 1 MB of the mempool (highest priority). The default lets the daemon choose.

Setting `BroadcastToMempool` to `false` builds and signs the transaction without broadcasting it. The transaction hex is still returned in the result.

---

## Staking

See [staking.md](staking.md) for a complete guide. Quick reference:

| Method | Description |
|--------|-------------|
| `CreateStakePool` | Create and fund a new staking pool |
| `DecommissionStakePool` | Wind down a pool and recover the pledge |
| `ListOwnedPools` | List pools owned by an account |
| `GetPoolBalance` | Get pool balance |
| `StartStaking` | Start block production |
| `StopStaking` | Stop block production |
| `GetStakingStatus` | Check whether staking is active |
| `CreateDelegation` | Create a delegation to a pool |
| `DelegateStaking` | Send coins into a delegation |
| `WithdrawFromDelegation` | Withdraw from a delegation |
| `ListDelegations` | List delegations owned by an account |

---

## Tokens and NFTs

See [tokens.md](tokens.md) for a complete guide. Quick reference:

| Method | Description |
|--------|-------------|
| `IssueToken` | Issue a new fungible token |
| `IssueNFT` | Issue a new NFT |
| `MintTokens` | Mint additional supply |
| `UnmintTokens` | Remove supply (return to unminted state) |
| `LockTokenSupply` | Permanently lock supply |
| `FreezeToken` | Freeze all transfers |
| `UnfreezeToken` | Unfreeze (if allowed) |
| `ChangeTokenAuthority` | Transfer the authority key |
| `SendToken` | Send tokens to an address |
