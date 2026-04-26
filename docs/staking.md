# Staking and delegations

Mintlayer uses Proof of Stake. ML holders can earn staking rewards by either:

- Running a **staking pool** directly (requires a node, VRF key, and pledge)
- **Delegating** to an existing pool (no node required)

The wallet daemon manages both flows. The indexer provides read-only access to pool and delegation state.

---

## Staking pools

### Creating a pool

```go
import "github.com/mintlayer/go-sdk/wallet"

wc := wallet.New("http://127.0.0.1:3034")

result, err := wc.CreateStakePool(ctx, wallet.CreatePoolParams{
    Account:                0,
    Amount:                 wallet.Amount{Atoms: "40000000000000"}, // minimum pledge
    CostPerBlock:           wallet.Amount{Atoms: "100000000"},      // flat fee per block
    MarginRatioPerThousand: "100",                                  // 10% staker cut
    DecommissionAddress:    decommissionAddr,
    // StakerAddress and VRFPublicKey default to wallet-managed keys when nil
})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("pool id: %s\n  tx id: %s\n", result.TxID, result.TxID)
```

`MarginRatioPerThousand` is the staker's cut of block rewards expressed in thousandths. `"100"` = 10%, `"50"` = 5%, `"1000"` = 100%.

`CostPerBlock` is a flat atom amount deducted from rewards before the margin split. Delegators receive the remainder proportionally to their stake.

### Starting and stopping block production

```go
err = wc.StartStaking(ctx, 0 /*account*/)

status, err := wc.GetStakingStatus(ctx, 0)
fmt.Println(*status) // "Staking" or "NotStaking"

err = wc.StopStaking(ctx, 0)
```

`StopStaking` stops block production but does not decommission the pool. Delegations and staked funds remain untouched.

### Listing owned pools

```go
pools, err := wc.ListOwnedPools(ctx, 0)
for _, p := range pools {
    fmt.Printf("pool %s  pledge=%s  balance=%s\n",
        p.PoolID, p.Pledge.Atoms, p.Balance.Atoms)
}
```

### Decommissioning a pool

```go
result, err := wc.DecommissionStakePool(ctx, wallet.DecommissionParams{
    Account:       0,
    PoolID:        "mpool1...",
    OutputAddress: returnAddr,
})
```

After decommissioning, the pledge is returned to `OutputAddress` after the maturity period. Delegators must withdraw their funds separately.

### Reading pool state from the indexer

```go
import "github.com/mintlayer/go-sdk/indexer"

idxClient := indexer.New("http://127.0.0.1:3000")

// All pools sorted by pledge size
pools, err := idxClient.ListPools(ctx, indexer.PoolListOpts{Sort: "by_pledge"})

// Single pool
pool, err := idxClient.GetPool(ctx, "mpool1...")
fmt.Printf("staker balance: %s\n", pool.StakerBalance.Decimal)
fmt.Printf("delegations:    %s\n", pool.DelegationsBalance.Decimal)

// Blocks produced in the last 24 hours
count, err := idxClient.GetPoolBlockStats(ctx,
    "mpool1...",
    time.Now().Add(-24*time.Hour),
    time.Now(),
)

// All delegations in the pool
delegations, err := idxClient.GetPoolDelegations(ctx, "mpool1...")
```

---

## Delegations

### Creating a delegation

A delegation ID is tied to a pool and an owner address. You create the delegation record first, then fund it separately.

```go
// Step 1: create the delegation
createResult, err := wc.CreateDelegation(ctx, wallet.CreateDelegationParams{
    Account: 0,
    Address: ownerAddr, // address that can withdraw funds
    PoolID:  "mpool1...",
})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("delegation id: %s\n", createResult.DelegationID)

// Step 2: fund the delegation (wait for the creation tx to confirm first)
delegateResult, err := wc.DelegateStaking(ctx, wallet.DelegateParams{
    Account:      0,
    Amount:       wallet.Amount{Atoms: "10000000000000"}, // 100 ML
    DelegationID: createResult.DelegationID,
})
```

You can send multiple `DelegateStaking` transactions to the same delegation to increase your stake.

### Withdrawing from a delegation

```go
result, err := wc.WithdrawFromDelegation(ctx, wallet.WithdrawParams{
    Account:      0,
    Address:      recipientAddr,
    Amount:       wallet.Amount{Atoms: "5000000000000"}, // 50 ML
    DelegationID: "mdelg1...",
})
```

Withdrawn funds arrive at `Address` after the lock period (determined by consensus rules).

### Listing delegations

```go
delegations, err := wc.ListDelegations(ctx, 0)
for _, d := range delegations {
    fmt.Printf("delegation %s  pool=%s  balance=%s\n",
        d.DelegationID, d.PoolID, d.Balance.Atoms)
}
```

### Reading delegation state from the indexer

```go
// All delegations owned by an address
delegationInfos, err := idxClient.GetDelegations(ctx, "mxtc1owner...")

// Single delegation by ID
delegation, err := idxClient.GetDelegation(ctx, "mdelg1...")
fmt.Printf("pool:    %s\n", delegation.PoolID)
fmt.Printf("balance: %s\n", delegation.Balance.Decimal)
fmt.Printf("nonce:   %d\n", delegation.NextNonce)
```

---

## Building delegation transactions manually

The `wasm` package provides the low-level primitives when you need to build delegation transactions without the wallet daemon.

### Create a delegation

```go
import mintlayer "github.com/mintlayer/go-sdk/wasm"

c, _ := mintlayer.New(ctx)
defer c.Close()

// Predict the delegation ID before broadcasting
delegationIDStr, err := c.GetDelegationId(encodedInputs, mintlayer.Mainnet)

// Encode the CreateDelegationId output
createDelegOutput, err := c.EncodeOutputCreateDelegation(
    "mpool1...",
    ownerAddress,
    mintlayer.Mainnet,
)
```

### Fund a delegation

```go
delegateOutput, err := c.EncodeOutputDelegateStaking(
    mintlayer.NewAmount("10000000000000"),
    "mdelg1...",
    mintlayer.Mainnet,
)
```

### Withdraw from a delegation

```go
// The nonce must match the current NextNonce from the indexer
delegation, err := idxClient.GetDelegation(ctx, "mdelg1...")

withdrawInput, err := c.EncodeInputForWithdrawFromDelegation(
    "mdelg1...",
    mintlayer.NewAmount("5000000000000"),
    delegation.NextNonce,
    mintlayer.Mainnet,
)

// The output receives the withdrawn coins after the lock period
withdrawOutput, err := c.EncodeOutputLockThenTransfer(
    mintlayer.NewAmount("5000000000000"),
    recipientAddress,
    lock, // from EncodeLockForBlockCount or similar
    mintlayer.Mainnet,
)
```

---

## Checking rewards

The indexer does not expose a rewards endpoint directly. To calculate staking rewards:

1. Get pool block stats for a time range (`GetPoolBlockStats`)
2. Get the pool's cost per block and margin ratio (`GetPool`)
3. Get your delegation's share of the total pool balance (`GetDelegation`)

The staker receives `CostPerBlock + MarginRatioPerThousand/1000 * (block_reward - CostPerBlock)`. Delegators split the remainder proportionally to their stake.
