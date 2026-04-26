# Indexer client

The `indexer` package is a REST client for the Mintlayer indexer (`api-web-server`).
All paths are relative to `/api/v2/`.

```go
import "github.com/mintlayer/go-sdk/indexer"

c := indexer.New("http://127.0.0.1:3000",
    indexer.WithTimeout(15*time.Second), // optional
)
```

**Default port:** 3000 (mainnet), 13000 (testnet).

Non-2xx HTTP responses are returned as `*indexer.HTTPError`:

```go
type HTTPError struct {
    StatusCode int
    Body       string
}
```

---

## Pagination

List endpoints accept `PageOpts`:

```go
type PageOpts struct {
    Offset uint32 // default: 0
    Items  uint32 // default: 10 (server-side default)
}
```

Pass zero values to use server defaults.

---

## Chain

### `GetTip`

```go
func (c *Client) GetTip(ctx context.Context) (*ChainTip, error)
```

Returns the highest confirmed block.

```go
type ChainTip struct {
    BlockHeight uint64 `json:"block_height"`
    BlockID     string `json:"block_id"`
}
```

### `GetGenesis`

```go
func (c *Client) GetGenesis(ctx context.Context) (*GenesisInfo, error)
```

Returns genesis block information.

### `GetBlockIDAtHeight`

```go
func (c *Client) GetBlockIDAtHeight(ctx context.Context, height uint64) (string, error)
```

Returns the block ID at a given height. Returns a 404 `HTTPError` if no block exists at that height (for example, when querying a height beyond the current tip).

---

## Blocks

### `GetBlock`

```go
func (c *Client) GetBlock(ctx context.Context, id string) (*Block, error)
```

Returns the full block including header, reward outputs, and all transactions.

### `GetBlockHeader`

```go
func (c *Client) GetBlockHeader(ctx context.Context, id string) (*BlockHeader, error)
```

Returns only the block header. Cheaper than `GetBlock` when you do not need transaction data.

### `GetBlockReward`

```go
func (c *Client) GetBlockReward(ctx context.Context, id string) ([]TxOutput, error)
```

Returns the reward outputs of a block as raw JSON messages.

### `GetBlockTransactionIDs`

```go
func (c *Client) GetBlockTransactionIDs(ctx context.Context, id string) ([]string, error)
```

Returns the transaction IDs included in a block. Use this to page through block contents without fetching full transaction data.

---

## Transactions

### `ListTransactions`

```go
func (c *Client) ListTransactions(ctx context.Context, opts PageOpts) ([]Transaction, error)
```

Returns a paginated list of confirmed transactions across the entire chain.

### `GetTransaction`

```go
func (c *Client) GetTransaction(ctx context.Context, id string) (*Transaction, error)
```

Returns a transaction by ID. The `BlockID`, `Timestamp`, and `Confirmations` fields are empty strings for unconfirmed transactions.

```go
type Transaction struct {
    ID            string          `json:"id"`
    Inputs        json.RawMessage `json:"inputs"`
    Outputs       json.RawMessage `json:"outputs"`
    BlockID       string          `json:"block_id"`
    Timestamp     string          `json:"timestamp"`
    Confirmations string          `json:"confirmations"`
}
```

### `GetTransactionMerklePath`

```go
func (c *Client) GetTransactionMerklePath(ctx context.Context, id string) (*MerklePath, error)
```

Returns the Merkle inclusion proof for a transaction. Returns a 404 `HTTPError` if the transaction is not yet in a block.

### `GetTransactionOutput`

```go
func (c *Client) GetTransactionOutput(ctx context.Context, txID string, idx uint32) (json.RawMessage, error)
```

Returns a single output from a transaction as raw JSON. The shape is determined by the `"type"` field. Common types: `"Transfer"`, `"LockThenTransfer"`, `"Burn"`, `"CreateStakePool"`, `"CreateDelegationId"`, `"DelegateStaking"`, `"IssueFungibleToken"`, `"IssueNft"`, `"DataDeposit"`, `"Htlc"`, `"CreateOrder"`.

### `SubmitTransaction`

```go
func (c *Client) SubmitTransaction(ctx context.Context, signedTxHex string) (string, error)
```

