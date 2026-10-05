# Changelog

All notable changes to the Mintlayer Go SDK are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added

#### Indexer client (`github.com/mintlayer/go-sdk/indexer`)

Cursor pagination for the api-server v2 endpoints introduced in mintlayer-core PR #2130 (api-server 1.4.1):

- **Pools**: `ListPoolsPage` and `PoolsPager` — cursor envelope over `/v2/pool`; cursors work only with the default `by_height` sort (any other sort with a cursor is a server 400 `Bad request`).
- **Transactions**: `ListTransactionsPage` and `TransactionsPager` — cursor envelope over the global listing (newest block first, transactions in block order within each block); `WithOffsetMode("legacy"|"absolute")` selects the offset-based listing instead, and cursor + `offset_mode` is rejected client-side with a `*RequestError`. `Transaction.BlockID` is documented as the confirming block's hash (empty for mempool transactions).
- **Holders**: `ListCoinHolders`, `ListTokenHolders`, `CoinHoldersPager`, `TokenHoldersPager`, and the `Holder` type (address + amount; token ids are exact case-sensitive bech32, unknown ids are 404 `Token not found`).
- **Order book**: `GetOrderBook`, `OrderBookPager`, and the `OrderBookPage`/`OrderBookLevel`/`OrderBookPrice` types — aggregated price levels (exact reduced-rational price plus floored decimal rendering) per side; required `WithSide`, side-specific cursors, and the truncation invariant (`Truncated` true ⇒ `NextCursor` nil at the server's `OrderBookMaxOrders` = 10000-order cap; the paginator stops there).
- **Shared paginator**: generic `Pager[T]` (`NewPager`, `NextPage`, `Walk`, `NextCursor`, `Truncated`) and the `CursorPage[T]` `{items, next_cursor}` envelope; pre-1.4.1 bare-array responses are accepted and wrapped, a server-issued empty cursor is treated as end-of-listing, and `NextPage` never loses the unserved remainder of a partially consumed page.
- **List options**: `WithItems` (validated 1..=100 = `MaxNumItems`; the server rejects `items=0` on every paginated endpoint), `WithOffset`, `WithCursor` (opaque, non-empty; never constructed client-side), `WithSort`, `WithOffsetMode`, `WithSide`; inapplicable options fail client-side with `*RequestError` — the single-page methods at call time, the cursor pagers at construction time (`PoolsPager` rejects non-default sorts, `TransactionsPager` rejects `WithOffsetMode`).
- **Typed errors**: `HTTPError.Message`/`HTTPError.Kind` populated from the server's `{"error": ...}` body, `ErrorKind` classification, and the `ErrInvalidCursor` / `ErrInvalidNumItems` / `ErrTokenNotFound` sentinels matched via `errors.Is`.

Documented semantics: page stability is only guaranteed once the indexer's scanner is fully caught up (walks during catch-up/reorgs may skip or repeat entries); offset-based listings remain unchanged as the simple alternative for shallow listings; on pools/holders/order book a cursor silently overrides the offset page position server-side.

Docs and examples: README indexer section, `docs/indexer.md` (cursor pagination guide, holders/order book reference, error table), and the runnable `examples/indexer-paging`.

---

## [0.1.0] — 2026-04-16

Initial public release of `github.com/mintlayer/go-sdk`.

### Added

#### Top-level client (`github.com/mintlayer/go-sdk`)
- `Config` struct with `NodeURL`, `IndexerURL`, `WalletURL`, `Username`, `Password`.
- `New(cfg Config) *Client` — constructs only the sub-clients whose URL is non-empty.
- `InitWASM(ctx) error` — opt-in WASM initialisation (~400 ms).
- `Close() error` — releases WASM resources.
- Convenience type re-exports: `Amount`, `Network`, `Mainnet`/`Testnet`/`Regtest`/`Signet`, `NewAmount`, `NewAmountZero`, `SignatureHashType`, `SourceId`, `TotalSupply`, `FreezableToken`, `TokenUnfreezable`, `TxAdditionalInfo`, `PoolInfo`, `OrderInfo`.

#### Node client (`github.com/mintlayer/go-sdk/node`)
- JSON-RPC 2.0 transport with `WithBasicAuth`, `WithHTTPClient`, `WithTimeout` options.
- **chainstate**: `ChainstateInfo`, `BestBlockID`, `BestBlockHeight`, `BlockIDAtHeight`, `BlockHeightInMainChain`, `GetBlock`, `GetBlockJSON`, `GetMainchainBlocks`, `GetUTXO`, `StakePoolBalance`, `StakerBalance`, `PoolDecommissionDestination`, `DelegationShare`, `TokenInfo`, `TokensInfo`, `OrderInfo`, `OrdersInfoByCurrencies`, `SubmitBlock`.
- **mempool**: `ContainsTx`, `ContainsOrphanTx`, `GetTransaction`, `SubmitTransaction`, `GetFeeRate`, `GetFeeRatePoints`, `MemoryUsage`.
- **node**: `NodeVersion`, `NodeShutdown`.
- **p2p**: `GetPeerCount`, `GetConnectedPeers`, `GetBindAddresses`, `AddReservedNode`, `RemoveReservedNode`, `Connect`, `Disconnect`, `ListBanned`, `Ban`, `Unban`.
- `RPCError` type with `Code` and `Message`.
- Unit tests using `httptest.NewServer` covering all RPC methods and error paths.

#### Indexer client (`github.com/mintlayer/go-sdk/indexer`)
- HTTP REST transport with `WithHTTPClient`, `WithTimeout` options.
- **chain**: `GetTip`, `GetGenesis`, `GetBlockIDAtHeight`.
- **block**: `GetBlock`, `GetBlockHeader`, `GetBlockReward`, `GetBlockTransactionIDs`.
- **transaction**: `ListTransactions`, `GetTransaction`, `GetTransactionMerklePath`, `GetTransactionOutput`, `SubmitTransaction`.
- **address**: `GetAddressInfo`, `GetSpendableUTXOs`, `GetAllUTXOs`, `GetDelegations`, `GetTokenAuthority`.
- **pool**: `ListPools`, `GetPool`, `GetPoolBlockStats`, `GetPoolDelegations`.
- **token/NFT**: `ListTokens`, `GetToken`, `GetTokenTransactions`, `FindTokensByTicker`, `GetNFT`.
- **order**: `ListOrders`, `GetOrder`, `ListOrdersByPair`.
- **statistics**: `GetCoinStatistics`, `GetTokenStatistics`, `GetFeeRate`.
- `PageOpts` and `PoolListOpts` pagination helpers.
- `HTTPError` type with `StatusCode` and `Body`.
- Unit tests using `httptest.NewServer`.

#### Wallet client (`github.com/mintlayer/go-sdk/wallet`)
- JSON-RPC 2.0 transport with `WithBasicAuth`, `WithHTTPClient`, `WithTimeout` options.
- **management**: `CreateWallet`, `RecoverWallet`, `OpenWallet`, `CloseWallet`, `GetWalletInfo`, `SyncWallet`, `RescanWallet`, `BestBlock`, `CreateAccount`, `RenameAccount`, `GetBalance`, `NewAddress`, `ShowReceiveAddresses`, `RevealPublicKey`, `EncryptPrivateKeys`, `UnlockPrivateKeys`, `LockPrivateKeys`.
- **transactions**: `AddressSend`, `TokenSend`, `SweepSpendable`, `SpendUTXO`, `ComposeTransaction`, `SignRawTransaction`, `InspectTransaction`, `SubmitTransaction`, `ListTransactionsByAddress`, `ListPendingTransactions`, `GetTransaction`, `AbandonTransaction`, `DepositData`.
- **staking**: `CreateStakePool`, `DecommissionStakePool`, `ListOwnedPools`, `GetPoolBalance`, `StartStaking`, `StopStaking`, `GetStakingStatus`, `CreateDelegation`, `DelegateStaking`, `WithdrawFromDelegation`, `ListDelegations`.
- **tokens**: `IssueToken`, `IssueNFT`, `MintTokens`, `UnmintTokens`, `LockTokenSupply`, `FreezeToken`, `UnfreezeToken`, `ChangeTokenAuthority`, `SendToken`.
- `RPCError` type with `Code` and `Message`.
- Unit tests using `httptest.NewServer`.

#### WASM client (`github.com/mintlayer/go-sdk/wasm`)
- Embedded `wasm_wrappers_bg.wasm` — no CGO, pure Go.
- **keys**: `MakePrivateKey`, `MakeDefaultAccountPrivkey`, `PublicKeyFromPrivateKey`, `ExtendedPublicKeyFromExtendedPrivateKey`, `MakeReceivingAddress`, `MakeChangeAddress`, `MakeReceivingAddressPublicKey`, `MakeChangeAddressPublicKey`.
- **addresses**: `EncodeDestination`, `PubkeyToPubkeyHashAddress`, `EncodeMultisigChallenge`, `MultisigChallengeToAddress`.
- **inputs**: `EncodeInputForUtxo`, `EncodeInputForWithdrawFromDelegation`, `EncodeInputForMintTokens`, `EncodeInputForUnmintTokens`, `EncodeInputForLockTokenSupply`, `EncodeInputForFreezeToken`, `EncodeInputForUnfreezeToken`, `EncodeInputForChangeTokenAuthority`, `EncodeInputForChangeTokenMetadataURI`, `EncodeInputForConcludeOrder`, `EncodeInputForFillOrder`, `EncodeInputForFreezeOrder`.
- **outputs**: `EncodeOutputTransfer`, `EncodeOutputTokenTransfer`, `EncodeOutputLockThenTransfer`, `EncodeOutputTokenLockThenTransfer`, `EncodeOutputCoinBurn`, `EncodeOutputTokenBurn`, `EncodeOutputCreateDelegation`, `EncodeOutputDelegateStaking`, `EncodeOutputCreateStakePool`, `EncodeOutputProduceBlockFromStake`, `EncodeOutputDataDeposit`, `EncodeOutputHTLC`, `EncodeOutputIssueFungibleToken`, `EncodeOutputIssueNFT`, `EncodeCreateOrderOutput`.
- **transactions**: `EncodeTransaction`, `EncodeOutpointSourceId`, `GetTransactionID`, `EstimateTransactionSize`, `EncodeSignedTransaction`, `EncodePartiallySignedTransaction`, `DecodePartiallySignedTransactionToJS`, `DecodeSignedTransactionToJS`, `ExtractHTLCSecret`, `InternalVerifyWitness`.
- **signing**: `EncodeWitness`, `EncodeWitnessNoSignature`, `EncodeWitnessHTLCSpend`, `EncodeWitnessHTLCRefundSingleSig`, `EncodeWitnessHTLCRefundMultisig`, `SignChallenge`, `VerifyChallenge`, `SignMessageForSpending`, `VerifySignatureForSpending`.
- **timelocks**: `EncodeLockForBlockCount`, `EncodeLockForSeconds`, `EncodeLockUntilHeight`, `EncodeLockUntilTime`.
- **staking**: `EncodeStakePoolData`, `EffectivePoolBalance`, `StakingPoolSpendMaturityBlockCount`.
- **fees**: `FungibleTokenIssuanceFee`, `NftIssuanceFee`, `DataDepositFee`, `TokenSupplyChangeFee`, `TokenFreezeFee`, `TokenChangeAuthorityFee`.
- **IDs**: `GetPoolId`, `GetTokenId`, `GetDelegationId`, `GetOrderId`.
- **intent**: `MakeTransactionIntentMessageToSign`, `EncodeSignedTransactionIntent`, `VerifyTransactionIntent`.
- `Amount` value type with `NewAmount`, `NewAmountZero`, `Atoms()`, JSON marshal/unmarshal.
- `Network`, `SignatureHashType`, `SourceId`, `TotalSupply`, `FreezableToken`, `TokenUnfreezable` enum types with documented constants.

#### Examples
- `examples/send-coins/` — full manual send flow: key derivation → UTXO fetch → build → sign → submit.
- `examples/issue-token/` — issue a fungible token and mint initial supply via the wallet daemon.

#### CI
- GitHub Actions workflow (`.github/workflows/ci.yml`):
  - `build` job: `go build ./...` and `go vet` across Go 1.21, 1.22, 1.23.
  - Separate `test-node`, `test-indexer`, `test-wallet`, `test-wasm` jobs for fast per-package feedback.
  - `all-tests` gate job that requires all test jobs to pass.

[Unreleased]: https://github.com/mintlayer/go-sdk/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/mintlayer/go-sdk/releases/tag/v0.1.0
