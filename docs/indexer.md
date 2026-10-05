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

The indexer exposes two pagination styles. Offset-based listings are the simple alternative for shallow listings; cursor pagination (added in api-server 1.4.1) backs large listings that must be walked in full.

### Offset pagination (PageOpts)

```go
type PageOpts struct {
    Offset uint32 // default: 0
    Items  uint32 // default: 10 (server-side default)
}
```

Pass zero values to use server defaults.

### Cursor pagination (ListOption)

Pools (`/v2/pool`), the global transaction listing (`/v2/transaction`), the coin/token holders, and the order book answer cursor requests with an envelope:

```go
type CursorPage[T any] struct {
    Items      []T     `json:"items"`
    NextCursor *string `json:"next_cursor"` // nil when the listing is exhausted
}
```

One-page methods (`ListPoolsPage`, `ListTransactionsPage`, `ListCoinHolders`, `ListTokenHolders`, `GetOrderBook`) take variadic `ListOption` values and return the envelope (the order book returns its own `OrderBookPage` with the same cursor rules). Pager constructors (`PoolsPager`, `TransactionsPager`, `CoinHoldersPager`, `TokenHoldersPager`, `OrderBookPager`) walk the listing for you, following the server cursor automatically and stopping on a `nil` one:

```go
pager := indexer.CoinHoldersPager(c)
err := pager.Walk(ctx, func(h indexer.Holder) bool {
    fmt.Println(h.Address, h.Amount.Decimal)
    return true // return false to stop early
})
```

Options:

| Option | Endpoints | Meaning |
|---|---|---|
| `WithItems(n)` | all | page size, accepted range 1..100 (`MaxNumItems`). The server rejects `items=0` with 400 `Invalid number of items` on every paginated endpoint, offset-based included; the SDK validates client-side. |
| `WithCursor(cur)` | all cursor endpoints | resume from a server-issued cursor. Cursors are opaque: never construct, decode, or modify one — a fabricated or cross-endpoint cursor is rejected with 400 `Invalid cursor`. |
| `WithOffset(n)` | pools, transactions | offset of an offset-based listing. On the holders/order-book endpoints the cursor defines the page position server-side and the offset is ignored (`items` still applies); the SDK sends both parameters as-is. |
| `WithSort(s)` | pools | `"by_height"` (default) or `"by_pledge"`; any other value fails client-side. Only the default creation-height sort supports cursors: on `ListPoolsPage` any other sort combined with a cursor is rejected by the server with 400 `Bad request` and propagated as `*HTTPError`; `PoolsPager` rejects the same combination client-side with a `*RequestError`. |
| `WithOffsetMode(m)` | transactions | `"legacy"` (default) or `"absolute"`; selects the offset-based listing, which has no cursors. Combining a cursor with `offset_mode` is rejected client-side with a `*RequestError`. |
| `WithSide(s)` | order book | `"ask"` or `"bid"` — required for the book. Cursors are side-specific (`book-ask` / `book-bid`): an ask cursor on a bid walk returns 400 `Invalid cursor`. |

