// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package wallet

import (
	"context"
	"encoding/json"
	"fmt"
)

// OutputValue is one side of an order: native coins or a token amount.
type OutputValue struct {
	// Coin selects native ML coins when true, a token amount when false.
	Coin bool
	// TokenID is the bech32 token id; required when Coin is false.
	TokenID string
	// Amount carries the atom amount (and decimal, when the daemon sent one).
	Amount Amount
}

func (v OutputValue) MarshalJSON() ([]byte, error) {
	if v.Coin {
		return json.Marshal(struct {
			Type    string `json:"type"`
			Content struct {
				Amount Amount `json:"amount"`
			} `json:"content"`
		}{Type: "Coin", Content: struct {
			Amount Amount `json:"amount"`
		}{Amount: v.Amount}})
	}
	if v.TokenID == "" {
		return nil, fmt.Errorf("wallet: token OutputValue requires TokenID")
	}
	return json.Marshal(struct {
		Type    string `json:"type"`
		Content struct {
			ID     string `json:"id"`
			Amount Amount `json:"amount"`
		} `json:"content"`
	}{Type: "Token", Content: struct {
		ID     string `json:"id"`
		Amount Amount `json:"amount"`
	}{ID: v.TokenID, Amount: v.Amount}})
}

func (v *OutputValue) UnmarshalJSON(b []byte) error {
	var raw struct {
		Type    string `json:"type"`
		Content struct {
			ID     string  `json:"id"`
			Amount *Amount `json:"amount"`
		} `json:"content"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	switch raw.Type {
	case "Coin":
		v.Coin, v.TokenID = true, ""
	case "Token":
		v.Coin, v.TokenID = false, raw.Content.ID
	default:
		return fmt.Errorf("wallet: unknown output value type %q", raw.Type)
	}
	if raw.Content.Amount != nil {
		v.Amount = *raw.Content.Amount
	}
	return nil
}

// CurrencyFilter restricts ListAllActiveOrders to one currency side.
type CurrencyFilter struct {
	Type    string
	Content string
}

// CoinFilter matches the native coin.
func CoinFilter() *CurrencyFilter { return &CurrencyFilter{Type: "Coin"} }

// TokenFilter matches a token by its bech32 id.
func TokenFilter(tokenID string) *CurrencyFilter {
	return &CurrencyFilter{Type: "Token", Content: tokenID}
}

func (f *CurrencyFilter) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	}{Type: f.Type, Content: f.Content})
}

// OrderState is the live balance snapshot of an own order.
type OrderState struct {
	AskBalance  Amount    `json:"ask_balance"`
	GiveBalance Amount    `json:"give_balance"`
	Frozen      bool      `json:"is_frozen"`
	Creation    Timestamp `json:"creation_timestamp"`
}

// OwnOrder is an order whose conclude key is owned by the account.
type OwnOrder struct {
	OrderID         string      `json:"order_id"`
	InitiallyAsked  OutputValue `json:"initially_asked"`
	InitiallyGiven  OutputValue `json:"initially_given"`
	Existing        *OrderState `json:"existing_order_data"`
	MarkedFrozen    bool        `json:"is_marked_as_frozen_in_wallet"`
	MarkedConcluded bool        `json:"is_marked_as_concluded_in_wallet"`
}

// ActiveOrder is one order from the account-wide listing.
type ActiveOrder struct {
	OrderID        string      `json:"order_id"`
	InitiallyAsked OutputValue `json:"initially_asked"`
	InitiallyGiven OutputValue `json:"initially_given"`
	AskBalance     Amount      `json:"ask_balance"`
	GiveBalance    Amount      `json:"give_balance"`
	IsOwn          bool        `json:"is_own"`
}

// OrderCreated is the result of CreateOrder.
type OrderCreated struct {
	OrderID     string `json:"order_id"`
	TxID        string `json:"tx_id"`
	Broadcasted bool   `json:"broadcasted"`
}

// CreateOrderParams configures CreateOrder.
type CreateOrderParams struct {
	Account         uint32      `json:"account"`
	Ask             OutputValue `json:"ask"`
	Give            OutputValue `json:"give"`
	ConcludeAddress string      `json:"conclude_address"`
	Options         TxOptions   `json:"options"`
}

// ConcludeOrderParams configures ConcludeOrder. An empty OutputAddress lets
// the daemon derive a fresh receive address for the remaining funds.
type ConcludeOrderParams struct {
	Account       uint32    `json:"account"`
	OrderID       string    `json:"order_id"`
	OutputAddress *string   `json:"output_address"`
	Options       TxOptions `json:"options"`
}

// FillOrderParams configures FillOrder. FillAmount is denominated in the
// order's ask currency. An empty OutputAddress lets the daemon derive one.
type FillOrderParams struct {
	Account       uint32    `json:"account"`
	OrderID       string    `json:"order_id"`
	FillAmount    Amount    `json:"fill_amount_in_ask_currency"`
	OutputAddress *string   `json:"output_address"`
	Options       TxOptions `json:"options"`
}

// FreezeOrderParams configures FreezeOrder.
type FreezeOrderParams struct {
	Account uint32    `json:"account"`
	OrderID string    `json:"order_id"`
	Options TxOptions `json:"options"`
}

// ListOrdersParams configures ListAllActiveOrders; nil filters match any
// currency on that side.
type ListOrdersParams struct {
	Account      uint32          `json:"account"`
	AskCurrency  *CurrencyFilter `json:"ask_currency"`
	GiveCurrency *CurrencyFilter `json:"give_currency"`
}

// CreateOrder places a DEX order: give `Give`, ask for `Ask`. The order is
// owned by the conclude key derived from ConcludeAddress.
func (c *Client) CreateOrder(ctx context.Context, params CreateOrderParams) (*OrderCreated, error) {
	var result OrderCreated
	if err := c.call(ctx, "order_create", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ConcludeOrder closes an order, transferring its remaining funds to the
// OutputAddress (or a fresh derived address when nil).
func (c *Client) ConcludeOrder(ctx context.Context, params ConcludeOrderParams) (*SendResult, error) {
	var result SendResult
	if err := c.call(ctx, "order_conclude", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// FillOrder fills an existing order partially (or fully) by FillAmount in
// the order's ask currency; proceeds go to the OutputAddress.
func (c *Client) FillOrder(ctx context.Context, params FillOrderParams) (*SendResult, error) {
	var result SendResult
	if err := c.call(ctx, "order_fill", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// FreezeOrder prevents further fills of an order. Frozen orders can only be
// concluded afterwards.
func (c *Client) FreezeOrder(ctx context.Context, params FreezeOrderParams) (*SendResult, error) {
	var result SendResult
	if err := c.call(ctx, "order_freeze", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListOwnOrders returns orders whose conclude key is owned by the account,
// including their live balances and wallet-side flags.
func (c *Client) ListOwnOrders(ctx context.Context, account uint32) ([]OwnOrder, error) {
	params := struct {
		Account uint32 `json:"account"`
	}{Account: account}
	var result []OwnOrder
	if err := c.call(ctx, "order_list_own", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListAllActiveOrders returns all active orders matching the currency pair.
// A nil filter matches any currency on that side.
func (c *Client) ListAllActiveOrders(ctx context.Context, params ListOrdersParams) ([]ActiveOrder, error) {
	var result []ActiveOrder
	if err := c.call(ctx, "order_list_all_active", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}
