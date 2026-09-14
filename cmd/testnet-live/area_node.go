// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/mintlayer/go-sdk/node"
)

// runNodeArea exercises the node JSON-RPC client against the live daemon.
func runNodeArea(ctx context.Context, r *Runner, h *harness) {
	const area = "node"
	nc := h.nc

	// -- version / chainstate -------------------------------------------------
	if v, err := nc.NodeVersion(ctx); err != nil {
		r.Record(area, "NodeVersion", StatusFail, err.Error())
	} else if v == "" {
		r.Record(area, "NodeVersion", StatusFail, "empty version string")
	} else {
		r.Record(area, "NodeVersion", StatusPass, "v"+v)
	}

	info, err := nc.ChainstateInfo(ctx)
	if err != nil {
		r.Record(area, "ChainstateInfo", StatusFail, err.Error())
		info = nil // remaining height checks degrade to skips
	} else if info.BestBlockHeight == 0 || len(info.BestBlockID) != 64 {
		r.Record(area, "ChainstateInfo", StatusFail,
			"implausible chainstate: height=0 or block id length != 64")
	} else {
		r.Record(area, "ChainstateInfo", StatusPass,
			"height="+uitoa(info.BestBlockHeight)+" ibd="+btoa(info.IsInitialBlockDownload))
	}

	height, herr := nc.BestBlockHeight(ctx)
	bid, biderr := nc.BestBlockID(ctx)
	switch {
	case herr != nil:
		r.Record(area, "BestBlockHeight", StatusFail, herr.Error())
	case biderr != nil:
		r.Record(area, "BestBlockHeight", StatusPass, uitoa(height))
		r.Record(area, "BestBlockID", StatusFail, biderr.Error())
	default:
		ok := info == nil || (info.BestBlockHeight == height && info.BestBlockID == bid)
		if !ok {
			r.Record(area, "BestBlockHeight/ID", StatusFail, "inconsistent with ChainstateInfo")
		} else {
			r.Record(area, "BestBlockHeight/ID vs ChainstateInfo", StatusPass, "height="+uitoa(height))
		}
	}

	// -- block queries ---------------------------------------------------------
	if herr == nil && biderr == nil {
		if id, err := nc.BlockIDAtHeight(ctx, height-1); err != nil {
			r.Record(area, "BlockIDAtHeight(tip-1)", StatusFail, err.Error())
		} else if id == nil || *id == "" {
			r.Record(area, "BlockIDAtHeight(tip-1)", StatusFail, "nil id for existing height")
		} else {
			r.Record(area, "BlockIDAtHeight(tip-1)", StatusPass, short(*id))
		}

		if hexBlk, err := nc.GetBlock(ctx, bid); err != nil {
			r.Record(area, "GetBlock(tip)", StatusFail, err.Error())
		} else if hexBlk == nil || len(*hexBlk) == 0 {
			r.Record(area, "GetBlock(tip)", StatusFail, "empty block hex")
		} else {
			r.Record(area, "GetBlock(tip)", StatusPass, itoa(len(*hexBlk)/2)+" bytes")
		}

		if bj, err := nc.GetBlockJSON(ctx, bid); err != nil {
			r.Record(area, "GetBlockJSON(tip)", StatusFail, err.Error())
		} else if !json.Valid(bj) {
			r.Record(area, "GetBlockJSON(tip)", StatusFail, "invalid JSON")
		} else {
			r.Record(area, "GetBlockJSON(tip)", StatusPass, itoa(len(bj))+" bytes")
		}

		if blks, err := nc.GetMainchainBlocks(ctx, height-2, 2); err != nil {
			r.Record(area, "GetMainchainBlocks", StatusFail, err.Error())
		} else if len(blks) != 2 {
			r.Record(area, "GetMainchainBlocks", StatusFail, "expected 2 blocks, got "+itoa(len(blks)))
		} else {
			r.Record(area, "GetMainchainBlocks", StatusPass, "2 blocks")
		}
	}

	// -- mempool ----------------------------------------------------------------
	if fr, err := nc.GetFeeRate(ctx, 1); err != nil || fr == nil || cmpAtoms(fr.AmountPerKB.Atoms, "0") <= 0 {
		detail := "nil rate"
		if err != nil {
			detail = err.Error()
		}
		r.Record(area, "GetFeeRate(1)", StatusFail, detail)
	} else {
		r.Record(area, "GetFeeRate(1)", StatusPass, fr.AmountPerKB.Atoms+" atoms/KB")
	}

	if pts, err := nc.GetFeeRatePoints(ctx); err != nil || len(pts) == 0 {
		detail := "empty points"
		if err != nil {
			detail = err.Error()
		}
		r.Record(area, "GetFeeRatePoints", StatusFail, detail)
	} else {
		r.Record(area, "GetFeeRatePoints", StatusPass, itoa(len(pts))+" points")
	}

	if mu, err := nc.MemoryUsage(ctx); err != nil {
		r.Record(area, "MemoryUsage", StatusFail, err.Error())
	} else {
		r.Record(area, "MemoryUsage", StatusPass, uitoa(mu)+" bytes")
	}

	if found, err := nc.ContainsTx(ctx, strings.Repeat("ab", 32)); err != nil {
		r.Record(area, "ContainsTx(unknown)", StatusFail, err.Error())
	} else if found {
		r.Record(area, "ContainsTx(unknown)", StatusFail, "unknown txid reported present")
	} else {
		r.Record(area, "ContainsTx(unknown)", StatusPass, "false for unknown txid")
	}

	// -- p2p ----------------------------------------------------------------------
	if pc, err := nc.GetPeerCount(ctx); err != nil {
		r.Record(area, "GetPeerCount", StatusFail, err.Error())
	} else {
		r.Record(area, "GetPeerCount", StatusPass, uitoa(pc)+" peers")
	}

	if peers, err := nc.GetConnectedPeers(ctx); err != nil {
		r.Record(area, "GetConnectedPeers", StatusFail, err.Error())
	} else {
		r.Record(area, "GetConnectedPeers", StatusPass, itoa(len(peers))+" peers")
	}

	if binds, err := nc.GetBindAddresses(ctx); err != nil {
		r.Record(area, "GetBindAddresses", StatusFail, err.Error())
	} else {
		r.Record(area, "GetBindAddresses", StatusPass, strings.Join(binds, ","))
	}

	if banned, err := nc.ListBanned(ctx); err != nil {
		r.Record(area, "ListBanned", StatusFail, err.Error())
	} else {
		r.Record(area, "ListBanned", StatusPass, itoa(len(banned))+" entries")
	}

	// -- token/order queries over pre-existing chain data --------------------------
	existingToken := envOr("EXISTING_TOKEN_ID", "tmltk1sm5xzzpjphv4nuv2r27ve6g2cexaea4muevtlhrejypehv4d25esmg2xe8")
	if ti, err := nc.TokenInfo(ctx, existingToken); err != nil {
		r.Record(area, "TokenInfo(existing)", StatusFail, err.Error())
	} else if ti == nil || ti.Type == "" {
		r.Record(area, "TokenInfo(existing)", StatusFail, "empty token info")
	} else {
		r.Record(area, "TokenInfo(existing)", StatusPass, "type="+ti.Type)
	}

	// An SDK-shape probe: node.OrderInfo.Nonce is null for active orders on this
	// daemon; the SDK types it as uint64, so unmarshalling may fail. Recorded as
	// a finding when it does.
	demoOrder := envOr("EXISTING_ORDER_ID", "")
	if demoOrder != "" {
		if _, err := nc.OrderInfo(ctx, demoOrder); err != nil {
			r.Record(area, "OrderInfo(existing)", StatusFail, "SDK unmarshal: "+err.Error())
			r.Finding("node.OrderInfo fails on active orders: daemon sends nonce:null but the SDK types Nonce as uint64 (%v)", err)
		} else {
			r.Record(area, "OrderInfo(existing)", StatusPass, short(demoOrder))
		}
	}

	if m, err := nc.OrdersInfoByCurrencies(ctx, &node.Currency{Type: "Coin"}, nil); err != nil {
		r.Record(area, "OrdersInfoByCurrencies(ask=Coin)", StatusFail, err.Error())
	} else {
		r.Record(area, "OrdersInfoByCurrencies(ask=Coin)", StatusPass, itoa(len(m))+" active coin-ask orders")
	}
}
