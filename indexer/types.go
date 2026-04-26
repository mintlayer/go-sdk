// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package indexer

import (
	"encoding/json"
	"net/url"
	"strconv"
)

// Amount represents a Mintlayer coin or token amount in both atoms and decimal form.
type Amount struct {
	Atoms   string `json:"atoms"`
	Decimal string `json:"decimal"`
}

// Timestamp wraps a Unix seconds timestamp returned by the indexer.
type Timestamp struct {
	Timestamp int64 `json:"timestamp"`
}

// PageOpts controls pagination for list endpoints.
// Zero values use server defaults (offset=0, items=10).
type PageOpts struct {
	Offset uint32
	Items  uint32
}

// PoolListOpts extends PageOpts with a pool-specific sort order.
type PoolListOpts struct {
	PageOpts
	// Sort is "by_height" (default, newest first) or "by_pledge" (largest staker balance first).
	Sort string
}

// TxOutput is the raw JSON of one transaction output.
// Its shape is determined by the "type" field.
// Common types: "Transfer", "LockThenTransfer", "Burn", "CreateStakePool",
// "ProduceBlockFromStake", "CreateDelegationId", "DelegateStaking",
// "IssueFungibleToken", "IssueNft", "DataDeposit", "Htlc", "CreateOrder".
type TxOutput = json.RawMessage

// ChainTip is returned by GetTip.
type ChainTip struct {
	BlockHeight uint64 `json:"block_height"`
	BlockID     string `json:"block_id"`
}

// GenesisInfo is returned by GetGenesis.
type GenesisInfo struct {
	BlockID        string          `json:"block_id"`
	GenesisMessage string          `json:"genesis_message"`
	Timestamp      Timestamp       `json:"timestamp"`
	UTXOs          json.RawMessage `json:"utxos"`
}

// BlockHeader holds the header fields of a block.
type BlockHeader struct {
	PreviousBlockID   string          `json:"previous_block_id"`
	Timestamp         Timestamp       `json:"timestamp"`
	MerkleRoot        string          `json:"merkle_root"`
	WitnessMerkleRoot string          `json:"witness_merkle_root"`
	ConsensusData     json.RawMessage `json:"consensus_data"`
}

// BlockBody holds the reward outputs and transactions of a block.
type BlockBody struct {
	Reward       []TxOutput    `json:"reward"`
	Transactions []Transaction `json:"transactions"`
}

// Block is returned by GetBlock.
type Block struct {
	Height uint64      `json:"height"`
	Header BlockHeader `json:"header"`
	Body   BlockBody   `json:"body"`
}

// Transaction is returned by GetTransaction and ListTransactions.
// BlockID, Timestamp, and Confirmations are empty strings for unconfirmed transactions.
type Transaction struct {
	ID            string          `json:"id"`
	Inputs        json.RawMessage `json:"inputs"`
	Outputs       json.RawMessage `json:"outputs"`
	BlockID       string          `json:"block_id"`
	Timestamp     string          `json:"timestamp"`
	Confirmations string          `json:"confirmations"`
}

// MerklePath is returned by GetTransactionMerklePath.
type MerklePath struct {
	BlockID          string   `json:"block_id"`
	TransactionIndex uint32   `json:"transaction_index"`
	MerkleRoot       string   `json:"merkle_root"`
	Path             []string `json:"merkle_path"`
}

// TokenBalance is one entry in AddressInfo.Tokens.
type TokenBalance struct {
	TokenID string `json:"token_id"`
	Amount  Amount `json:"amount"`
}

// AddressInfo is returned by GetAddressInfo.
type AddressInfo struct {
	CoinBalance        Amount         `json:"coin_balance"`
	LockedCoinBalance  Amount         `json:"locked_coin_balance"`
	TransactionHistory []string       `json:"transaction_history"`
	Tokens             []TokenBalance `json:"tokens"`
}

// UTXOOutpoint identifies a UTXO by its source transaction id and output index.
type UTXOOutpoint struct {
	SourceID string `json:"source_id"`
	Index    uint32 `json:"index"`
}

// UTXO is one entry returned by GetSpendableUTXOs and GetAllUTXOs.
type UTXO struct {
	Outpoint UTXOOutpoint `json:"outpoint"`
	Output   TxOutput     `json:"utxo"`
}

// DelegationInfo is one entry returned by GetDelegations (address endpoint).
type DelegationInfo struct {
	DelegationID     string `json:"delegation_id"`
	PoolID           string `json:"pool_id"`
	NextNonce        Uint64 `json:"next_nonce"`
	SpendDestination string `json:"spend_destination"`
	Balance          Amount `json:"balance"`
}

