// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package wallet

import "encoding/json"

// Amount represents a coin or token amount. At least one of Atoms or Decimal must be set.
// The daemon accepts either field; responses always include both.
type Amount struct {
	Atoms   string `json:"atoms,omitempty"`
	Decimal string `json:"decimal,omitempty"`
}

// Timestamp wraps a Unix seconds timestamp.
type Timestamp struct {
	Timestamp int64 `json:"timestamp"`
}

// TxOptions controls fee priority and broadcasting behaviour for transaction methods.
type TxOptions struct {
	// InTopXMb targets the transaction to be in the top X MB of the mempool priority queue.
	InTopXMb *uint32 `json:"in_top_x_mb"`
	// BroadcastToMempool controls whether to broadcast immediately (default: true).
	BroadcastToMempool *bool `json:"broadcast_to_mempool"`
}

// OutpointSourceID identifies the transaction or block reward that produced a UTXO.
type OutpointSourceID struct {
	Type    string          `json:"type"`    // "Transaction" or "BlockReward"
	Content json.RawMessage `json:"content"` // {"tx_id":"hex"} or {"block_id":"hex"}
}

// Outpoint identifies a specific output within a transaction or block reward.
type Outpoint struct {
	SourceID OutpointSourceID `json:"source_id"`
	Index    uint32           `json:"index"`
}

// FeesBreakdown contains the fees paid by a transaction, split by coin and token.
type FeesBreakdown struct {
	Coins  Amount            `json:"coins"`
	Tokens map[string]Amount `json:"tokens"`
}

// SendResult is returned by transaction-sending methods.
type SendResult struct {
	TxID        string        `json:"tx_id"`
	Fees        FeesBreakdown `json:"fees"`
	Broadcasted bool          `json:"broadcasted"`
}

// SubmitResult is returned by SubmitTransaction.
type SubmitResult struct {
	TxID string `json:"tx_id"`
}

// ComposedTx is returned by ComposeTransaction.
type ComposedTx struct {
	// Hex is the hex-encoded PartiallySignedTransaction.
	Hex  string        `json:"hex"`
	Fees FeesBreakdown `json:"fees"`
}

// SignedTx is returned by SignRawTransaction.
type SignedTx struct {
	// Hex is the hex-encoded (partially or fully) signed transaction.
	Hex               string          `json:"hex"`
	CurrentSignatures json.RawMessage `json:"current_signatures"`
}

// TxStats holds aggregate statistics about a transaction's inputs and signatures.
type TxStats struct {
	NumInputs       uint32 `json:"num_inputs"`
	TotalSignatures uint32 `json:"total_signatures"`
}

// TxInspection is returned by InspectTransaction.
type TxInspection struct {
	Stats TxStats        `json:"stats"`
	Fees  *FeesBreakdown `json:"fees,omitempty"`
}

// WalletTx is one entry from ListTransactionsByAddress.
type WalletTx struct {
	ID        string    `json:"id"`
	Height    uint64    `json:"height"`
	Timestamp Timestamp `json:"timestamp"`
}

// -- Wallet management types --

// MnemonicContent holds a BIP-39 mnemonic phrase.
type MnemonicContent struct {
	Mnemonic string `json:"mnemonic"`
}

// MnemonicResult is the mnemonic field in CreateWalletResult.
// Type is "NewlyGenerated" when the daemon generated the mnemonic, or
// "UserProvided" when the caller supplied it.
type MnemonicResult struct {
	Type    string           `json:"type"`
	Content *MnemonicContent `json:"content,omitempty"`
}

// CreateWalletResult is returned by CreateWallet.
type CreateWalletResult struct {
	Mnemonic *MnemonicResult `json:"mnemonic,omitempty"`
}

// CreateWalletParams is the parameter block for CreateWallet.
type CreateWalletParams struct {
	Path            string  `json:"path"`
	StoreSeedPhrase bool    `json:"store_seed_phrase"`
	Mnemonic        *string `json:"mnemonic"`
	Passphrase      *string `json:"passphrase"`
	HardwareWallet  *string `json:"hardware_wallet"`
}

// RecoverWalletParams is the parameter block for RecoverWallet.
type RecoverWalletParams struct {
	Path            string  `json:"path"`
	StoreSeedPhrase bool    `json:"store_seed_phrase"`
	Mnemonic        string  `json:"mnemonic"`
	Passphrase      *string `json:"passphrase"`
	HardwareWallet  *string `json:"hardware_wallet"`
}

// WalletExtraInfo contains hardware wallet type information.
type WalletExtraInfo struct {
	// Type is one of "SoftwareWallet", "TrezorWallet", "LedgerWallet".
	Type string `json:"type"`
}