Invalid combinations and values fail client-side with `*indexer.RequestError` before any request is sent. Server-side rejections (400/404) are returned as `*indexer.HTTPError` with `Kind` and sentinel matching — see [Errors](#errors).

Semantics:

- **Page stability** is only guaranteed once the indexer's scanner is fully caught up. A walk performed during catch-up or a reorg may skip or repeat an entry.
- Cursors may only be taken from `CursorPage.NextCursor` / `Pager.NextCursor`. `Pager.NextCursor()` is nil until the first page is fetched and after the walk ends. If `Walk` stops early mid-page, the current page still holds unserved items: persisting the cursor then and resuming in a fresh pager skips them, so only persist it once the current page has been consumed fully (the same pager serves the remainder first).
- `Walk` and `NextPage` share one consumption position and may be mixed on the same pager: `NextPage` returns the items neither API has consumed yet (remainder of the current page first, then a fresh fetch), and `Walk` resumes wherever the previous calls stopped, never serving an item twice.
- `Pager.NextPage(ctx)` returns `(nil, nil)` once the listing is exhausted; a fetched page is never nil, so `page == nil` uniquely means the walk is finished. Items left unserved by a partial `Walk` are returned by the next `NextPage` before a new page is fetched. Errors are retryable (a failed page does not advance the walk).
- The cursor pagers validate their options at construction time; inapplicable options surface as a deferred `*indexer.RequestError` on the first `NextPage` (named after the pager, e.g. `TransactionsPager` rejects `WithOffsetMode`).

Pair formatting (`{base}_{quote}`): the native coin ticker matches case-insensitively, while token IDs are exact, case-sensitive bech32. The SDK passes the pair through unchanged — never lowercase it.

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

Returns a paginated list of confirmed transactions across the entire chain, ordered newest block first, transactions in block order within each block.

### `ListTransactionsPage`

```go
func (c *Client) ListTransactionsPage(ctx context.Context, opts ...ListOption) (*CursorPage[Transaction], error)
```

One page of the global transaction listing as a cursor envelope; pass `NextCursor` to `WithCursor` (or use `TransactionsPager`) to continue the walk. `WithOffsetMode("legacy"|"absolute")` selects the offset-based listing instead, which has no cursors (such a page arrives with a nil `NextCursor`). Combining a cursor with `offset_mode` is rejected client-side with a `*RequestError`. In the default (cursor) mode the cursor parameter is always present (empty on a first page), so the server silently overrides the `WithOffset` page position with the cursor (`items` still applies) — use `WithOffsetMode` for offset-based pages. Per-block transaction listings remain offset-based (see `GetBlockTransactionIDs`).

### `TransactionsPager`

```go
func TransactionsPager(c *Client, opts ...ListOption) *Pager[Transaction]
```

Walks the global transaction listing (newest block first, transactions in block order within each block). The pager does not accept `WithOffsetMode` (a cursor walk cannot be combined with an offset listing — rejected at construction): use `ListTransactionsPage` with `WithOffsetMode` for the offset-based listing.

```go
pager := indexer.TransactionsPager(c, indexer.WithItems(100))
for {
    page, err := pager.NextPage(ctx)
    if err != nil {
        return err
    }
    if page == nil {
        break
    }
    for _, tx := range page {
        fmt.Println(tx.ID, tx.BlockID)
    }
}
```

### `GetTransaction`

```go
func (c *Client) GetTransaction(ctx context.Context, id string) (*Transaction, error)
```

Returns a transaction by ID. `BlockID` is the hash of the confirming block; the `BlockID`, `Timestamp`, and `Confirmations` fields are empty strings for unconfirmed (mempool) transactions.

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

### `ListPoolsPage`

```go
func (c *Client) ListPoolsPage(ctx context.Context, opts ...ListOption) (*CursorPage[Pool], error)
```

One page of staking pools as a cursor envelope (newest creation height first by default); pass `NextCursor` to `WithCursor` (or use `PoolsPager`) to continue the walk. The default creation-height sort is the only cursor-compatible one: `WithSort("by_pledge")` together with a cursor is rejected by the server with 400 `Bad request` and propagated as `*HTTPError` (only `PoolsPager` validates it client-side), and without a cursor a non-default sort takes the offset path (no cursors, single page).

### `PoolsPager`

```go
func PoolsPager(c *Client, opts ...ListOption) *Pager[Pool]
```

Walks the pools listing, following the server cursor automatically. The pager only accepts the default creation-height sort (`WithSort` with any other value is rejected at construction); use `ListPoolsPage` for `"by_pledge"` listings. When `WithOffset` is set without a cursor, the cursor silently overrides the offset page position server-side (`items` still applies).

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

### `GetOrderBook`

```go
func (c *Client) GetOrderBook(ctx context.Context, pair string, opts ...ListOption) (*OrderBookPage, error)
```

One aggregated page of the order book for the trading pair `"{base}_{quote}"` (e.g. `"ML_tmltken1..."`). The side is required — pass `WithSide(indexer.SideAsk)` or `WithSide(indexer.SideBid)`. Levels come best-price-first (ascending asks, descending bids).

```go
type OrderBookPage struct {
    Levels     []OrderBookLevel `json:"items"`
    Truncated  bool             `json:"truncated,omitempty"`
    NextCursor *string          `json:"next_cursor"`
}

type OrderBookLevel struct {
    Price  OrderBookPrice `json:"price"`
    Amount Amount         `json:"amount"`
}

type OrderBookPrice struct {
    Atoms   string `json:"atoms"`   // exact price: reduced rational "numer/denom" (quote atoms per base atom)
    Decimal string `json:"decimal"` // floored-toward-zero rendering at the quote currency's decimals
}
```

`Amount` on a level is the total remaining base-currency amount across all orders at that price. A nil `NextCursor` means the end of the book — unless `Truncated` is true: the server then hit its per-request aggregation cap (`OrderBookMaxOrders` = 10000 live orders scanned), the levels are an incomplete aggregation, and **no continuation cursor exists** (`NextCursor` is always nil when `Truncated`). The pair is passed through exactly as given: the native coin ticker matches case-insensitively, while token IDs are exact, case-sensitive bech32 — never lowercase user input.

Cursors are side-specific (`book-ask` / `book-bid`): reusing an ask cursor on a bid walk returns 400 `Invalid cursor`.

### `OrderBookPager`

```go
func OrderBookPager(c *Client, pair string, side string, opts ...ListOption) *Pager[OrderBookLevel]
```

Walks the order book of `pair` on `side`, following the server cursor automatically and stopping at the end of the book — or as soon as a page reports truncation (`Pager.Truncated()`; a truncated page has no continuation cursor by construction).

See the [examples/indexer-paging](../examples/indexer-paging/main.go) example for a complete both-side walk.

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

All four counters are always present (api-server 1.4.1+): a counter that has never been written renders as zero, so clients may read the fields directly without nil/emptiness checks.

### `ListCoinHolders`

```go
func (c *Client) ListCoinHolders(ctx context.Context, opts ...ListOption) (*CursorPage[Holder], error)
```

One page of the native coin holders, largest balance first. The server ignores the offset on this endpoint; the cursor defines the page position. See `CoinHoldersPager` for full walks.

### `ListTokenHolders`

```go
func (c *Client) ListTokenHolders(ctx context.Context, tokenID string, opts ...ListOption) (*CursorPage[Holder], error)
```

One page of the holders of the given token (bech32, case-sensitive — pass it exactly as issued). Unknown token IDs fail with `*indexer.HTTPError` matching `indexer.ErrTokenNotFound`.

```go
type Holder struct {
    Address string `json:"address"`
    Amount  Amount `json:"amount"`
}
```

`Amount.Decimal` is rendered with the decimals of the listed currency: 9 for the native coin, the token's `number_of_decimals` for tokens.

### `CoinHoldersPager` / `TokenHoldersPager`

```go
func CoinHoldersPager(c *Client, opts ...ListOption) *Pager[Holder]
func TokenHoldersPager(c *Client, tokenID string, opts ...ListOption) *Pager[Holder]
```

Walk the holders listings, following the server cursor automatically.

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

---

## Errors

Non-2xx responses are returned as `*indexer.HTTPError`:

```go
type HTTPError struct {
    StatusCode int       // HTTP status code (400, 404, ...)
    Body       string    // raw response body
    Message    string    // the server's {"error": "..."} field
    Kind       ErrorKind // classified server error, ErrorKindOther when unrecognised
}
```

Every well-known api-server error message maps to an `ErrorKind` value with a matching sentinel for `errors.Is` (`ErrorKindOther` is the only kind without one). Matching is gated on the documented status code (400 for the client errors, 404 for the token lookup): the same message arriving under an unexpected status degrades to `ErrorKindOther` and does not match the sentinel.

| Server response | Kind | Sentinel |
|---|---|---|
| 400 `Invalid cursor` | `ErrorKindInvalidCursor` | `indexer.ErrInvalidCursor` |
| 400 `Invalid number of items` | `ErrorKindInvalidNumItems` | `indexer.ErrInvalidNumItems` |
| 404 `Token not found` | `ErrorKindTokenNotFound` | `indexer.ErrTokenNotFound` |
| 400 `Bad request` | `ErrorKindBadRequest` | `indexer.ErrBadRequest` |
| 400 `Invalid offset mode` | `ErrorKindInvalidOffsetMode` | `indexer.ErrInvalidOffsetMode` |
| 400 `Invalid pools sort order` | `ErrorKindInvalidPoolsSortOrder` | `indexer.ErrInvalidPoolsSortOrder` |
| 400 `Invalid token Id` | `ErrorKindInvalidTokenID` | `indexer.ErrInvalidTokenID` |
| 400 `Invalid order trading pair` | `ErrorKindInvalidOrderPair` | `indexer.ErrInvalidOrderPair` |

```go
holders, err := c.ListTokenHolders(ctx, tokenID)
if errors.Is(err, indexer.ErrTokenNotFound) {
    // unknown token id
}
```

Invalid options or combinations rejected client-side (before any request) are returned as `*indexer.RequestError`:

```go
type RequestError struct {
    Option string // e.g. "WithItems", "WithCursor", "WithSide"
    Reason string // human-readable explanation
}
```