// Pool is one entry returned by ListPools and GetPool.
type Pool struct {
	PoolID                  string `json:"pool_id"`
	DecommissionDestination string `json:"decommission_destination"`
	StakerBalance           Amount `json:"staker_balance"`
	MarginRatioPerThousand  PerThousand `json:"margin_ratio_per_thousand"`
	CostPerBlock            Amount `json:"cost_per_block"`
	VRFPublicKey            string `json:"vrf_public_key"`
	DelegationsBalance      Amount `json:"delegations_balance"`
}

// Delegation is returned by GetDelegation.
type Delegation struct {
	DelegationID        string `json:"delegation_id"`
	PoolID              string `json:"pool_id"`
	NextNonce           Uint64 `json:"next_nonce"`
	SpendDestination    string `json:"spend_destination"`
	Balance             Amount `json:"balance"`
	CreationBlockHeight Uint64 `json:"creation_block_height"`
}

// PoolDelegation is one entry returned by GetPoolDelegations.
type PoolDelegation struct {
	DelegationID        string `json:"delegation_id"`
	NextNonce           Uint64 `json:"next_nonce"`
	SpendDestination    string `json:"spend_destination"`
	Balance             Amount `json:"balance"`
	CreationBlockHeight Uint64 `json:"creation_block_height"`
}

// TokenInfo is returned by GetToken.
type TokenInfo struct {
	Authority         string          `json:"authority"`
	IsLocked          bool            `json:"is_locked"`
	CirculatingSupply Amount          `json:"circulating_supply"`
	TokenTicker       string          `json:"token_ticker"`
	MetadataURI       string          `json:"metadata_uri"`
	NumberOfDecimals  uint8           `json:"number_of_decimals"`
	TotalSupply       json.RawMessage `json:"total_supply"`
	Frozen            bool            `json:"frozen"`
	// IsTokenUnfreezable is non-nil only when Frozen is true.
	IsTokenUnfreezable *bool `json:"is_token_unfreezable"`
	// IsTokenFreezable is non-nil only when Frozen is false.
	IsTokenFreezable *bool  `json:"is_token_freezable"`
	NextNonce        Uint64 `json:"next_nonce"`
}

// TokenTx is one entry returned by GetTokenTransactions.
type TokenTx struct {
	TxGlobalIndex uint64 `json:"tx_global_index"`
	TxID          string `json:"tx_id"`
}

// NFTMetadata holds the on-chain metadata of an NFT.
type NFTMetadata struct {
	Creator               *string `json:"creator"`
	Name                  string  `json:"name"`
	Description           string  `json:"description"`
	Ticker                string  `json:"ticker"`
	IconURI               string  `json:"icon_uri"`
	AdditionalMetadataURI *string `json:"additional_metadata_uri"`
	MediaURI              *string `json:"media_uri"`
	MediaHash             string  `json:"media_hash"`
}

// NFTInfo is returned by GetNFT.
type NFTInfo struct {
	Owner    string      `json:"owner"`
	TokenID  string      `json:"token_id"`
	Metadata NFTMetadata `json:"metadata"`
}

// Order is one entry returned by ListOrders, GetOrder, and ListOrdersByPair.
// GiveCurrency and AskCurrency are raw JSON with a "type" field of "Coin" or "Token".
type Order struct {
	OrderID             string          `json:"order_id"`
	ConcludeDestination string          `json:"conclude_destination"`
	GiveCurrency        json.RawMessage `json:"give_currency"`
	InitiallyGiven      Amount          `json:"initially_given"`
	GiveBalance         Amount          `json:"give_balance"`
	AskCurrency         json.RawMessage `json:"ask_currency"`
	InitiallyAsked      Amount          `json:"initially_asked"`
	AskBalance          Amount          `json:"ask_balance"`
	Nonce               Uint64          `json:"nonce"`
}

// CoinStats is returned by GetCoinStatistics and GetTokenStatistics.
type CoinStats struct {
	CirculatingSupply Amount `json:"circulating_supply"`
	Preminted         Amount `json:"preminted"`
	Burned            Amount `json:"burned"`
	Staked            Amount `json:"staked"`
}

// pageQuery converts PageOpts to url.Values, omitting zero-value fields.
func pageQuery(opts PageOpts) url.Values {
	q := url.Values{}
	if opts.Offset > 0 {
		q.Set("offset", strconv.FormatUint(uint64(opts.Offset), 10))
	}
	if opts.Items > 0 {
		q.Set("items", strconv.FormatUint(uint64(opts.Items), 10))
	}
	return q
}