// WalletInfo is returned by GetWalletInfo.
type WalletInfo struct {
	WalletID     string          `json:"wallet_id"`
	AccountNames []string        `json:"account_names"`
	ExtraInfo    WalletExtraInfo `json:"extra_info"`
}

// BestBlock is returned by BestBlock.
type BestBlock struct {
	Height uint64 `json:"height"`
	ID     string `json:"id"`
}

// AccountInfo is returned by CreateAccount.
type AccountInfo struct {
	Account uint32 `json:"account"`
	Name    string `json:"name"`
}

// Balance is returned by GetBalance.
type Balance struct {
	Coins  Amount            `json:"coins"`
	Tokens map[string]Amount `json:"tokens"`
}

// AddressWithUsage is one entry from ShowReceiveAddresses.
type AddressWithUsage struct {
	Address string `json:"address"`
	Used    bool   `json:"used"`
	Coins   Amount `json:"coins"`
}

// RevealPublicKeyResult is returned by RevealPublicKey.
type RevealPublicKeyResult struct {
	PublicKeyHex     string `json:"public_key_hex"`
	PublicKeyAddress string `json:"public_key_address"`
}

// -- Transaction parameter types --

// SendParams is the parameter block for AddressSend.
type SendParams struct {
	Account       uint32     `json:"account"`
	Address       string     `json:"address"`
	Amount        Amount     `json:"amount"`
	SelectedUTXOs []Outpoint `json:"selected_utxos"`
	Options       TxOptions  `json:"options"`
}

// SweepParams is the parameter block for SweepSpendable.
type SweepParams struct {
	Account            uint32    `json:"account"`
	DestinationAddress string    `json:"destination_address"`
	FromAddresses      []string  `json:"from_addresses"`
	All                bool      `json:"all"`
	Options            TxOptions `json:"options"`
}

// UTXOSpendParams is the parameter block for SpendUTXO.
type UTXOSpendParams struct {
	Account       uint32    `json:"account"`
	UTXO          Outpoint  `json:"utxo"`
	OutputAddress string    `json:"output_address"`
	HTLCSecret    *string   `json:"htlc_secret"`
	Options       TxOptions `json:"options"`
}

// ComposeParams is the parameter block for ComposeTransaction.
type ComposeParams struct {
	Inputs          []Outpoint        `json:"inputs"`
	Outputs         []json.RawMessage `json:"outputs"`
	HTLCSecrets     *json.RawMessage  `json:"htlc_secrets"`
	OnlyTransaction bool              `json:"only_transaction"`
}

// -- Staking parameter types --

// CreatePoolParams is the parameter block for CreateStakePool.
type CreatePoolParams struct {
	Account                uint32    `json:"account"`
	Amount                 Amount    `json:"amount"`
	CostPerBlock           Amount    `json:"cost_per_block"`
	MarginRatioPerThousand string    `json:"margin_ratio_per_thousand"`
	DecommissionAddress    string    `json:"decommission_address"`
	StakerAddress          *string   `json:"staker_address"`
	VRFPublicKey           *string   `json:"vrf_public_key"`
	Options                TxOptions `json:"options"`
}

// DecommissionParams is the parameter block for DecommissionStakePool.
type DecommissionParams struct {
	Account       uint32    `json:"account"`
	PoolID        string    `json:"pool_id"`
	OutputAddress string    `json:"output_address"`
	Options       TxOptions `json:"options"`
}

// OwnedPool is one entry from ListOwnedPools.
type OwnedPool struct {
	PoolID                 string `json:"pool_id"`
	Pledge                 Amount `json:"pledge"`
	Balance                Amount `json:"balance"`
	MarginRatioPerThousand string `json:"margin_ratio_per_thousand"`
	CostPerBlock           Amount `json:"cost_per_block"`
}

// StakingStatus is returned by GetStakingStatus.
// Possible values: "Staking", "NotStaking".
type StakingStatus string

const (
	StakingStatusActive   StakingStatus = "Staking"
	StakingStatusInactive StakingStatus = "NotStaking"
)

// CreateDelegationParams is the parameter block for CreateDelegation.
type CreateDelegationParams struct {
	Account uint32    `json:"account"`
	Address string    `json:"address"`
	PoolID  string    `json:"pool_id"`
	Options TxOptions `json:"options"`
}

// CreateDelegationResult is returned by CreateDelegation.
type CreateDelegationResult struct {
	DelegationID string `json:"delegation_id"`
	TxID         string `json:"tx_id"`
}

