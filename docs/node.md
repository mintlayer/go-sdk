# Node client

The `node` package is a JSON-RPC 2.0 client for the Mintlayer node daemon.

```go
import "github.com/mintlayer/go-sdk/node"

c := node.New("http://127.0.0.1:3030",
    node.WithBasicAuth("user", "pass"), // optional
    node.WithTimeout(10*time.Second),   // optional
)
```

**Default ports:** 3030 (mainnet), 13030 (testnet).

Errors returned by the daemon are wrapped as `*node.RPCError`:

```go
type RPCError struct {
    Code    int
    Message string
}
```

---

## Chain state

### `ChainstateInfo`

```go
func (c *Client) ChainstateInfo(ctx context.Context) (*ChainstateInfo, error)
```

Returns a summary of the current chain state.

```go
type ChainstateInfo struct {
    BestBlockHeight    uint64    `json:"best_block_height"`
    BestBlockID        string    `json:"best_block_id"`
    BestBlockTimestamp Timestamp `json:"best_block_timestamp"`
    MedianTime         Timestamp `json:"median_time"`
    IsInitialBlockDownload bool  `json:"is_initial_block_download"`
}
```

### `BestBlockID`

```go
func (c *Client) BestBlockID(ctx context.Context) (string, error)
```

Returns the hex block ID of the current tip.

### `BestBlockHeight`

```go
func (c *Client) BestBlockHeight(ctx context.Context) (uint64, error)
```

Returns the height of the current tip.

### `BlockIDAtHeight`

```go
func (c *Client) BlockIDAtHeight(ctx context.Context, height uint64) (*string, error)
```

Returns the block ID at a given height, or `nil` if no block exists at that height.

### `BlockHeightInMainChain`

```go
func (c *Client) BlockHeightInMainChain(ctx context.Context, blockID string) (*uint64, error)
```

Returns the mainchain height for a block ID, or `nil` if the block is not on the main chain.

### `GetBlock`

```go
func (c *Client) GetBlock(ctx context.Context, id string) (*string, error)
```

Returns the hex-encoded raw block.

### `GetBlockJSON`

```go
func (c *Client) GetBlockJSON(ctx context.Context, id string) (json.RawMessage, error)
```

Returns the block as a JSON object. Useful for inspection without custom deserialization.

### `GetMainchainBlocks`

```go
func (c *Client) GetMainchainBlocks(ctx context.Context, from uint64, maxCount uint32) ([]string, error)
```

Returns up to `maxCount` hex-encoded blocks starting at height `from`.

### `GetUTXO`

```go
func (c *Client) GetUTXO(ctx context.Context, outpoint Outpoint) (json.RawMessage, error)
```

Returns the output at a given outpoint as raw JSON.

```go
type Outpoint struct {
    SourceID OutpointSourceID `json:"source_id"`
    Index    uint32           `json:"index"`
}

type OutpointSourceID struct {
    Type    string          `json:"type"`    // "Transaction" or "BlockReward"
    Content json.RawMessage `json:"content"` // {"tx_id":"hex"} or {"block_id":"hex"}
}
```

### `SubmitBlock`

```go
func (c *Client) SubmitBlock(ctx context.Context, blockHex string) error
```

Submits a hex-encoded block. Used by block producers.

---

## Pool and delegation queries

### `StakePoolBalance`

```go
func (c *Client) StakePoolBalance(ctx context.Context, poolAddress string) (*Amount, error)
```

Returns the total balance of a pool (staker pledge plus all delegations). Returns `nil` if the pool is not found.

### `StakerBalance`

```go
func (c *Client) StakerBalance(ctx context.Context, poolAddress string) (*Amount, error)
```

Returns the staker's own balance, excluding delegations. Returns `nil` if the pool is not found.

### `PoolDecommissionDestination`

```go
func (c *Client) PoolDecommissionDestination(ctx context.Context, poolAddress string) (*string, error)
```

Returns the address that receives funds when the pool is decommissioned.

### `DelegationShare`

```go
func (c *Client) DelegationShare(ctx context.Context, poolAddress, delegationAddress string) (*Amount, error)
```

Returns the amount owned by a specific delegation in a pool.

---

## Token and order info

### `TokenInfo`

```go
func (c *Client) TokenInfo(ctx context.Context, tokenID string) (*TokenInfo, error)
```

Returns on-chain token metadata.

```go
type TokenInfo struct {
    Type    string          `json:"type"`
    Content json.RawMessage `json:"content"`
}
```

### `TokensInfo`

```go
func (c *Client) TokensInfo(ctx context.Context, tokenIDs []string) ([]TokenInfo, error)
```

Batch version of `TokenInfo`. More efficient than calling `TokenInfo` in a loop.

### `OrderInfo`

```go
func (c *Client) OrderInfo(ctx context.Context, orderID string) (*OrderInfo, error)
```

Returns the current state of an order.

```go
type OrderInfo struct {
    ConcludeKey   string  `json:"conclude_key"`
    InitiallyAsked Amount `json:"initially_asked"`
    InitiallyGiven Amount `json:"initially_given"`
    AskBalance    Amount  `json:"ask_balance"`
    GiveBalance   Amount  `json:"give_balance"`
    Nonce         uint64  `json:"nonce"`
    IsFrozen      bool    `json:"is_frozen"`
}
```

