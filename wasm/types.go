// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mintlayer

import "encoding/json"

// Network represents a Mintlayer blockchain network.
type Network int32

const (
	Mainnet Network = 0 // Production network.
	Testnet Network = 1 // Public test network.
	Regtest Network = 2 // Local regression-test network (instant block generation).
	Signet  Network = 3 // Signet (controlled-mining test network).
)

// SignatureHashType controls which parts of a transaction are covered by a signature.
// Similar to Bitcoin's sighash types.
type SignatureHashType int32

const (
	// SigHashAll signs all inputs and all outputs. Most common; use this for normal transfers.
	SigHashAll SignatureHashType = 0
	// SigHashNone signs all inputs but no outputs, allowing outputs to be set by other parties.
	SigHashNone SignatureHashType = 1
	// SigHashSingle signs all inputs and the output at the same index as the signed input.
	SigHashSingle SignatureHashType = 2
	// SigHashAnyoneCanPay allows additional inputs to be added to the transaction by third parties.
	SigHashAnyoneCanPay SignatureHashType = 3
)

// SourceId identifies whether a UTXO comes from a transaction output or a block reward.
type SourceId int32

const (
	// SourceTransaction indicates the UTXO was created by a regular transaction output.
	SourceTransaction SourceId = 0
	// SourceBlockReward indicates the UTXO was created by a block reward (coinbase).
	SourceBlockReward SourceId = 1
)

// TotalSupply describes the supply policy of a fungible token.
type TotalSupply int32

const (
	// TotalSupplyLockable means the supply is unlimited until explicitly locked.
	TotalSupplyLockable TotalSupply = 0
	// TotalSupplyUnlimited means the supply is always unlimited.
	TotalSupplyUnlimited TotalSupply = 1
	// TotalSupplyFixed means the supply is fixed at issuance.
	TotalSupplyFixed TotalSupply = 2
)

// FreezableToken indicates whether a token can be frozen after issuance.
type FreezableToken int32

const (
	FreezableNo  FreezableToken = 0
	FreezableYes FreezableToken = 1
)

// TokenUnfreezable indicates whether a frozen token can later be unfrozen.
type TokenUnfreezable int32

const (
	TokenUnfreezableNo  TokenUnfreezable = 0
	TokenUnfreezableYes TokenUnfreezable = 1
)

// Amount represents a coin or token quantity as a decimal atom count.
// Atoms are the smallest, indivisible unit. For ML coins, 1 ML = 1e11 atoms.
//
// The zero value (empty atoms string) is invalid; use [NewAmount] or [NewAmountFromUint64].
type Amount struct {
	atoms string
}

// NewAmount creates an Amount from a decimal atom string (e.g. "100000000000").
func NewAmount(atoms string) Amount {
	return Amount{atoms: atoms}
}

// NewAmountZero returns an Amount representing zero atoms.
func NewAmountZero() Amount {
	return Amount{atoms: "0"}
}

// Atoms returns the decimal string representation of the atom count.
func (a Amount) Atoms() string {
	return a.atoms
}

// String implements fmt.Stringer.
func (a Amount) String() string {
	return a.atoms
}

// MarshalJSON serialises the amount as {"atoms":"<value>"}.
func (a Amount) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Atoms string `json:"atoms"`
	}{Atoms: a.atoms})
}

// UnmarshalJSON deserialises {"atoms":"<value>"} into an Amount.
func (a *Amount) UnmarshalJSON(data []byte) error {
	var v struct {
		Atoms string `json:"atoms"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	a.atoms = v.Atoms
	return nil
}

// SimpleCurrencyAmount is used inside TxAdditionalInfo to represent ask/give balances.
type SimpleCurrencyAmount struct {
	Atoms string `json:"atoms"`
}

// PoolInfo holds pool-related data required for transaction signing.
type PoolInfo struct {
	StakerBalance Amount `json:"staker_balance"`
}

// OrderInfo holds DEX order data required for transaction signing.
type OrderInfo struct {
	InitiallyAsked SimpleCurrencyAmount `json:"initially_asked"`
	InitiallyGiven SimpleCurrencyAmount `json:"initially_given"`
	AskBalance     Amount               `json:"ask_balance"`
	GiveBalance    Amount               `json:"give_balance"`
}

// TxAdditionalInfo provides out-of-band data (pool/order state) needed when signing
// transactions that spend pool or order UTXOs.
type TxAdditionalInfo struct {
	PoolInfo  map[string]PoolInfo  `json:"pool_info"`
	OrderInfo map[string]OrderInfo `json:"order_info"`
}