Submits a hex-encoded signed transaction to the network. Returns the transaction ID on success.

**Requires** the indexer to be started with `--enable-post-routes`.

---

## Addresses

### `GetAddressInfo`

```go
func (c *Client) GetAddressInfo(ctx context.Context, address string) (*AddressInfo, error)
```

Returns balance and transaction history for a bech32m address. Returns a 404 `HTTPError` if the address has no on-chain history.

```go
type AddressInfo struct {
    CoinBalance        Amount         `json:"coin_balance"`
    LockedCoinBalance  Amount         `json:"locked_coin_balance"`
    TransactionHistory []string       `json:"transaction_history"`
    Tokens             []TokenBalance `json:"tokens"`
}
```

### `GetSpendableUTXOs`

```go
func (c *Client) GetSpendableUTXOs(ctx context.Context, address string) ([]UTXO, error)
```

Returns confirmed, unspent UTXOs that can be spent immediately.

### `GetAllUTXOs`

```go
func (c *Client) GetAllUTXOs(ctx context.Context, address string) ([]UTXO, error)
```

Returns all UTXOs including those that are locked or otherwise unspendable.

### `GetDelegations`

```go
func (c *Client) GetDelegations(ctx context.Context, address string) ([]DelegationInfo, error)
```

Returns all staking delegations owned by an address.

```go
type DelegationInfo struct {
    DelegationID     string `json:"delegation_id"`
    PoolID           string `json:"pool_id"`
    NextNonce        uint64 `json:"next_nonce"`
    SpendDestination string `json:"spend_destination"`
    Balance          Amount `json:"balance"`
}
```

### `GetTokenAuthority`

```go
func (c *Client) GetTokenAuthority(ctx context.Context, address string) ([]string, error)
```

Returns the IDs (bech32m) of fungible tokens for which the address holds authority (can mint, freeze, etc.).

---

## Pools and delegations

### `ListPools`

```go
func (c *Client) ListPools(ctx context.Context, opts PoolListOpts) ([]Pool, error)
```

Returns staking pools with optional pagination. The `Sort` field accepts:

- `"by_height"` (default): newest pools first
- `"by_pledge"`: largest staker balance first

```go
type PoolListOpts struct {
    PageOpts
    Sort string
}
```

### `GetPool`

```go
func (c *Client) GetPool(ctx context.Context, id string) (*Pool, error)
```

Returns a single staking pool by its bech32m pool ID.

```go
type Pool struct {
    PoolID                  string `json:"pool_id"`
    DecommissionDestination string `json:"decommission_destination"`
    StakerBalance           Amount `json:"staker_balance"`
    MarginRatioPerThousand  uint32 `json:"margin_ratio_per_thousand"`
    CostPerBlock            Amount `json:"cost_per_block"`
    VRFPublicKey            string `json:"vrf_public_key"`
    DelegationsBalance      Amount `json:"delegations_balance"`
}
```

### `GetPoolBlockStats`

```go
func (c *Client) GetPoolBlockStats(ctx context.Context, id string, from, to time.Time) (uint64, error)
```

Returns the number of blocks produced by a pool in the half-open interval `[from, to)`.

```go
from := time.Now().Add(-24 * time.Hour)
to   := time.Now()
count, err := c.GetPoolBlockStats(ctx, "mpool1...", from, to)
```

### `GetDelegation`

```go
func (c *Client) GetDelegation(ctx context.Context, id string) (*Delegation, error)
```

Returns a single delegation by its bech32m delegation ID.

```go
type Delegation struct {
    DelegationID        string `json:"delegation_id"`
    PoolID              string `json:"pool_id"`
    NextNonce           uint64 `json:"next_nonce"`
    SpendDestination    string `json:"spend_destination"`
    Balance             Amount `json:"balance"`
    CreationBlockHeight uint64 `json:"creation_block_height"`
}
```

### `GetPoolDelegations`

```go
func (c *Client) GetPoolDelegations(ctx context.Context, id string) ([]PoolDelegation, error)
```

Returns all delegations in a pool. Each entry includes the `CreationBlockHeight` in addition to the standard delegation fields.

---

## Tokens and NFTs

### `ListTokens`

```go
func (c *Client) ListTokens(ctx context.Context, opts PageOpts) ([]string, error)
```

