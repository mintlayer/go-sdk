// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package node_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mintlayer/go-sdk/node"
)

// rpcHandler returns an httptest.Server that replies with the given JSON result.
func rpcHandler(t *testing.T, result any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate request shape.
		var req struct {
			JSONRPC string `json:"jsonrpc"`
			ID      uint64 `json:"id"`
			Method  string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if req.JSONRPC != "2.0" {
			t.Errorf("expected jsonrpc 2.0, got %q", req.JSONRPC)
		}

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

// rpcErrorHandler returns an httptest.Server that replies with a JSON-RPC error.
func rpcErrorHandler(t *testing.T, code int, message string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID uint64 `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"error": map[string]any{
				"code":    code,
				"message": message,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// --- Transport ---

func TestNewClient_Defaults(t *testing.T) {
	c := node.New("http://127.0.0.1:3030")
	if c == nil {
		t.Fatal("New returned nil")
	}
}

func TestWithTimeout(t *testing.T) {
	c := node.New("http://127.0.0.1:3030", node.WithTimeout(5*time.Second))
	if c == nil {
		t.Fatal("New returned nil")
	}
}

func TestWithBasicAuth(t *testing.T) {
	// Verify that basic auth header is sent.
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		resp := map[string]any{"jsonrpc": "2.0", "id": 1, "result": "1.0.0"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := node.New(srv.URL, node.WithBasicAuth("alice", "secret"))
	_, err := c.NodeVersion(context.Background())
	if err != nil {
		t.Fatalf("NodeVersion: %v", err)
	}
	if gotAuth == "" {
		t.Error("expected Authorization header, got none")
	}
}

// --- Error path ---

func TestRPCError(t *testing.T) {
	srv := rpcErrorHandler(t, -32601, "method not found")
	defer srv.Close()

	c := node.New(srv.URL)
	_, err := c.NodeVersion(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	rpcErr, ok := err.(*node.RPCError)
	if !ok {
		t.Fatalf("expected *node.RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != -32601 {
		t.Errorf("expected code -32601, got %d", rpcErr.Code)
	}
}

// --- node module ---

func TestNodeVersion(t *testing.T) {
	srv := rpcHandler(t, "1.3.0")
	defer srv.Close()

	c := node.New(srv.URL)
	v, err := c.NodeVersion(context.Background())
	if err != nil {
		t.Fatalf("NodeVersion: %v", err)
	}
	if v != "1.3.0" {
		t.Errorf("expected 1.3.0, got %q", v)
	}
}

func TestNodeShutdown(t *testing.T) {
	srv := rpcHandler(t, nil)
	defer srv.Close()

	c := node.New(srv.URL)
	if err := c.NodeShutdown(context.Background()); err != nil {
		t.Fatalf("NodeShutdown: %v", err)
	}
}

// --- chainstate module ---

func TestChainstateInfo(t *testing.T) {
	info := node.ChainstateInfo{
		BestBlockHeight:        123456,
		BestBlockID:            "aabbccdd",
		BestBlockTimestamp:     node.Timestamp{Timestamp: 1700000000},
		MedianTime:             node.Timestamp{Timestamp: 1699999500},
		IsInitialBlockDownload: false,
	}
	srv := rpcHandler(t, info)
	defer srv.Close()

	c := node.New(srv.URL)
	got, err := c.ChainstateInfo(context.Background())
	if err != nil {
		t.Fatalf("ChainstateInfo: %v", err)
	}
	if got.BestBlockHeight != 123456 {
		t.Errorf("expected height 123456, got %d", got.BestBlockHeight)
	}
	if got.BestBlockID != "aabbccdd" {
		t.Errorf("expected id aabbccdd, got %q", got.BestBlockID)
	}
	if got.IsInitialBlockDownload {
		t.Error("expected IsInitialBlockDownload=false")
	}
}

func TestBestBlockID(t *testing.T) {
	srv := rpcHandler(t, "deadbeef01020304")
	defer srv.Close()

	c := node.New(srv.URL)
	id, err := c.BestBlockID(context.Background())
	if err != nil {
		t.Fatalf("BestBlockID: %v", err)
	}
	if id != "deadbeef01020304" {
		t.Errorf("unexpected id: %q", id)
	}
}

func TestBestBlockHeight(t *testing.T) {
	srv := rpcHandler(t, uint64(999))
	defer srv.Close()

	c := node.New(srv.URL)
	h, err := c.BestBlockHeight(context.Background())
	if err != nil {
		t.Fatalf("BestBlockHeight: %v", err)
	}
	if h != 999 {
		t.Errorf("expected 999, got %d", h)
	}
}

func TestBlockIDAtHeight_Nil(t *testing.T) {
	srv := rpcHandler(t, nil)
	defer srv.Close()

	c := node.New(srv.URL)
	id, err := c.BlockIDAtHeight(context.Background(), 9999999)
	if err != nil {
		t.Fatalf("BlockIDAtHeight: %v", err)
	}
	if id != nil {
		t.Errorf("expected nil, got %q", *id)
	}
}

func TestStakePoolBalance(t *testing.T) {
	srv := rpcHandler(t, node.Amount{Atoms: "100000000000"})
	defer srv.Close()

	c := node.New(srv.URL)
	bal, err := c.StakePoolBalance(context.Background(), "mpool1abc")
	if err != nil {
		t.Fatalf("StakePoolBalance: %v", err)
	}
	if bal == nil {
		t.Fatal("expected non-nil balance")
	}
	if bal.Atoms != "100000000000" {
		t.Errorf("unexpected atoms: %q", bal.Atoms)
	}
}

func TestGetUTXO_NotFound(t *testing.T) {
	srv := rpcHandler(t, nil)
	defer srv.Close()

	c := node.New(srv.URL)
	raw, err := c.GetUTXO(context.Background(), node.Outpoint{
		SourceID: node.OutpointSourceID{Type: "Transaction"},
		Index:    0,
	})
	if err != nil {
		t.Fatalf("GetUTXO: %v", err)
	}
	// null result unmarshals as "null" bytes
	if string(raw) != "null" {
		t.Errorf("expected null, got %q", raw)
	}
}

// --- mempool module ---

func TestContainsTx(t *testing.T) {
	srv := rpcHandler(t, true)
	defer srv.Close()

	c := node.New(srv.URL)
	ok, err := c.ContainsTx(context.Background(), "aabb1234")
	if err != nil {
		t.Fatalf("ContainsTx: %v", err)
	}
	if !ok {
		t.Error("expected true")
	}
}

func TestGetTransaction_NotFound(t *testing.T) {
	srv := rpcHandler(t, nil)
	defer srv.Close()

	c := node.New(srv.URL)
	tx, err := c.GetTransaction(context.Background(), "deadbeef")
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if tx != nil {
		t.Errorf("expected nil, got %+v", tx)
	}
}

func TestGetTransaction_Found(t *testing.T) {
	want := node.MempoolTx{
		ID:          "aabb1234",
		Status:      "InMempool",
		Transaction: "cafebabe",
	}
	srv := rpcHandler(t, want)
	defer srv.Close()

	c := node.New(srv.URL)
	tx, err := c.GetTransaction(context.Background(), "aabb1234")
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if tx == nil {
		t.Fatal("expected non-nil MempoolTx")
	}
	if tx.Status != "InMempool" {
		t.Errorf("unexpected status: %q", tx.Status)
	}
}

func TestGetFeeRate(t *testing.T) {
	want := node.FeeRate{AmountPerKB: node.Amount{Atoms: "1000"}}
	srv := rpcHandler(t, want)
	defer srv.Close()

	c := node.New(srv.URL)
	fr, err := c.GetFeeRate(context.Background(), 5)
	if err != nil {
		t.Fatalf("GetFeeRate: %v", err)
	}
	if fr.AmountPerKB.Atoms != "1000" {
		t.Errorf("unexpected atoms: %q", fr.AmountPerKB.Atoms)
	}
}

func TestGetFeeRatePoints(t *testing.T) {
	// Wire format: array of [size, feeRate] pairs.
	raw := json.RawMessage(`[[1024,{"amount_per_kb":{"atoms":"500"}}],[2048,{"amount_per_kb":{"atoms":"750"}}]]`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID uint64 `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  raw,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := node.New(srv.URL)
	pts, err := c.GetFeeRatePoints(context.Background())
	if err != nil {
		t.Fatalf("GetFeeRatePoints: %v", err)
	}
	if len(pts) != 2 {
		t.Fatalf("expected 2 points, got %d", len(pts))
	}
	if pts[0].Size != 1024 {
		t.Errorf("expected size 1024, got %d", pts[0].Size)
	}
	if pts[1].Rate.AmountPerKB.Atoms != "750" {
		t.Errorf("unexpected atoms: %q", pts[1].Rate.AmountPerKB.Atoms)
	}
}

func TestMemoryUsage(t *testing.T) {
	srv := rpcHandler(t, uint64(4096))
	defer srv.Close()

	c := node.New(srv.URL)
	mem, err := c.MemoryUsage(context.Background())
	if err != nil {
		t.Fatalf("MemoryUsage: %v", err)
	}
	if mem != 4096 {
		t.Errorf("expected 4096, got %d", mem)
	}
}

// --- p2p module ---

func TestGetPeerCount(t *testing.T) {
	srv := rpcHandler(t, uint64(7))
	defer srv.Close()

	c := node.New(srv.URL)
	n, err := c.GetPeerCount(context.Background())
	if err != nil {
		t.Fatalf("GetPeerCount: %v", err)
	}
	if n != 7 {
		t.Errorf("expected 7, got %d", n)
	}
}

func TestGetConnectedPeers(t *testing.T) {
	peers := []node.PeerInfo{
		{
			PeerID:          42,
			Address:         "1.2.3.4:3031",
			PeerRole:        "OutboundFullRelay",
			UserAgent:       "mintlayer-node/1.3.0",
			SoftwareVersion: "1.3.0",
		},
	}
	srv := rpcHandler(t, peers)
	defer srv.Close()

	c := node.New(srv.URL)
	got, err := c.GetConnectedPeers(context.Background())
	if err != nil {
		t.Fatalf("GetConnectedPeers: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(got))
	}
	if got[0].PeerID != 42 {
		t.Errorf("expected peer_id 42, got %d", got[0].PeerID)
	}
}

func TestListBanned(t *testing.T) {
	// Wire format: [["addr", {"time": [secs, nanos]}], ...]
	raw := json.RawMessage(`[["1.2.3.4",{"time":[1700000000,0]}]]`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID uint64 `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  raw,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := node.New(srv.URL)
	banned, err := c.ListBanned(context.Background())
	if err != nil {
		t.Fatalf("ListBanned: %v", err)
	}
	if len(banned) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(banned))
	}
	if banned[0].Address != "1.2.3.4" {
		t.Errorf("unexpected address: %q", banned[0].Address)
	}
	if banned[0].BanTime[0] != 1700000000 {
		t.Errorf("unexpected ban time secs: %d", banned[0].BanTime[0])
	}
}

func TestBan(t *testing.T) {
	var gotParams map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     uint64         `json:"id"`
			Params map[string]any `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotParams = req.Params
		resp := map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": nil}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := node.New(srv.URL)
	if err := c.Ban(context.Background(), "5.6.7.8", 24*time.Hour); err != nil {
		t.Fatalf("Ban: %v", err)
	}
	if gotParams["address"] != "5.6.7.8" {
		t.Errorf("unexpected address param: %v", gotParams["address"])
	}
	// duration should be [86400, 0]
	dur, ok := gotParams["duration"].([]any)
	if !ok {
		t.Fatalf("duration is not array: %T", gotParams["duration"])
	}
	if len(dur) != 2 {
		t.Fatalf("expected duration len 2, got %d", len(dur))
	}
}

func TestP2PSubmitTransaction(t *testing.T) {
	srv := rpcHandler(t, nil)
	defer srv.Close()

	c := node.New(srv.URL)
	err := c.P2PSubmitTransaction(context.Background(), "cafebabe", node.TrustPolicyUntrusted)
	if err != nil {
		t.Fatalf("P2PSubmitTransaction: %v", err)
	}
}

func TestMempoolSubmitTransaction(t *testing.T) {
	srv := rpcHandler(t, nil)
	defer srv.Close()

	c := node.New(srv.URL)
	err := c.MempoolSubmitTransaction(context.Background(), "cafebabe", node.TrustPolicyUntrusted)
	if err != nil {
		t.Fatalf("MempoolSubmitTransaction: %v", err)
	}
}

// --- concurrent ID generation ---

func TestConcurrentCalls(t *testing.T) {
	srv := rpcHandler(t, "1.0.0")
	defer srv.Close()

	c := node.New(srv.URL)
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := c.NodeVersion(context.Background())
			done <- err
		}()
	}
	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent NodeVersion: %v", err)
		}
	}
}
