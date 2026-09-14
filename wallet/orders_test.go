// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package wallet_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mintlayer/go-sdk/wallet"
)

// capturedRequest records the last JSON-RPC method and params seen by the
// test server, so tests can assert the exact wire shape.
type capturedRequest struct {
	Method string
	Params json.RawMessage
}

// captureHandler returns an httptest.Server that records each request's
// method and params, and replies with the given JSON result.
func captureHandler(t *testing.T, result any, captured *capturedRequest) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      uint64          `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		captured.Method = req.Method
		captured.Params = req.Params

		raw, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("marshal result: %v", err)
		}
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  json.RawMessage(raw),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// mustJSON marshals v or fails the test.
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	return string(b)
}

// mustTokenFilter builds a TokenFilter or fails the test.
func mustTokenFilter(t *testing.T, tokenID string) *wallet.CurrencyFilter {
	t.Helper()
	f, err := wallet.TokenFilter(tokenID)
	if err != nil {
		t.Fatalf("TokenFilter(%q): %v", tokenID, err)
	}
	return f
}

// --- OutputValue marshaling / unmarshaling ---

func TestOutputValue_MarshalCoin(t *testing.T) {
	v := wallet.OutputValue{Coin: true, Amount: wallet.Amount{Atoms: "1000000000000"}}
	want := `{"type":"Coin","content":{"amount":{"atoms":"1000000000000"}}}`
	if got := mustJSON(t, v); got != want {
		t.Errorf("coin wire shape:\n got  %s\n want %s", got, want)
	}
}

func TestOutputValue_MarshalToken(t *testing.T) {
	v := wallet.OutputValue{TokenID: "tok1abc", Amount: wallet.Amount{Atoms: "250000000"}}
	want := `{"type":"Token","content":{"id":"tok1abc","amount":{"atoms":"250000000"}}}`
	if got := mustJSON(t, v); got != want {
		t.Errorf("token wire shape:\n got  %s\n want %s", got, want)
	}
}

func TestOutputValue_MarshalTokenWithoutIDErrors(t *testing.T) {
	v := wallet.OutputValue{Amount: wallet.Amount{Atoms: "1"}}
	b, err := json.Marshal(v)
	if err == nil {
		t.Fatalf("expected error marshaling token OutputValue without TokenID, got %s", b)
	}
}

func TestOutputValue_UnmarshalCoin(t *testing.T) {
	raw := []byte(`{"type":"Coin","content":{"amount":{"atoms":"700","decimal":"0.0000007"}}}`)
	var v wallet.OutputValue
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !v.Coin {
		t.Error("expected Coin=true")
	}
	if v.TokenID != "" {
		t.Errorf("expected empty TokenID, got %q", v.TokenID)
	}
	if v.Amount.Atoms != "700" || v.Amount.Decimal != "0.0000007" {
		t.Errorf("unexpected amount: %+v", v.Amount)
	}
}

func TestOutputValue_UnmarshalToken(t *testing.T) {
	raw := []byte(`{"type":"Token","content":{"id":"tok1xyz","amount":{"atoms":"42"}}}`)
	var v wallet.OutputValue
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v.Coin {
		t.Error("expected Coin=false for token")
	}
	if v.TokenID != "tok1xyz" {
		t.Errorf("unexpected token id: %q", v.TokenID)
	}
	if v.Amount.Atoms != "42" {
		t.Errorf("unexpected amount: %+v", v.Amount)
	}
}

// TestOutputValue_UnmarshalMissingAmountErrors pins that decoding an
// OutputValue with a missing, null, or empty amount fails on both the Coin
// and Token branches.
func TestOutputValue_UnmarshalMissingAmountErrors(t *testing.T) {
	cases := map[string]string{
		"coin missing amount":  `{"type":"Coin"}`,
		"coin null amount":     `{"type":"Coin","content":{"amount":null}}`,
		"coin empty amount":    `{"type":"Coin","content":{"amount":{}}}`,
		"coin empty atoms":     `{"type":"Coin","content":{"amount":{"atoms":""}}}`,
		"token missing amount": `{"type":"Token","content":{"id":"tok1noamount"}}`,
		"token null amount":    `{"type":"Token","content":{"id":"tok1noamount","amount":null}}`,
		"token empty amount":   `{"type":"Token","content":{"id":"tok1noamount","amount":{}}}`,
	}
	for name, raw := range cases {
		var v wallet.OutputValue
		err := json.Unmarshal([]byte(raw), &v)
		if err == nil {
			t.Errorf("%s: expected error, got nil (decoded %+v)", name, v)
			continue
		}
		if want := "requires an amount"; !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error %q does not contain %q", name, err, want)
		}
	}
}

func TestOutputValue_UnmarshalUnknownTypeErrors(t *testing.T) {
	raw := []byte(`{"type":"NFT","content":{"amount":{"atoms":"1"}}}`)
	var v wallet.OutputValue
	if err := json.Unmarshal(raw, &v); err == nil {
		t.Fatal("expected error for unknown output value type, got nil")
	}
}

func TestOutputValue_UnmarshalMalformedJSONErrors(t *testing.T) {
	// Complete JSON with a wrong-typed field: the outer scan succeeds, so
	// OutputValue.UnmarshalJSON runs and its inner unmarshal errors.
	var v wallet.OutputValue
	if err := json.Unmarshal([]byte(`{"type":123,"content":{}}`), &v); err == nil {
		t.Fatal("expected error for wrong-typed field, got nil")
	}
}

// TestOutputValue_MarshalCoinWithoutAmountErrors pins the marshal-side amount
// requirement on the Coin branch: a coin OutputValue with an empty Amount
// (both Atoms and Decimal "") must fail before any JSON is produced.
func TestOutputValue_MarshalCoinWithoutAmountErrors(t *testing.T) {
	v := wallet.OutputValue{Coin: true}
	b, err := json.Marshal(v)
	if err == nil {
		t.Fatalf("expected error marshaling coin OutputValue without amount, got %s", b)
	}
	if want := "requires an amount"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err, want)
	}
}

// TestOutputValue_MarshalTokenWithoutAmountErrors pins the same requirement on
// the Token branch, ordered before the TokenID check in the wire shape.
func TestOutputValue_MarshalTokenWithoutAmountErrors(t *testing.T) {
	v := wallet.OutputValue{TokenID: "tok1noamount"}
	if b, err := json.Marshal(v); err == nil {
		t.Fatalf("expected error marshaling token OutputValue without amount, got %s", b)
	}
}

// TestOutputValue_UnmarshalCoinKeepsBothAmountFields pins that a Coin amount
// carrying both atoms and decimal survives unmarshaling with both fields
// populated (the empty-amount validation must not drop either).
func TestOutputValue_UnmarshalCoinKeepsBothAmountFields(t *testing.T) {
	raw := []byte(`{"type":"Coin","content":{"amount":{"atoms":"700","decimal":"0.0000007"}}}`)
	var v wallet.OutputValue
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v.Amount.Atoms != "700" {
		t.Errorf("expected atoms kept, got %q", v.Amount.Atoms)
	}
	if v.Amount.Decimal != "0.0000007" {
		t.Errorf("expected decimal kept, got %q", v.Amount.Decimal)
	}
}

// TestOutputValue_UnmarshalTokenWithoutIDErrors pins the unmarshal-side TokenID
// requirement (previously enforced only when marshaling).
func TestOutputValue_UnmarshalTokenWithoutIDErrors(t *testing.T) {
	raw := []byte(`{"type":"Token","content":{"amount":{"atoms":"1"}}}`)
	var v wallet.OutputValue
	if err := json.Unmarshal(raw, &v); err == nil {
		t.Fatalf("expected error for token without id, got %+v", v)
	}
}

func TestOutputValue_RoundTrip(t *testing.T) {
	cases := map[string]wallet.OutputValue{
		"coin":  {Coin: true, Amount: wallet.Amount{Atoms: "1000000000000", Decimal: "10000.0"}},
		"token": {TokenID: "tok1roundtrip", Amount: wallet.Amount{Atoms: "999", Decimal: "0.000000999"}},
	}
	for name, want := range cases {
		b, err := json.Marshal(want)
		if err != nil {
			t.Fatalf("%s: marshal: %v", name, err)
		}
		var got wallet.OutputValue
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatalf("%s: unmarshal: %v", name, err)
		}
		if got != want {
			t.Errorf("%s: round trip:\n got  %+v\n want %+v", name, got, want)
		}
	}
}

// --- CreateOrder ---

// createOrderWire mirrors the CreateOrderParams wire shape, keeping the
// OutputValue sides as raw JSON so the exact encoded form can be asserted.
type createOrderWire struct {
	Account         uint32          `json:"account"`
	Ask             json.RawMessage `json:"ask"`
	Give            json.RawMessage `json:"give"`
	ConcludeAddress string          `json:"conclude_address"`
}

func TestCreateOrder_WireShape(t *testing.T) {
	result := wallet.OrderCreated{OrderID: "ord1created", TxID: "tx1created", Broadcasted: true}
	var captured capturedRequest
	srv := captureHandler(t, result, &captured)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.CreateOrder(context.Background(), wallet.CreateOrderParams{
		Account:         0,
		Ask:             wallet.OutputValue{Coin: true, Amount: wallet.Amount{Atoms: "1000000000000"}},
		Give:            wallet.OutputValue{TokenID: "tok1giveabc", Amount: wallet.Amount{Atoms: "250000000"}},
		ConcludeAddress: "tmltool1conclude",
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}

	if captured.Method != "order_create" {
		t.Errorf("expected method order_create, got %q", captured.Method)
	}

	var wire createOrderWire
	if err := json.Unmarshal(captured.Params, &wire); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if wire.Account != 0 {
		t.Errorf("expected account 0, got %d", wire.Account)
	}

	wantAsk := `{"type":"Coin","content":{"amount":{"atoms":"1000000000000"}}}`
	if string(wire.Ask) != wantAsk {
		t.Errorf("ask wire shape:\n got  %s\n want %s", wire.Ask, wantAsk)
	}
	wantGive := `{"type":"Token","content":{"id":"tok1giveabc","amount":{"atoms":"250000000"}}}`
	if string(wire.Give) != wantGive {
		t.Errorf("give wire shape:\n got  %s\n want %s", wire.Give, wantGive)
	}
	if wire.ConcludeAddress != "tmltool1conclude" {
		t.Errorf("expected conclude_address tmltool1conclude, got %q", wire.ConcludeAddress)
	}

	// Result decoding.
	if got.OrderID != "ord1created" {
		t.Errorf("unexpected order_id: %q", got.OrderID)
	}
	if got.TxID != "tx1created" {
		t.Errorf("unexpected tx_id: %q", got.TxID)
	}
	if !got.Broadcasted {
		t.Error("expected broadcasted=true")
	}
}

func TestCreateOrder_TokenSideWithoutIDFailsBeforeSend(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		resp := map[string]any{"jsonrpc": "2.0", "id": 1, "result": nil}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := wallet.New(srv.URL)
	_, err := c.CreateOrder(context.Background(), wallet.CreateOrderParams{
		Account: 0,
		Ask:     wallet.OutputValue{Coin: true, Amount: wallet.Amount{Atoms: "1"}},
		Give:    wallet.OutputValue{Amount: wallet.Amount{Atoms: "2"}}, // no TokenID
	})
	if err == nil {
		t.Fatal("expected error for token side without TokenID, got nil")
	}
	if requests != 0 {
		t.Errorf("expected no request to reach the server, got %d", requests)
	}
}

// --- ConcludeOrder / FillOrder / FreezeOrder ---

type orderActionWire struct {
	Account       uint32          `json:"account"`
	OrderID       string          `json:"order_id"`
	OutputAddress *string         `json:"output_address"`
	FillAmount    json.RawMessage `json:"fill_amount_in_ask_currency"`
}

func TestConcludeOrder_WireShape(t *testing.T) {
	result := wallet.SendResult{TxID: "tx1conclude", Broadcasted: true}
	var captured capturedRequest
	srv := captureHandler(t, result, &captured)
	defer srv.Close()

	outAddr := "tmltool1remainder"
	c := wallet.New(srv.URL)
	got, err := c.ConcludeOrder(context.Background(), wallet.ConcludeOrderParams{
		Account:       0,
		OrderID:       "ord1conclude",
		OutputAddress: &outAddr,
	})
	if err != nil {
		t.Fatalf("ConcludeOrder: %v", err)
	}

	if captured.Method != "order_conclude" {
		t.Errorf("expected method order_conclude, got %q", captured.Method)
	}
	var wire orderActionWire
	if err := json.Unmarshal(captured.Params, &wire); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if wire.OrderID != "ord1conclude" {
		t.Errorf("unexpected order_id: %q", wire.OrderID)
	}
	if wire.OutputAddress == nil || *wire.OutputAddress != "tmltool1remainder" {
		t.Errorf("expected output_address tmltool1remainder, got %v", wire.OutputAddress)
	}
	if got.TxID != "tx1conclude" || !got.Broadcasted {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestConcludeOrder_NilOutputAddressStaysNull(t *testing.T) {
	var captured capturedRequest
	srv := captureHandler(t, wallet.SendResult{TxID: "tx1c2", Broadcasted: false}, &captured)
	defer srv.Close()

	c := wallet.New(srv.URL)
	if _, err := c.ConcludeOrder(context.Background(), wallet.ConcludeOrderParams{
		Account: 0,
		OrderID: "ord1c2",
	}); err != nil {
		t.Fatalf("ConcludeOrder: %v", err)
	}

	var wire orderActionWire
	if err := json.Unmarshal(captured.Params, &wire); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if wire.OutputAddress != nil {
		t.Errorf("expected output_address null, got %q", *wire.OutputAddress)
	}
	if want := `"output_address":null`; !strings.Contains(string(captured.Params), want) {
		t.Errorf("expected %s in params, got %s", want, captured.Params)
	}
}

func TestFillOrder_WireShape(t *testing.T) {
	result := wallet.SendResult{TxID: "tx1fill", Broadcasted: true}
	var captured capturedRequest
	srv := captureHandler(t, result, &captured)
	defer srv.Close()

	outAddr := "tmltool1filldest"
	c := wallet.New(srv.URL)
	got, err := c.FillOrder(context.Background(), wallet.FillOrderParams{
		Account:       0,
		OrderID:       "ord1fill",
		FillAmount:    wallet.Amount{Atoms: "100000000", Decimal: "1.0"},
		OutputAddress: &outAddr,
	})
	if err != nil {
		t.Fatalf("FillOrder: %v", err)
	}

	if captured.Method != "order_fill" {
		t.Errorf("expected method order_fill, got %q", captured.Method)
	}
	var wire orderActionWire
	if err := json.Unmarshal(captured.Params, &wire); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	wantFill := `{"atoms":"100000000","decimal":"1.0"}`
	if string(wire.FillAmount) != wantFill {
		t.Errorf("fill_amount_in_ask_currency:\n got  %s\n want %s", wire.FillAmount, wantFill)
	}
	if wire.OrderID != "ord1fill" {
		t.Errorf("unexpected order_id: %q", wire.OrderID)
	}
	if wire.OutputAddress == nil || *wire.OutputAddress != "tmltool1filldest" {
		t.Errorf("expected output_address tmltool1filldest, got %v", wire.OutputAddress)
	}
	if got.TxID != "tx1fill" {
		t.Errorf("unexpected tx_id: %q", got.TxID)
	}
}

func TestFreezeOrder_WireShape(t *testing.T) {
	result := wallet.SendResult{TxID: "tx1freeze", Broadcasted: true}
	var captured capturedRequest
	srv := captureHandler(t, result, &captured)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.FreezeOrder(context.Background(), wallet.FreezeOrderParams{
		Account: 0,
		OrderID: "ord1freeze",
	})
	if err != nil {
		t.Fatalf("FreezeOrder: %v", err)
	}

	if captured.Method != "order_freeze" {
		t.Errorf("expected method order_freeze, got %q", captured.Method)
	}
	var wire orderActionWire
	if err := json.Unmarshal(captured.Params, &wire); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if wire.OrderID != "ord1freeze" {
		t.Errorf("unexpected order_id: %q", wire.OrderID)
	}
	if got.TxID != "tx1freeze" {
		t.Errorf("unexpected tx_id: %q", got.TxID)
	}
}

// --- TokenFilter ---

// TestTokenFilter_ReturnsFilter pins the (tokenID string) (*CurrencyFilter,
// error) signature: a valid id yields a Token filter carrying the id.
func TestTokenFilter_ReturnsFilter(t *testing.T) {
	f := mustTokenFilter(t, "tok1filterabc")
	if f == nil {
		t.Fatal("expected non-nil filter")
	}
	if f.Type != "Token" {
		t.Errorf("expected type Token, got %q", f.Type)
	}
	if f.Content != "tok1filterabc" {
		t.Errorf("expected content tok1filterabc, got %q", f.Content)
	}
}

// TestTokenFilter_EmptyTokenIDErrors pins that an empty id is rejected instead
// of silently matching nothing; CoinFilter is the native-coin path.
func TestTokenFilter_EmptyTokenIDErrors(t *testing.T) {
	f, err := wallet.TokenFilter("")
	if err == nil {
		t.Fatalf("expected error for empty token id, got filter %+v", f)
	}
	if f != nil {
		t.Errorf("expected nil filter on error, got %+v", f)
	}
	if want := "requires a token id"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err, want)
	}
}

// TestTokenFilter_WireShape pins that the filter returned by the new signature
// still encodes the id as content on the wire.
func TestTokenFilter_WireShape(t *testing.T) {
	want := `{"type":"Token","content":"tok1wireabc"}`
	if got := mustJSON(t, mustTokenFilter(t, "tok1wireabc")); got != want {
		t.Errorf("token filter wire shape:\n got  %s\n want %s", got, want)
	}
}

// --- ListOwnOrders ---

func TestListOwnOrders_Decoding(t *testing.T) {
	listing := json.RawMessage(`[
		{
			"order_id": "ord1owned",
			"initially_asked": {
				"type": "Token",
				"content": {"id": "tok1askedabc", "amount": {"atoms": "100000000", "decimal": "1.0"}}
			},
			"initially_given": {
				"type": "Coin",
				"content": {"amount": {"atoms": "5000000000", "decimal": "50.0"}}
			},
			"existing_order_data": {
				"ask_balance": {"atoms": "90000000", "decimal": "0.9"},
				"give_balance": {"atoms": "4500000000", "decimal": "45.0"},
				"is_frozen": true,
				"creation_timestamp": {"timestamp": 1700000000}
			},
			"is_marked_as_frozen_in_wallet": true,
			"is_marked_as_concluded_in_wallet": false
		},
		{
			"order_id": "ord2owned",
			"initially_asked": {
				"type": "Coin",
				"content": {"amount": {"atoms": "10"}}
			},
			"initially_given": {
				"type": "Token",
				"content": {"id": "tok2given", "amount": {"atoms": "20"}}
			},
			"existing_order_data": null,
			"is_marked_as_frozen_in_wallet": false,
			"is_marked_as_concluded_in_wallet": true
		}
	]`)
	var captured capturedRequest
	srv := captureHandler(t, listing, &captured)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.ListOwnOrders(context.Background(), 0)
	if err != nil {
		t.Fatalf("ListOwnOrders: %v", err)
	}

	if captured.Method != "order_list_own" {
		t.Errorf("expected method order_list_own, got %q", captured.Method)
	}
	var wire struct {
		Account uint32 `json:"account"`
	}
	if err := json.Unmarshal(captured.Params, &wire); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if wire.Account != 0 {
		t.Errorf("expected account 0, got %d", wire.Account)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(got))
	}

	first := got[0]
	if first.OrderID != "ord1owned" {
		t.Errorf("unexpected order_id: %q", first.OrderID)
	}
	// initially_asked: Token with id + amount carrying both atoms and decimal.
	if first.InitiallyAsked.Coin {
		t.Error("expected initially_asked to be a token")
	}
	if first.InitiallyAsked.TokenID != "tok1askedabc" {
		t.Errorf("unexpected asked token id: %q", first.InitiallyAsked.TokenID)
	}
	if first.InitiallyAsked.Amount.Atoms != "100000000" || first.InitiallyAsked.Amount.Decimal != "1.0" {
		t.Errorf("unexpected asked amount: %+v", first.InitiallyAsked.Amount)
	}
	// initially_given: Coin.
	if !first.InitiallyGiven.Coin {
		t.Error("expected initially_given to be a coin")
	}
	if first.InitiallyGiven.TokenID != "" {
		t.Errorf("expected empty token id for coin, got %q", first.InitiallyGiven.TokenID)
	}
	if first.InitiallyGiven.Amount.Atoms != "5000000000" {
		t.Errorf("unexpected given amount: %+v", first.InitiallyGiven.Amount)
	}
	// existing_order_data.
	if first.Existing == nil {
		t.Fatal("expected existing_order_data, got nil")
	}
	if first.Existing.AskBalance.Atoms != "90000000" {
		t.Errorf("unexpected ask_balance: %+v", first.Existing.AskBalance)
	}
	if first.Existing.GiveBalance.Atoms != "4500000000" {
		t.Errorf("unexpected give_balance: %+v", first.Existing.GiveBalance)
	}
	if !first.Existing.Frozen {
		t.Error("expected is_frozen=true")
	}
	if first.Existing.Creation.Timestamp != 1700000000 {
		t.Errorf("unexpected creation timestamp: %d", first.Existing.Creation.Timestamp)
	}
	if !first.MarkedFrozen {
		t.Error("expected is_marked_as_frozen_in_wallet=true")
	}
	if first.MarkedConcluded {
		t.Error("expected is_marked_as_concluded_in_wallet=false")
	}

	second := got[1]
	if second.OrderID != "ord2owned" {
		t.Errorf("unexpected order_id: %q", second.OrderID)
	}
	if second.Existing != nil {
		t.Errorf("expected null existing_order_data to decode as nil, got %+v", second.Existing)
	}
	if second.MarkedConcluded != true {
		t.Error("expected second order is_marked_as_concluded_in_wallet=true")
	}
}

// --- ListAllActiveOrders ---

type listOrdersWire struct {
	Account      uint32          `json:"account"`
	AskCurrency  json.RawMessage `json:"ask_currency"`
	GiveCurrency json.RawMessage `json:"give_currency"`
}

func TestListAllActiveOrders_WithFilters(t *testing.T) {
	result := json.RawMessage(`[
		{
			"order_id": "ord1active",
			"initially_asked": {"type": "Coin", "content": {"amount": {"atoms": "1000000000000"}}},
			"initially_given": {"type": "Token", "content": {"id": "tok1givenabc", "amount": {"atoms": "2500000000"}}},
			"ask_balance": {"atoms": "1000000000000"},
			"give_balance": {"atoms": "2500000000"},
			"is_own": true
		}
	]`)
	var captured capturedRequest
	srv := captureHandler(t, result, &captured)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.ListAllActiveOrders(context.Background(), wallet.ListOrdersParams{
		Account:      0,
		AskCurrency:  wallet.CoinFilter(),
		GiveCurrency: mustTokenFilter(t, "tok1givenabc"),
	})
	if err != nil {
		t.Fatalf("ListAllActiveOrders: %v", err)
	}

	if captured.Method != "order_list_all_active" {
		t.Errorf("expected method order_list_all_active, got %q", captured.Method)
	}
	var wire listOrdersWire
	if err := json.Unmarshal(captured.Params, &wire); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	// Coin filters carry no content on the wire (daemon-verified).
	wantAsk := `{"type":"Coin"}`
	if string(wire.AskCurrency) != wantAsk {
		t.Errorf("ask_currency filter:\n got  %s\n want %s", wire.AskCurrency, wantAsk)
	}
	wantGive := `{"type":"Token","content":"tok1givenabc"}`
	if string(wire.GiveCurrency) != wantGive {
		t.Errorf("give_currency filter:\n got  %s\n want %s", wire.GiveCurrency, wantGive)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 active order, got %d", len(got))
	}
	order := got[0]
	if order.OrderID != "ord1active" {
		t.Errorf("unexpected order_id: %q", order.OrderID)
	}
	if !order.InitiallyAsked.Coin {
		t.Error("expected initially_asked to be a coin")
	}
	if order.InitiallyGiven.TokenID != "tok1givenabc" {
		t.Errorf("unexpected given token id: %q", order.InitiallyGiven.TokenID)
	}
	if order.AskBalance.Atoms != "1000000000000" {
		t.Errorf("unexpected ask_balance: %+v", order.AskBalance)
	}
	if order.GiveBalance.Atoms != "2500000000" {
		t.Errorf("unexpected give_balance: %+v", order.GiveBalance)
	}
	if !order.IsOwn {
		t.Error("expected is_own=true")
	}
}

func TestListAllActiveOrders_NilFilters(t *testing.T) {
	var captured capturedRequest
	srv := captureHandler(t, json.RawMessage(`[]`), &captured)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.ListAllActiveOrders(context.Background(), wallet.ListOrdersParams{Account: 3})
	if err != nil {
		t.Fatalf("ListAllActiveOrders: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty listing, got %d entries", len(got))
	}

	var wire listOrdersWire
	if err := json.Unmarshal(captured.Params, &wire); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if wire.Account != 3 {
		t.Errorf("expected account 3, got %d", wire.Account)
	}
	// A nil filter is encoded as JSON null (RawMessage is non-nil even for null).
	if string(wire.AskCurrency) != "null" {
		t.Errorf("expected null ask_currency, got %s", wire.AskCurrency)
	}
	if string(wire.GiveCurrency) != "null" {
		t.Errorf("expected null give_currency, got %s", wire.GiveCurrency)
	}
}

// --- Error propagation ---

func TestOrderMethods_RPCErrorPropagates(t *testing.T) {
	srv := rpcErrorHandler(t, -32000, "order failure")
	defer srv.Close()

	c := wallet.New(srv.URL)
	outAddr := "tmltool1dest"

	t.Run("CreateOrder", func(t *testing.T) {
		got, err := c.CreateOrder(context.Background(), wallet.CreateOrderParams{
			Ask:  wallet.OutputValue{Coin: true, Amount: wallet.Amount{Atoms: "1"}},
			Give: wallet.OutputValue{Coin: true, Amount: wallet.Amount{Atoms: "2"}},
		})
		assertRPCError(t, err)
		if got != nil {
			t.Errorf("expected nil result, got %+v", got)
		}
	})
	t.Run("ConcludeOrder", func(t *testing.T) {
		got, err := c.ConcludeOrder(context.Background(), wallet.ConcludeOrderParams{OrderID: "ord1", OutputAddress: &outAddr})
		assertRPCError(t, err)
		if got != nil {
			t.Errorf("expected nil result, got %+v", got)
		}
	})
	t.Run("FillOrder", func(t *testing.T) {
		got, err := c.FillOrder(context.Background(), wallet.FillOrderParams{OrderID: "ord1", FillAmount: wallet.Amount{Atoms: "1"}, OutputAddress: &outAddr})
		assertRPCError(t, err)
		if got != nil {
			t.Errorf("expected nil result, got %+v", got)
		}
	})
	t.Run("FreezeOrder", func(t *testing.T) {
		got, err := c.FreezeOrder(context.Background(), wallet.FreezeOrderParams{OrderID: "ord1"})
		assertRPCError(t, err)
		if got != nil {
			t.Errorf("expected nil result, got %+v", got)
		}
	})
	t.Run("ListOwnOrders", func(t *testing.T) {
		got, err := c.ListOwnOrders(context.Background(), 0)
		assertRPCError(t, err)
		if got != nil {
			t.Errorf("expected nil result, got %+v", got)
		}
	})
	t.Run("ListAllActiveOrders", func(t *testing.T) {
		got, err := c.ListAllActiveOrders(context.Background(), wallet.ListOrdersParams{})
		assertRPCError(t, err)
		if got != nil {
			t.Errorf("expected nil result, got %+v", got)
		}
	})
}

func assertRPCError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, ok := err.(*wallet.RPCError); !ok {
		t.Fatalf("expected *wallet.RPCError, got %T: %v", err, err)
	}
}
