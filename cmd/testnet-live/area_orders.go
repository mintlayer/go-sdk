// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/mintlayer/go-sdk/wallet"
	mintlayer "github.com/mintlayer/go-sdk/wasm"
)

// Order fixture (all amounts in atoms):
//
//	give: 100 demo tokens (escrowed by the maker = account 0)
//	ask:  0.01 ML coins (paid by the filler = counterparty account)
//
// The counterparty fills half (0.005 ML) and receives 50 tokens, leaving the
// order with give=50 tokens + ask=0.005 ML accumulated for freeze/conclude.
const (
	orderGiveTokens  = "100000000"  // 100 tokens @ 6 decimals
	orderAskCoins    = "1000000000" // 0.01 ML
	orderFillCoins   = "500000000"  // 0.005 ML
	orderPostGiveTok = "50000000"   // 50 tokens remaining after the half fill
	orderPostAsk     = "500000000"  // 0.005 ML accumulated
)

// runOrdersArea runs the full sacrificial order lifecycle.
func runOrdersArea(ctx context.Context, r *Runner, h *harness, timeout time.Duration) {
	const area = "orders"
	if h.TokenID == "" {
		r.Record(area, "order lifecycle", StatusSkip, "no token provisioned (tokens area failed)")
		return
	}
	wc := h.wc
	w := h.wasm

	concludeAddr, err := wc.NewAddress(ctx, 0)
	if err != nil {
		r.Record(area, "conclude address", StatusFail, err.Error())
		return
	}

	// ── 1. create ────────────────────────────────────────────────────────────────
	if r.OverBudget("25000000000") {
		r.Record(area, "CreateOrder", StatusSkip, "fee budget exceeded")
		return
	}
	created, err := wc.CreateOrder(ctx, wallet.CreateOrderParams{
		Account:         0,
		Ask:             wallet.OutputValue{Coin: true, Amount: wallet.Amount{Atoms: orderAskCoins}},
		Give:            wallet.OutputValue{Coin: false, TokenID: h.TokenID, Amount: wallet.Amount{Atoms: orderGiveTokens}},
		ConcludeAddress: concludeAddr,
	})
	if err != nil {
		r.Record(area, "CreateOrder", StatusFail, err.Error())
		return
	}
	h.OrderID, h.OrderTxID = created.OrderID, created.TxID
	r.Record(area, "CreateOrder(100 tokens ⇄ 0.01 ML)", StatusPass,
		fmt.Sprintf("order=%s tx=%s broadcasted=%v", short(created.OrderID), short(created.TxID), created.Broadcasted))
	r.Finding("wallet.OrderCreated drops the daemon's `fees` and `tx` (hex) fields (NewOrderTransaction has both) — order-create fee spend is untrackable through the SDK")

	if err := confirmTx(ctx, h, created.TxID, timeout); err != nil {
		r.Record(area, "order creation confirmed", StatusFail, err.Error())
		return
	}

	// ── 2. GetOrderId(inputs) prediction vs real order id ─────────────────────────
	rawTx, err := wc.GetTransaction(ctx, 0, created.TxID)
	if err != nil {
		r.Record(area, "GetOrderId prediction", StatusSkip, "wallet has no tx json: "+err.Error())
	} else if inputs, err := parseTxInputs(rawTx); err != nil {
		r.Record(area, "GetOrderId prediction", StatusFail, err.Error())
	} else {
		var encoded []byte
		bad := false
		for _, in := range inputs {
			idBytes, derr := hex.DecodeString(in.Source)
			if derr != nil {
				r.Record(area, "GetOrderId prediction", StatusFail, "bad source hex: "+derr.Error())
				bad = true
				break
			}
			kind := mintlayer.SourceTransaction
			if in.IsReward {
				kind = mintlayer.SourceBlockReward
			}
			srcID, serr := w.EncodeOutpointSourceId(idBytes, kind)
			if serr != nil {
				r.Record(area, "GetOrderId prediction", StatusFail, "EncodeOutpointSourceId: "+serr.Error())
				bad = true
				break
			}
			ib, ierr := w.EncodeInputForUtxo(srcID, in.Index)
			if ierr != nil {
				r.Record(area, "GetOrderId prediction", StatusFail, "EncodeInputForUtxo: "+ierr.Error())
				bad = true
				break
			}
			encoded = append(encoded, ib...)
		}
		if !bad {
			predicted, gerr := w.GetOrderId(encoded, h.Net)
			if gerr != nil {
				r.Record(area, "GetOrderId prediction", StatusFail, gerr.Error())
			} else if predicted != created.OrderID {
				r.Record(area, "GetOrderId prediction", StatusFail,
					fmt.Sprintf("predicted %s != actual %s", predicted, created.OrderID))
			} else {
				r.Record(area, "GetOrderId(inputs) == real order id", StatusPass, predicted)
			}
		}
	}

	// ── 3. on-chain state via the node client ─────────────────────────────────────
	rawOI, _, oerr := rawRPC(ctx, h.cfg.NodeURL, h.cfg.RPCUser, h.cfg.RPCPass,
		"chainstate_order_info", []any{created.OrderID})
	if oerr != nil {
		r.Record(area, "node OrderInfo (raw)", StatusFail, oerr.Error())
	} else {
		var oi struct {
			Nonce      *uint64 `json:"nonce"`
			IsFrozen   bool    `json:"is_frozen"`
			GiveAmount struct {
				Atoms string `json:"atoms"`
			} `json:"give_balance"`
			AskAmount struct {
				Atoms string `json:"atoms"`
			} `json:"ask_balance"`
		}
		if jerr := jsonUnmarshal(rawOI, &oi); jerr != nil {
			r.Record(area, "node OrderInfo (raw)", StatusFail, jerr.Error())
		} else if oi.IsFrozen || oi.GiveAmount.Atoms != orderGiveTokens {
			r.Record(area, "node OrderInfo (raw)", StatusFail,
				"give="+oi.GiveAmount.Atoms+" frozen="+btoa(oi.IsFrozen))
		} else {
			r.Record(area, "node OrderInfo (raw)", StatusPass,
				"give="+oi.GiveAmount.Atoms+" nonce="+ptrStr(oi.Nonce))
		}
	}

	// SDK-typed OrderInfo: active orders carry nonce:null, the SDK types uint64 —
	// recorded as a finding when unmarshalling breaks.
	if _, err := h.nc.OrderInfo(ctx, created.OrderID); err != nil {
		r.Finding("node.OrderInfo unmarshal fails on active orders (daemon sends nonce:null, SDK uses uint64): %v", err)
	}

	// ── 4. listings ───────────────────────────────────────────────────────────────
	own, err := wc.ListOwnOrders(ctx, 0)
	if err != nil {
		r.Record(area, "ListOwnOrders", StatusFail, err.Error())
	} else {
		o := findOwn(own, created.OrderID)
		if o == nil {
			r.Record(area, "ListOwnOrders", StatusFail, "created order missing from listing")
		} else if o.InitiallyGiven.Amount.Atoms != orderGiveTokens || o.InitiallyGiven.TokenID != h.TokenID {
			r.Record(area, "ListOwnOrders", StatusFail, "initially_given mismatch: "+o.InitiallyGiven.Amount.Atoms)
		} else {
			r.Record(area, "ListOwnOrders", StatusPass, fmt.Sprintf("order present, given=%s tokens frozen=%v",
				o.InitiallyGiven.Amount.Atoms, o.MarkedFrozen))
		}
	}

	giveTokFilter, _ := wallet.TokenFilter(h.TokenID)
	active, err := wc.ListAllActiveOrders(ctx, wallet.ListOrdersParams{
		Account: 0, GiveCurrency: giveTokFilter,
	})
	if err != nil {
		r.Record(area, "ListAllActiveOrders(give=token)", StatusFail, err.Error())
	} else if findActive(active, created.OrderID) == nil {
		r.Record(area, "ListAllActiveOrders(give=token)", StatusFail, "new order missing from active list")
	} else {
		r.Record(area, "ListAllActiveOrders(give=token)", StatusPass,
			fmt.Sprintf("%d active token-give orders, ours present (is_own)", len(active)))
	}

	coinFilter := wallet.CoinFilter()
	activeCoin, err := wc.ListAllActiveOrders(ctx, wallet.ListOrdersParams{
		Account: 0, AskCurrency: coinFilter,
	})
	if err != nil {
		r.Record(area, "ListAllActiveOrders(ask=Coin)", StatusFail, err.Error())
	} else if findActive(activeCoin, created.OrderID) == nil {
		r.Record(area, "ListAllActiveOrders(ask=Coin)", StatusFail, "coin-ask order missing")
	} else {
		r.Record(area, "ListAllActiveOrders(ask=Coin)", StatusPass, itoa(len(activeCoin))+" active coin-ask orders")
	}

	// ── 5. fill from the counterparty account ──────────────────────────────────────
	if r.OverBudget("25000000000") {
		r.Record(area, "FillOrder", StatusSkip, "fee budget exceeded")
		return
	}
	// The daemon submits to the node internally; transient rejections (mempool
	// contention with the concurrently running market-maker bot) were observed,
	// so one retry is applied and full errors are kept for the report.
	var fill *wallet.SendResult
	var fillErr error
	for attempt := 0; attempt < 2; attempt++ {
		fill, fillErr = wc.FillOrder(ctx, wallet.FillOrderParams{
			Account:    h.CounterpartyAccount,
			OrderID:    created.OrderID,
			FillAmount: wallet.Amount{Atoms: orderFillCoins},
		})
		if fillErr == nil {
			break
		}
		time.Sleep(3 * time.Second)
	}
	if fillErr != nil {
		r.Record(area, "FillOrder(from account 1)", StatusFail, fillErr.Error())
		return
	}
	r.RecordFee(area, "FillOrder(from account 1)", StatusPass,
		"tx="+short(fill.TxID), fill.Fees.Coins.Atoms)

	// the fill tx is immediately pending in the filler wallet
	if pend, err := wc.ListPendingTransactions(ctx, h.CounterpartyAccount); err != nil {
		r.Record(area, "ListPendingTransactions(filler)", StatusFail, err.Error())
	} else if !containsStr(pend, fill.TxID) {
		r.Record(area, "ListPendingTransactions(filler)", StatusFail, "fill tx not pending")
	} else {
		r.Record(area, "ListPendingTransactions(filler)", StatusPass, "fill tx listed pre-confirmation")
	}

	if err := confirmTx(ctx, h, fill.TxID, timeout); err != nil {
		r.Record(area, "fill confirmed", StatusFail, err.Error())
		return
	}

	// counterparty received give-side tokens
	bal, err := wc.GetBalance(ctx, h.CounterpartyAccount)
	if err != nil {
		r.Record(area, "filler token balance", StatusFail, err.Error())
	} else if got, ok := bal.Tokens[h.TokenID]; !ok {
		r.Record(area, "filler token balance", StatusFail, "no tokens received by the filler")
	} else {
		r.Record(area, "filler token balance", StatusPass, got.Atoms+" filler tokens (~50 received)")
	}

	// ── 6. freeze ─────────────────────────────────────────────────────────────────
	if r.OverBudget("25000000000") {
		r.Record(area, "FreezeOrder", StatusSkip, "fee budget exceeded")
		return
	}
	frz, err := wc.FreezeOrder(ctx, wallet.FreezeOrderParams{Account: 0, OrderID: created.OrderID})
	if err != nil {
		r.Record(area, "FreezeOrder", StatusFail, err.Error())
		return
	}
	r.RecordFee(area, "FreezeOrder", StatusPass, "tx="+short(frz.TxID), frz.Fees.Coins.Atoms)
	confirmTx(ctx, h, frz.TxID, timeout)

	if rawOI, _, oerr := rawRPC(ctx, h.cfg.NodeURL, h.cfg.RPCUser, h.cfg.RPCPass,
		"chainstate_order_info", []any{created.OrderID}); oerr == nil {
		if strings.Contains(string(rawOI), `"is_frozen":true`) {
			r.Record(area, "order frozen on chain", StatusPass, "is_frozen=true")
		} else {
			r.Record(area, "order frozen on chain", StatusFail, string(rawOI))
		}
	}

	if active, err := wc.ListAllActiveOrders(ctx, wallet.ListOrdersParams{
		Account: 0, GiveCurrency: giveTokFilter,
	}); err != nil {
		r.Record(area, "active list after freeze", StatusFail, err.Error())
	} else if findActive(active, created.OrderID) != nil {
		r.Record(area, "active list after freeze", StatusFail, "frozen order still listed as active")
	} else {
		r.Record(area, "active list after freeze", StatusPass, "frozen order excluded")
	}

	// ── 7. conclude ───────────────────────────────────────────────────────────────
	if r.OverBudget("25000000000") {
		r.Record(area, "ConcludeOrder", StatusSkip, "fee budget exceeded")
		return
	}
	conc, err := wc.ConcludeOrder(ctx, wallet.ConcludeOrderParams{Account: 0, OrderID: created.OrderID})
	if err != nil {
		r.Record(area, "ConcludeOrder", StatusFail, err.Error())
		return
	}
	r.RecordFee(area, "ConcludeOrder", StatusPass, "tx="+short(conc.TxID), conc.Fees.Coins.Atoms)
	confirmTx(ctx, h, conc.TxID, timeout)

	own, err = wc.ListOwnOrders(ctx, 0)
	if err != nil {
		r.Record(area, "own orders after conclude", StatusFail, err.Error())
	} else if o := findOwn(own, created.OrderID); o == nil {
		r.Record(area, "own orders after conclude", StatusPass, "order removed from own list")
	} else if !o.MarkedConcluded {
		r.Record(area, "own orders after conclude", StatusFail, "order still present but not flagged concluded")
	} else {
		r.Record(area, "own orders after conclude", StatusPass, "flagged is_marked_as_concluded_in_wallet")
	}
}

func findOwn(orders []wallet.OwnOrder, id string) *wallet.OwnOrder {
	for i := range orders {
		if orders[i].OrderID == id {
			return &orders[i]
		}
	}
	return nil
}

func findActive(orders []wallet.ActiveOrder, id string) *wallet.ActiveOrder {
	for i := range orders {
		if orders[i].OrderID == id {
			return &orders[i]
		}
	}
	return nil
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func ptrStr(p *uint64) string {
	if p == nil {
		return "null"
	}
	return uitoa(*p)
}