Returns a paginated list of fungible token IDs (bech32m).

### `GetToken`

```go
func (c *Client) GetToken(ctx context.Context, id string) (*TokenInfo, error)
```

Returns full information about a fungible token.

```go
type TokenInfo struct {
    Authority         string          `json:"authority"`
    IsLocked          bool            `json:"is_locked"`
    CirculatingSupply Amount          `json:"circulating_supply"`
    TokenTicker       string          `json:"token_ticker"`
    MetadataURI       string          `json:"metadata_uri"`
    NumberOfDecimals  uint8           `json:"number_of_decimals"`
    TotalSupply       json.RawMessage `json:"total_supply"`
    Frozen            bool            `json:"frozen"`
    IsTokenUnfreezable *bool          `json:"is_token_unfreezable"` // non-nil only when Frozen is true
    IsTokenFreezable   *bool          `json:"is_token_freezable"`  // non-nil only when Frozen is false
    NextNonce         uint64          `json:"next_nonce"`
}
```

### `GetTokenTransactions`

```go
func (c *Client) GetTokenTransactions(ctx context.Context, id string, opts PageOpts) ([]TokenTx, error)
```

Returns the transaction history for a token (issuance, mints, transfers, burns).

### `FindTokensByTicker`

```go
func (c *Client) FindTokensByTicker(ctx context.Context, ticker string, opts PageOpts) ([]string, error)
```

Returns token IDs whose ticker matches the given string. Tickers are not unique, so this may return multiple results.

### `GetNFT`

```go
func (c *Client) GetNFT(ctx context.Context, id string) (*NFTInfo, error)
```

Returns information about an NFT.

---

## Orders

### `ListOrders`

```go
func (c *Client) ListOrders(ctx context.Context, opts PageOpts) ([]Order, error)
```

Returns active orders.

### `GetOrder`

```go
func (c *Client) GetOrder(ctx context.Context, id string) (*Order, error)
```

Returns a single order by its bech32m order ID.

```go
type Order struct {
    OrderID             string          `json:"order_id"`
    ConcludeDestination string          `json:"conclude_destination"`
    GiveCurrency        json.RawMessage `json:"give_currency"`
    InitiallyGiven      Amount          `json:"initially_given"`
    GiveBalance         Amount          `json:"give_balance"`
    AskCurrency         json.RawMessage `json:"ask_currency"`
    InitiallyAsked      Amount          `json:"initially_asked"`
    AskBalance          Amount          `json:"ask_balance"`
    Nonce               uint64          `json:"nonce"`
}
```

`GiveCurrency` and `AskCurrency` are raw JSON objects with a `"type"` field of `"Coin"` or `"Token"`.

### `ListOrdersByPair`

```go
func (c *Client) ListOrdersByPair(ctx context.Context, askCurrency, giveCurrency string, opts PageOpts) ([]Order, error)
```

Returns orders filtered by a trading pair. Pass `"Coin"` or a token ID (bech32m) for each currency.

---

## Statistics

### `GetCoinStatistics`

```go
func (c *Client) GetCoinStatistics(ctx context.Context) (*CoinStats, error)
```

Returns supply statistics for the native ML coin.

```go
type CoinStats struct {
    CirculatingSupply Amount `json:"circulating_supply"`
    Preminted         Amount `json:"preminted"`
    Burned            Amount `json:"burned"`
    Staked            Amount `json:"staked"`
}
```

### `GetTokenStatistics`

```go
func (c *Client) GetTokenStatistics(ctx context.Context, tokenID string) (*CoinStats, error)
```

Returns supply statistics for a fungible token.

### `GetFeeRate`

```go
func (c *Client) GetFeeRate(ctx context.Context, inTopXMb uint32) (string, error)
```

Returns the current fee rate in atoms per kilobyte needed to place a transaction in the top `inTopXMb` megabytes of the mempool priority queue.

---

## Amounts

The `Amount` type carries both raw atoms and a human-readable decimal:

```go
type Amount struct {
    Atoms   string `json:"atoms"`
    Decimal string `json:"decimal"`
}
```

All values populated by the server include both fields. When constructing amounts to send to the server (for example in `AddressSend`), you only need to set `Atoms`.
