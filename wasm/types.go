// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mintlayer

import (
	"encoding/json"
	"fmt"
)

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
// Mintlayer models these as a CurrencyAmount enum; the JSON shape is
// externally tagged:
//
//	{"coins":{"atoms":"<atoms>"}}
//	{"tokens":{"atoms":"<atoms>","token_id":"<bech32 token id>"}}
type SimpleCurrencyAmount struct {
	Atoms string `json:"-"`
	// Kind selects the JSON variant (and thus coins vs tokens semantics).
	// The zero value is CurrencyCoins, matching the common coin case.
	Kind CurrencyAmountKind `json:"-"`
	// TokenID is the token the amount is denominated in; required when
	// Kind is CurrencyTokens, ignored for CurrencyCoins.
	TokenID *string `json:"-"`
}

// CurrencyAmountKind selects the coins/tokens variant of a SimpleCurrencyAmount.
type CurrencyAmountKind uint8

const (
	// CurrencyCoins marks a SimpleCurrencyAmount as native coins.
	CurrencyCoins CurrencyAmountKind = iota
	// CurrencyTokens marks a SimpleCurrencyAmount as fungible tokens.
	CurrencyTokens
)

// CoinsAmount returns a SimpleCurrencyAmount carrying native coins.
func CoinsAmount(atoms string) SimpleCurrencyAmount {
	return SimpleCurrencyAmount{Atoms: atoms, Kind: CurrencyCoins}
}

// TokensAmount returns a SimpleCurrencyAmount carrying fungible tokens of the
// given token ID (bech32).
func TokensAmount(atoms, tokenID string) SimpleCurrencyAmount {
	return SimpleCurrencyAmount{Atoms: atoms, Kind: CurrencyTokens, TokenID: &tokenID}
}

// MarshalJSON serialises the amount as
// {"coins":{"atoms":"<atoms>"}} or
// {"tokens":{"amount":{"atoms":"<atoms>"},"token_id":"..."}} — the
// CurrencyAmount enum shape the wasm module expects.
func (a SimpleCurrencyAmount) MarshalJSON() ([]byte, error) {
	balance := func(atoms string, tokenID *string) ([]byte, error) {
		inner, err := json.Marshal(struct {
			Atoms string `json:"atoms"`
		}{Atoms: atoms})
		if err != nil {
			return nil, err
		}
		return json.Marshal(struct {
			Amount  json.RawMessage `json:"amount"`
			TokenID *string         `json:"token_id"`
		}{Amount: inner, TokenID: tokenID})
	}
	if a.Kind == CurrencyTokens {
		tokens, err := balance(a.Atoms, a.TokenID)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]json.RawMessage{"tokens": tokens})
	}
	coins, err := json.Marshal(struct {
		Atoms string `json:"atoms"`
	}{Atoms: a.Atoms})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]json.RawMessage{"coins": coins})
}

// UnmarshalJSON deserialises the externally tagged CurrencyAmount shapes
// {"coins":{"atoms":"<atoms>"}} and
// {"tokens":{"amount":{"atoms":"..."},"token_id":"..."}}.
func (a *SimpleCurrencyAmount) UnmarshalJSON(data []byte) error {
	var v map[string]json.RawMessage
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	for variant, raw := range v {
		switch variant {
		case "coins":
			a.Kind = CurrencyCoins
			a.TokenID = nil
		case "tokens":
			a.Kind = CurrencyTokens
		default:
			continue
		}
		var sa struct {
			Atoms  string `json:"atoms"`
			Amount *struct {
				Atoms string `json:"atoms"`
			} `json:"amount"`
			TokenID *string `json:"token_id"`
		}
		if err := json.Unmarshal(raw, &sa); err != nil {
			// tolerate the flat string form {"coins":"<atoms>"}
			var s string
			if err2 := json.Unmarshal(raw, &s); err2 != nil {
				return err
			}
			sa.Atoms = s
		} else if sa.Amount != nil {
			sa.Atoms = sa.Amount.Atoms
		}
		a.Atoms = sa.Atoms
		a.TokenID = sa.TokenID
		return nil
	}
	return fmt.Errorf("mintlayer: SimpleCurrencyAmount: unknown variant (expected coins or tokens)")
}

// PoolInfo holds pool-related data required for transaction signing.
type PoolInfo struct {
	StakerBalance Amount `json:"staker_balance"`
}

// OrderBalance is one side of a DEX order's remaining balance: an atom count
// plus the token it is denominated in (TokenID nil = native coins).
type OrderBalance struct {
	Atoms   string  `json:"-"`
	TokenID *string `json:"-"`
}

type orderBalanceJSON struct {
	Atoms  string `json:"atoms"`
	Amount struct {
		Atoms string `json:"atoms"`
	} `json:"amount"`
	TokenID *string `json:"token_id"`
}

// MarshalJSON serialises as
// {"atoms":"<atoms>","amount":{"atoms":"<atoms>"},"token_id":null|"..."} —
// the redundant-but-required shape this wasm module's OrderBalance expects.
func (b OrderBalance) MarshalJSON() ([]byte, error) {
	var ob orderBalanceJSON
	ob.Atoms = b.Atoms
	ob.Amount.Atoms = b.Atoms
	ob.TokenID = b.TokenID
	return json.Marshal(ob)
}

// UnmarshalJSON accepts {"amount":{"atoms":"..."},"token_id":...} and the
// flatter {"atoms":"...","token_id":...} shape.
func (b *OrderBalance) UnmarshalJSON(data []byte) error {
	var ob orderBalanceJSON
	if err := json.Unmarshal(data, &ob); err == nil && ob.Amount.Atoms != "" {
		b.Atoms, b.TokenID = ob.Amount.Atoms, ob.TokenID
		return nil
	}
	var flat struct {
		Atoms   string  `json:"atoms"`
		TokenID *string `json:"token_id"`
	}
	if err := json.Unmarshal(data, &flat); err != nil {
		return err
	}
	b.Atoms, b.TokenID = flat.Atoms, flat.TokenID
	return nil
}

// OrderInfo holds DEX order data required for transaction signing.
type OrderInfo struct {
	InitiallyAsked SimpleCurrencyAmount `json:"initially_asked"`
	InitiallyGiven SimpleCurrencyAmount `json:"initially_given"`
	AskBalance     OrderBalance         `json:"ask_balance"`
	GiveBalance    OrderBalance         `json:"give_balance"`
}

// TxAdditionalInfo provides out-of-band data (pool/order state) needed when signing
// transactions that spend pool or order UTXOs.
type TxAdditionalInfo struct {
	PoolInfo  map[string]PoolInfo  `json:"pool_info"`
	OrderInfo map[string]OrderInfo `json:"order_info"`
}