### `OrdersInfoByCurrencies`

```go
func (c *Client) OrdersInfoByCurrencies(ctx context.Context, ask, give *Currency) (map[string]OrderInfo, error)
```

Returns all orders matching the given currency pair. Returns a map from order ID to `OrderInfo`. Pass `nil` for either currency to match any.

```go
type Currency struct {
    Type    string          `json:"type"`    // "Coin" or "Token"
    Content json.RawMessage `json:"content"` // {"token_id":"..."} when Type is "Token"
}
```

---

## Mempool

### `ContainsTx`

```go
func (c *Client) ContainsTx(ctx context.Context, txID string) (bool, error)
```

Returns `true` if the mempool contains the transaction.

### `ContainsOrphanTx`

```go
func (c *Client) ContainsOrphanTx(ctx context.Context, txID string) (bool, error)
```

Returns `true` if the orphan pool contains the transaction.

### `GetTransaction`

```go
func (c *Client) GetTransaction(ctx context.Context, txID string) (*MempoolTx, error)
```

Returns a mempool transaction.

```go
type MempoolTx struct {
    ID          string          `json:"id"`
    Status      string          `json:"status"`
    Transaction json.RawMessage `json:"transaction"`
}
```

### `MempoolSubmitTransaction`

```go
func (c *Client) MempoolSubmitTransaction(ctx context.Context, txHex string, trustPolicy TrustPolicy) error
```

Submits a transaction to the local mempool only, without broadcasting to peers. Use `TrustPolicyTrusted` for transactions you constructed yourself; use `TrustPolicyUntrusted` for externally sourced transactions.

### `GetFeeRate`

```go
func (c *Client) GetFeeRate(ctx context.Context, inTopXMb uint32) (*FeeRate, error)
```

Returns the fee rate needed to land in the top `inTopXMb` megabytes of the mempool.

```go
type FeeRate struct {
    AmountPerKB Amount `json:"amount_per_kb"`
}
```

### `GetFeeRatePoints`

```go
func (c *Client) GetFeeRatePoints(ctx context.Context) ([]FeeRatePoint, error)
```

Returns the mempool fee-rate curve as a slice of (size, rate) pairs.

### `MemoryUsage`

```go
func (c *Client) MemoryUsage(ctx context.Context) (uint64, error)
```

Returns the current mempool memory usage in bytes.

---

## P2P

### `GetPeerCount`

```go
func (c *Client) GetPeerCount(ctx context.Context) (uint64, error)
```

Returns the number of currently connected peers.

### `GetConnectedPeers`

```go
func (c *Client) GetConnectedPeers(ctx context.Context) ([]PeerInfo, error)
```

Returns details about all connected peers.

```go
type PeerInfo struct {
    PeerID          uint64  `json:"peer_id"`
    Address         string  `json:"address"`
    PeerRole        string  `json:"peer_role"`
    BanScore        uint32  `json:"ban_score"`
    UserAgent       string  `json:"user_agent"`
    SoftwareVersion string  `json:"software_version"`
    PingWait        *uint64 `json:"ping_wait"`
    PingLast        *uint64 `json:"ping_last"`
    PingMin         *uint64 `json:"ping_min"`
    LastTipBlockTime *int64 `json:"last_tip_block_time"`
}
```

### `GetBindAddresses`

```go
func (c *Client) GetBindAddresses(ctx context.Context) ([]string, error)
```

Returns the addresses the node is listening on for P2P connections.

### `AddReservedNode`

```go
func (c *Client) AddReservedNode(ctx context.Context, addr string) error
```

Adds a persistent peer that the node always attempts to reconnect to.

### `RemoveReservedNode`

```go
func (c *Client) RemoveReservedNode(ctx context.Context, addr string) error
```

Removes a persistent peer.

### `Connect`

```go
func (c *Client) Connect(ctx context.Context, addr string) error
```

Makes a one-time connection attempt to a peer address.

### `Disconnect`

```go
func (c *Client) Disconnect(ctx context.Context, peerID uint64) error
```

Closes the connection to a peer by ID.

### `ListBanned`

```go
func (c *Client) ListBanned(ctx context.Context) ([]BannedPeer, error)
```

Returns the list of banned peers.

```go
type BannedPeer struct {
    Address string    `json:"address"`
    BanTime Timestamp `json:"ban_time"`
}
```

### `Ban`

```go
func (c *Client) Ban(ctx context.Context, address string, duration time.Duration) error
```

Bans a peer for the specified duration.

### `Unban`

```go
func (c *Client) Unban(ctx context.Context, address string) error
```

Removes a peer from the ban list.

### `P2PSubmitTransaction`

```go
func (c *Client) P2PSubmitTransaction(ctx context.Context, txHex string, trustPolicy TrustPolicy) error
```

Submits a transaction to the mempool and broadcasts it to peers. This is the normal path for publishing a transaction to the network.

---

## Node management

### `NodeVersion`

```go
func (c *Client) NodeVersion(ctx context.Context) (string, error)
```

Returns the node software version string.

### `NodeShutdown`

```go
func (c *Client) NodeShutdown(ctx context.Context) error
```

Initiates a graceful node shutdown.