// DelegateParams is the parameter block for DelegateStaking.
type DelegateParams struct {
	Account      uint32    `json:"account"`
	Amount       Amount    `json:"amount"`
	DelegationID string    `json:"delegation_id"`
	Options      TxOptions `json:"options"`
}

// WithdrawParams is the parameter block for WithdrawFromDelegation.
type WithdrawParams struct {
	Account      uint32    `json:"account"`
	Address      string    `json:"address"`
	Amount       Amount    `json:"amount"`
	DelegationID string    `json:"delegation_id"`
	Options      TxOptions `json:"options"`
}

// DelegationInfo is one entry from ListDelegations.
type DelegationInfo struct {
	DelegationID string `json:"delegation_id"`
	PoolID       string `json:"pool_id"`
	Balance      Amount `json:"balance"`
}

// -- Token parameter types --

// TokenSupply describes the supply policy of a fungible token.
// Type is one of "Fixed", "Lockable", "Unlimited".
// Content is only set for "Fixed" supply (holds the maximum amount).
type TokenSupply struct {
	Type    string  `json:"type"`
	Content *Amount `json:"content,omitempty"`
}

// TokenMetadata describes a fungible token at issuance time.
type TokenMetadata struct {
	TokenTicker      string      `json:"token_ticker"`
	NumberOfDecimals uint8       `json:"number_of_decimals"`
	MetadataURI      string      `json:"metadata_uri"`
	TokenSupply      TokenSupply `json:"token_supply"`
	IsFreezable      bool        `json:"is_freezable"`
}

// IssueTokenParams is the parameter block for IssueToken.
type IssueTokenParams struct {
	Account            uint32        `json:"account"`
	DestinationAddress string        `json:"destination_address"`
	Metadata           TokenMetadata `json:"metadata"`
	Options            TxOptions     `json:"options"`
}

// IssueTokenResult is returned by IssueToken and IssueNFT.
type IssueTokenResult struct {
	TokenID string `json:"token_id"`
	TxID    string `json:"tx_id"`
}

// NFTMetadata describes an NFT at issuance time.
type NFTMetadata struct {
	MediaHash             string  `json:"media_hash"`
	Name                  string  `json:"name"`
	Description           string  `json:"description"`
	Ticker                string  `json:"ticker"`
	Creator               *string `json:"creator"`
	IconURI               *string `json:"icon_uri"`
	MediaURI              *string `json:"media_uri"`
	AdditionalMetadataURI *string `json:"additional_metadata_uri"`
}

// IssueNFTParams is the parameter block for IssueNFT.
type IssueNFTParams struct {
	Account            uint32      `json:"account"`
	DestinationAddress string      `json:"destination_address"`
	Metadata           NFTMetadata `json:"metadata"`
	Options            TxOptions   `json:"options"`
}

// MintParams is the parameter block for MintTokens.
type MintParams struct {
	Account uint32    `json:"account"`
	TokenID string    `json:"token_id"`
	Address string    `json:"address"`
	Amount  Amount    `json:"amount"`
	Options TxOptions `json:"options"`
}

// UnmintParams is the parameter block for UnmintTokens.
type UnmintParams struct {
	Account uint32    `json:"account"`
	TokenID string    `json:"token_id"`
	Amount  Amount    `json:"amount"`
	Options TxOptions `json:"options"`
}

// LockSupplyParams is the parameter block for LockTokenSupply.
// Note: the wire field is account_index, not account.
type LockSupplyParams struct {
	AccountIndex uint32    `json:"account_index"`
	TokenID      string    `json:"token_id"`
	Options      TxOptions `json:"options"`
}

// FreezeParams is the parameter block for FreezeToken.
type FreezeParams struct {
	Account       uint32    `json:"account"`
	TokenID       string    `json:"token_id"`
	IsUnfreezable bool      `json:"is_unfreezable"`
	Options       TxOptions `json:"options"`
}

// UnfreezeParams is the parameter block for UnfreezeToken.
type UnfreezeParams struct {
	Account uint32    `json:"account"`
	TokenID string    `json:"token_id"`
	Options TxOptions `json:"options"`
}

// ChangeAuthorityParams is the parameter block for ChangeTokenAuthority.
type ChangeAuthorityParams struct {
	Account uint32    `json:"account"`
	TokenID string    `json:"token_id"`
	Address string    `json:"address"`
	Options TxOptions `json:"options"`
}

// TokenSendParams is the parameter block for SendToken (and TokenSend in transactions).
type TokenSendParams struct {
	Account uint32    `json:"account"`
	TokenID string    `json:"token_id"`
	Address string    `json:"address"`
	Amount  Amount    `json:"amount"`
	Options TxOptions `json:"options"`
}
