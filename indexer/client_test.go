// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package indexer_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mintlayer/go-sdk/indexer"
)

// jsonHandler returns an httptest.Server that encodes result as JSON on every request.
func jsonHandler(t *testing.T, result any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(result); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
}

// errorHandler returns an httptest.Server that replies with statusCode and body.
func errorHandler(t *testing.T, statusCode int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, body, statusCode)
	}))
}

// --- Transport ---

func TestNewClient(t *testing.T) {
	c := indexer.New("http://127.0.0.1:3000")
	if c == nil {
		t.Fatal("New returned nil")
	}
}

func TestWithTimeout(t *testing.T) {
	c := indexer.New("http://127.0.0.1:3000", indexer.WithTimeout(5*time.Second))
	if c == nil {
		t.Fatal("New returned nil")
	}
}

func TestWithHTTPClient(t *testing.T) {
	c := indexer.New("http://127.0.0.1:3000", indexer.WithHTTPClient(&http.Client{}))
	if c == nil {
		t.Fatal("New returned nil")
	}
}

// --- HTTP error path ---

func TestHTTPError_404(t *testing.T) {
	srv := errorHandler(t, http.StatusNotFound, `{"error":"NotFound"}`)
	defer srv.Close()

	c := indexer.New(srv.URL)
	_, err := c.GetTip(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	httpErr, ok := err.(*indexer.HTTPError)
	if !ok {
		t.Fatalf("expected *indexer.HTTPError, got %T: %v", err, err)
	}
	if httpErr.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", httpErr.StatusCode)
	}
}

func TestHTTPError_500(t *testing.T) {
	srv := errorHandler(t, http.StatusInternalServerError, "internal error")
	defer srv.Close()

	c := indexer.New(srv.URL)
	_, err := c.GetPool(context.Background(), "mpool1abc")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	httpErr, ok := err.(*indexer.HTTPError)
	if !ok {
		t.Fatalf("expected *indexer.HTTPError, got %T", err)
	}
	if httpErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", httpErr.StatusCode)
	}
}

// --- Chain (3b) ---

func TestGetTip(t *testing.T) {
	tip := indexer.ChainTip{BlockHeight: 123456, BlockID: "aabbccdd"}
	srv := jsonHandler(t, tip)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetTip(context.Background())
	if err != nil {
		t.Fatalf("GetTip: %v", err)
	}
	if got.BlockHeight != 123456 {
		t.Errorf("expected height 123456, got %d", got.BlockHeight)
	}
	if got.BlockID != "aabbccdd" {
		t.Errorf("expected id aabbccdd, got %q", got.BlockID)
	}
}

func TestGetGenesis(t *testing.T) {
	genesis := indexer.GenesisInfo{
		BlockID:        "genesisid",
		GenesisMessage: "mintlayer",
		Timestamp:      indexer.Timestamp{Timestamp: 1700000000},
	}
	srv := jsonHandler(t, genesis)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetGenesis(context.Background())
	if err != nil {
		t.Fatalf("GetGenesis: %v", err)
	}
	if got.BlockID != "genesisid" {
		t.Errorf("expected genesisid, got %q", got.BlockID)
	}
	if got.Timestamp.Timestamp != 1700000000 {
		t.Errorf("expected timestamp 1700000000, got %d", got.Timestamp.Timestamp)
	}
}

func TestGetBlockIDAtHeight(t *testing.T) {
	srv := jsonHandler(t, "deadbeef0102")
	defer srv.Close()

	c := indexer.New(srv.URL)
	id, err := c.GetBlockIDAtHeight(context.Background(), 100000)
	if err != nil {
		t.Fatalf("GetBlockIDAtHeight: %v", err)
	}
	if id != "deadbeef0102" {
		t.Errorf("unexpected id: %q", id)
	}
}

func TestGetBlockIDAtHeight_PathContainsHeight(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode("aabb")
	}))
	defer srv.Close()

	c := indexer.New(srv.URL)
	_, _ = c.GetBlockIDAtHeight(context.Background(), 12345)
	if !strings.Contains(gotPath, "12345") {
		t.Errorf("expected 12345 in path %q", gotPath)
	}
}

// --- Block (3c) ---

func TestGetBlockTransactionIDs(t *testing.T) {
	ids := []string{"tx1", "tx2", "tx3"}
	srv := jsonHandler(t, ids)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetBlockTransactionIDs(context.Background(), "aabbccdd")
	if err != nil {
		t.Fatalf("GetBlockTransactionIDs: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 ids, got %d", len(got))
	}
	if got[0] != "tx1" {
		t.Errorf("expected tx1, got %q", got[0])
	}
}

func TestGetBlockHeader(t *testing.T) {
	header := indexer.BlockHeader{
		PreviousBlockID: "prevblock",
		MerkleRoot:      "merkleroot",
	}
	srv := jsonHandler(t, header)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetBlockHeader(context.Background(), "blockid01")
	if err != nil {
		t.Fatalf("GetBlockHeader: %v", err)
	}
	if got.PreviousBlockID != "prevblock" {
		t.Errorf("expected prevblock, got %q", got.PreviousBlockID)
	}
}

// --- Transaction (3d) ---

func TestGetTransaction(t *testing.T) {
	tx := indexer.Transaction{
		ID:            "aabbccdd",
		BlockID:       "blockid01",
		Timestamp:     "1700000000",
		Confirmations: "100",
	}
	srv := jsonHandler(t, tx)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetTransaction(context.Background(), "aabbccdd")
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if got.ID != "aabbccdd" {
		t.Errorf("unexpected tx id: %q", got.ID)
	}
	if got.Confirmations != "100" {
		t.Errorf("unexpected confirmations: %q", got.Confirmations)
	}
}

func TestGetTransaction_Unconfirmed(t *testing.T) {
	tx := indexer.Transaction{
		ID:            "pending01",
		BlockID:       "",
		Timestamp:     "",
		Confirmations: "",
	}
	srv := jsonHandler(t, tx)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetTransaction(context.Background(), "pending01")
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if got.BlockID != "" {
		t.Errorf("expected empty block_id for unconfirmed tx, got %q", got.BlockID)
	}
}

func TestListTransactions_Pagination(t *testing.T) {
	var gotURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]indexer.Transaction{})
	}))
	defer srv.Close()

	c := indexer.New(srv.URL)
	_, err := c.ListTransactions(context.Background(), indexer.PageOpts{Offset: 10, Items: 20})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if !strings.Contains(gotURI, "offset=10") {
		t.Errorf("expected offset=10 in %q", gotURI)
	}
	if !strings.Contains(gotURI, "items=20") {
		t.Errorf("expected items=20 in %q", gotURI)
	}
}

func TestGetTransactionMerklePath(t *testing.T) {
	mp := indexer.MerklePath{
		BlockID:          "block01",
		TransactionIndex: 3,
		MerkleRoot:       "root01",
		Path:             []string{"hash1", "hash2"},
	}
	srv := jsonHandler(t, mp)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetTransactionMerklePath(context.Background(), "tx01")
	if err != nil {
		t.Fatalf("GetTransactionMerklePath: %v", err)
	}
	if got.TransactionIndex != 3 {
		t.Errorf("expected index 3, got %d", got.TransactionIndex)
	}
	if len(got.Path) != 2 {
		t.Errorf("expected 2 path elements, got %d", len(got.Path))
	}
}

func TestSubmitTransaction(t *testing.T) {
	var gotBody string
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 64)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		gotContentType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"tx_id": "newtxid"})
	}))
	defer srv.Close()

	c := indexer.New(srv.URL)
	txID, err := c.SubmitTransaction(context.Background(), "cafebabe")
	if err != nil {
		t.Fatalf("SubmitTransaction: %v", err)
	}
	if txID != "newtxid" {
		t.Errorf("expected newtxid, got %q", txID)
	}
	if gotBody != "cafebabe" {
		t.Errorf("expected body cafebabe, got %q", gotBody)
	}
	if !strings.HasPrefix(gotContentType, "text/plain") {
		t.Errorf("expected text/plain content-type, got %q", gotContentType)
	}
}

// --- Address (3e) ---

func TestGetAddressInfo(t *testing.T) {
	info := indexer.AddressInfo{
		CoinBalance:        indexer.Amount{Atoms: "100000000000", Decimal: "1.0"},
		LockedCoinBalance:  indexer.Amount{Atoms: "0", Decimal: "0"},
		TransactionHistory: []string{"tx1", "tx2"},
		Tokens:             []indexer.TokenBalance{},
	}
	srv := jsonHandler(t, info)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetAddressInfo(context.Background(), "mtc1abc")
	if err != nil {
		t.Fatalf("GetAddressInfo: %v", err)
	}
	if got.CoinBalance.Atoms != "100000000000" {
		t.Errorf("unexpected atoms: %q", got.CoinBalance.Atoms)
	}
	if len(got.TransactionHistory) != 2 {
		t.Errorf("expected 2 txs, got %d", len(got.TransactionHistory))
	}
}

func TestGetSpendableUTXOs(t *testing.T) {
	utxos := []indexer.UTXO{
		{Outpoint: indexer.UTXOOutpoint{SourceID: "tx1", Index: 0}},
		{Outpoint: indexer.UTXOOutpoint{SourceID: "tx2", Index: 1}},
	}
	srv := jsonHandler(t, utxos)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetSpendableUTXOs(context.Background(), "mtc1abc")
	if err != nil {
		t.Fatalf("GetSpendableUTXOs: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 utxos, got %d", len(got))
	}
	if got[0].Outpoint.SourceID != "tx1" {
		t.Errorf("unexpected source id: %q", got[0].Outpoint.SourceID)
	}
	if got[1].Outpoint.Index != 1 {
		t.Errorf("expected index 1, got %d", got[1].Outpoint.Index)
	}
}

func TestGetDelegations(t *testing.T) {
	delgs := []indexer.DelegationInfo{
		{
			DelegationID:     "mdelg1abc",
			PoolID:           "mpool1xyz",
			NextNonce:        3,
			SpendDestination: "mtc1dest",
			Balance:          indexer.Amount{Atoms: "500000000000", Decimal: "5.0"},
		},
	}
	srv := jsonHandler(t, delgs)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetDelegations(context.Background(), "mtc1abc")
	if err != nil {
		t.Fatalf("GetDelegations: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 delegation, got %d", len(got))
	}
	if got[0].DelegationID != "mdelg1abc" {
		t.Errorf("unexpected delegation id: %q", got[0].DelegationID)
	}
	if got[0].NextNonce != 3 {
		t.Errorf("expected nonce 3, got %d", got[0].NextNonce)
	}
}

func TestGetTokenAuthority(t *testing.T) {
	tokenIDs := []string{"mmltk1aaa", "mmltk1bbb"}
	srv := jsonHandler(t, tokenIDs)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetTokenAuthority(context.Background(), "mtc1abc")
	if err != nil {
		t.Fatalf("GetTokenAuthority: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 token ids, got %d", len(got))
	}
	if got[0] != "mmltk1aaa" {
		t.Errorf("unexpected token id: %q", got[0])
	}
}

// --- Pool (3f) ---

func TestListPools(t *testing.T) {
	pools := []indexer.Pool{
		{
			PoolID:                 "mpool1abc",
			StakerBalance:          indexer.Amount{Atoms: "40000000000000", Decimal: "400000.0"},
			MarginRatioPerThousand: 100,
		},
	}
	srv := jsonHandler(t, pools)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.ListPools(context.Background(), indexer.PoolListOpts{})
	if err != nil {
		t.Fatalf("ListPools: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(got))
	}
	if got[0].PoolID != "mpool1abc" {
		t.Errorf("unexpected pool id: %q", got[0].PoolID)
	}
	if got[0].MarginRatioPerThousand != 100 {
		t.Errorf("expected margin 100, got %d", got[0].MarginRatioPerThousand)
	}
}

func TestListPools_SortParam(t *testing.T) {
	var gotURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]indexer.Pool{})
	}))
	defer srv.Close()

	c := indexer.New(srv.URL)
	_, err := c.ListPools(context.Background(), indexer.PoolListOpts{Sort: "by_pledge"})
	if err != nil {
		t.Fatalf("ListPools: %v", err)
	}
	if !strings.Contains(gotURI, "sort=by_pledge") {
		t.Errorf("expected sort=by_pledge in %q", gotURI)
	}
}

func TestGetPool(t *testing.T) {
	pool := indexer.Pool{
		PoolID:        "mpool1abc",
		StakerBalance: indexer.Amount{Atoms: "40000000000000", Decimal: "400000.0"},
	}
	srv := jsonHandler(t, pool)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetPool(context.Background(), "mpool1abc")
	if err != nil {
		t.Fatalf("GetPool: %v", err)
	}
	if got.PoolID != "mpool1abc" {
		t.Errorf("unexpected pool id: %q", got.PoolID)
	}
}

func TestGetPoolBlockStats(t *testing.T) {
	srv := jsonHandler(t, map[string]any{"block_count": uint64(42)})
	defer srv.Close()

	c := indexer.New(srv.URL)
	from := time.Unix(1700000000, 0)
	to := time.Unix(1700086400, 0)
	count, err := c.GetPoolBlockStats(context.Background(), "mpool1abc", from, to)
	if err != nil {
		t.Fatalf("GetPoolBlockStats: %v", err)
	}
	if count != 42 {
		t.Errorf("expected 42, got %d", count)
	}
}

func TestGetPoolBlockStats_QueryParams(t *testing.T) {
	var gotURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"block_count": 0})
	}))
	defer srv.Close()

	c := indexer.New(srv.URL)
	from := time.Unix(1700000000, 0)
	to := time.Unix(1700086400, 0)
	_, _ = c.GetPoolBlockStats(context.Background(), "mpool1abc", from, to)
	if !strings.Contains(gotURI, "from=1700000000") {
		t.Errorf("expected from param in %q", gotURI)
	}
	if !strings.Contains(gotURI, "to=1700086400") {
		t.Errorf("expected to param in %q", gotURI)
	}
}

func TestGetDelegation(t *testing.T) {
	delg := indexer.Delegation{
		DelegationID:        "mdelg1abc",
		PoolID:              "mpool1xyz",
		NextNonce:           7,
		SpendDestination:    "mtc1dest",
		Balance:             indexer.Amount{Atoms: "500000000000", Decimal: "5.0"},
		CreationBlockHeight: 10000,
	}
	srv := jsonHandler(t, delg)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetDelegation(context.Background(), "mdelg1abc")
	if err != nil {
		t.Fatalf("GetDelegation: %v", err)
	}
	if got.DelegationID != "mdelg1abc" {
		t.Errorf("unexpected delegation id: %q", got.DelegationID)
	}
	if got.PoolID != "mpool1xyz" {
		t.Errorf("unexpected pool id: %q", got.PoolID)
	}
	if got.NextNonce != 7 {
		t.Errorf("expected nonce 7, got %d", got.NextNonce)
	}
	if got.CreationBlockHeight != 10000 {
		t.Errorf("expected height 10000, got %d", got.CreationBlockHeight)
	}
}

func TestGetPoolDelegations(t *testing.T) {
	delgs := []indexer.PoolDelegation{
		{
			DelegationID:        "mdelg1abc",
			NextNonce:           5,
			SpendDestination:    "mtc1dest",
			Balance:             indexer.Amount{Atoms: "500000000000", Decimal: "5.0"},
			CreationBlockHeight: 10000,
		},
	}
	srv := jsonHandler(t, delgs)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetPoolDelegations(context.Background(), "mpool1abc")
	if err != nil {
		t.Fatalf("GetPoolDelegations: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 delegation, got %d", len(got))
	}
	if got[0].CreationBlockHeight != 10000 {
		t.Errorf("expected height 10000, got %d", got[0].CreationBlockHeight)
	}
}

// --- Token / NFT (3g) ---

func TestListTokens(t *testing.T) {
	ids := []string{"mmltk1aaa", "mmltk1bbb"}
	srv := jsonHandler(t, ids)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.ListTokens(context.Background(), indexer.PageOpts{})
	if err != nil {
		t.Fatalf("ListTokens: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(got))
	}
}

func TestGetToken(t *testing.T) {
	info := indexer.TokenInfo{
		Authority:        "mtc1abc",
		TokenTicker:      "TKN",
		NumberOfDecimals: 8,
		NextNonce:        2,
	}
	srv := jsonHandler(t, info)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetToken(context.Background(), "mmltk1abc")
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if got.TokenTicker != "TKN" {
		t.Errorf("unexpected ticker: %q", got.TokenTicker)
	}
	if got.NumberOfDecimals != 8 {
		t.Errorf("expected 8 decimals, got %d", got.NumberOfDecimals)
	}
}

func TestGetTokenTransactions(t *testing.T) {
	txs := []indexer.TokenTx{
		{TxGlobalIndex: 12345, TxID: "tx01"},
		{TxGlobalIndex: 12346, TxID: "tx02"},
	}
	srv := jsonHandler(t, txs)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetTokenTransactions(context.Background(), "mmltk1abc", indexer.PageOpts{})
	if err != nil {
		t.Fatalf("GetTokenTransactions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 txs, got %d", len(got))
	}
	if got[0].TxGlobalIndex != 12345 {
		t.Errorf("expected global index 12345, got %d", got[0].TxGlobalIndex)
	}
}

func TestFindTokensByTicker(t *testing.T) {
	srv := jsonHandler(t, []string{"mmltk1aaa"})
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.FindTokensByTicker(context.Background(), "TKN", indexer.PageOpts{})
	if err != nil {
		t.Fatalf("FindTokensByTicker: %v", err)
	}
	if len(got) != 1 || got[0] != "mmltk1aaa" {
		t.Errorf("unexpected result: %v", got)
	}
}

func TestGetNFT(t *testing.T) {
	nft := indexer.NFTInfo{
		Owner:   "mtc1abc",
		TokenID: "mmltk1nft",
		Metadata: indexer.NFTMetadata{
			Name:   "My NFT",
			Ticker: "NFT",
		},
	}
	srv := jsonHandler(t, nft)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetNFT(context.Background(), "mmltk1nft")
	if err != nil {
		t.Fatalf("GetNFT: %v", err)
	}
	if got.Metadata.Name != "My NFT" {
		t.Errorf("unexpected name: %q", got.Metadata.Name)
	}
	if got.Owner != "mtc1abc" {
		t.Errorf("unexpected owner: %q", got.Owner)
	}
}

// --- Order (3h) ---

func TestListOrders(t *testing.T) {
	orders := []indexer.Order{
		{OrderID: "mord1abc", Nonce: 0},
	}
	srv := jsonHandler(t, orders)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.ListOrders(context.Background(), indexer.PageOpts{})
	if err != nil {
		t.Fatalf("ListOrders: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 order, got %d", len(got))
	}
	if got[0].OrderID != "mord1abc" {
		t.Errorf("unexpected order id: %q", got[0].OrderID)
	}
}

func TestGetOrder(t *testing.T) {
	order := indexer.Order{
		OrderID:        "mord1abc",
		Nonce:          5,
		InitiallyGiven: indexer.Amount{Atoms: "100000000000", Decimal: "1.0"},
	}
	srv := jsonHandler(t, order)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetOrder(context.Background(), "mord1abc")
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if got.Nonce != 5 {
		t.Errorf("expected nonce 5, got %d", got.Nonce)
	}
}

func TestListOrdersByPair(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]indexer.Order{})
	}))
	defer srv.Close()

	c := indexer.New(srv.URL)
	_, err := c.ListOrdersByPair(context.Background(), "ML", "mmltk1abc", indexer.PageOpts{})
	if err != nil {
		t.Fatalf("ListOrdersByPair: %v", err)
	}
	if !strings.Contains(gotPath, "ML_mmltk1abc") {
		t.Errorf("expected ML_mmltk1abc in path %q", gotPath)
	}
}

// --- Statistics (3i) ---

func TestGetCoinStatistics(t *testing.T) {
	stats := indexer.CoinStats{
		CirculatingSupply: indexer.Amount{Atoms: "1000000000000000", Decimal: "10000000.0"},
		Preminted:         indexer.Amount{Atoms: "400000000000000", Decimal: "4000000.0"},
		Burned:            indexer.Amount{Atoms: "0", Decimal: "0"},
		Staked:            indexer.Amount{Atoms: "500000000000000", Decimal: "5000000.0"},
	}
	srv := jsonHandler(t, stats)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetCoinStatistics(context.Background())
	if err != nil {
		t.Fatalf("GetCoinStatistics: %v", err)
	}
	if got.CirculatingSupply.Atoms != "1000000000000000" {
		t.Errorf("unexpected supply: %q", got.CirculatingSupply.Atoms)
	}
	if got.Staked.Decimal != "5000000.0" {
		t.Errorf("unexpected staked decimal: %q", got.Staked.Decimal)
	}
}

func TestGetTokenStatistics(t *testing.T) {
	stats := indexer.CoinStats{
		CirculatingSupply: indexer.Amount{Atoms: "1000000000", Decimal: "10.0"},
	}
	srv := jsonHandler(t, stats)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetTokenStatistics(context.Background(), "mmltk1abc")
	if err != nil {
		t.Fatalf("GetTokenStatistics: %v", err)
	}
	if got.CirculatingSupply.Atoms != "1000000000" {
		t.Errorf("unexpected supply: %q", got.CirculatingSupply.Atoms)
	}
}

func TestGetFeeRate(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode("1000")
	}))
	defer srv.Close()

	c := indexer.New(srv.URL)
	rate, err := c.GetFeeRate(context.Background(), 5)
	if err != nil {
		t.Fatalf("GetFeeRate: %v", err)
	}
	if rate != "1000" {
		t.Errorf("expected 1000, got %q", rate)
	}
	if !strings.Contains(gotQuery, "in_top_x_mb=5") {
		t.Errorf("expected in_top_x_mb=5 in query %q", gotQuery)
	}
}

func TestGetFeeRate_DefaultParam(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode("500")
	}))
	defer srv.Close()

	c := indexer.New(srv.URL)
	_, _ = c.GetFeeRate(context.Background(), 0) // pass 0 → no param sent
	if strings.Contains(gotQuery, "in_top_x_mb") {
		t.Errorf("expected no in_top_x_mb param when inTopXMb=0, got %q", gotQuery)
	}
}

// --- Pagination end-to-end ---

func TestPageOpts_ZeroValues_NoParams(t *testing.T) {
	var gotURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]indexer.Order{})
	}))
	defer srv.Close()

	c := indexer.New(srv.URL)
	_, _ = c.ListOrders(context.Background(), indexer.PageOpts{}) // zero opts
	if strings.Contains(gotURI, "offset") || strings.Contains(gotURI, "items") {
		t.Errorf("expected no pagination params for zero PageOpts, got %q", gotURI)
	}
}

// --- String-encoded integer fields (API returns quoted numbers) ---

func rawHandler(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

func TestPool_MarginRatio_StringForm(t *testing.T) {
	raw := `[{"pool_id":"mpool1abc","decommission_destination":"","staker_balance":{"atoms":"0","decimal":"0"},"margin_ratio_per_thousand":"10","cost_per_block":{"atoms":"0","decimal":"0"},"vrf_public_key":"","delegations_balance":{"atoms":"0","decimal":"0"}}]`
	srv := rawHandler(t, raw)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.ListPools(context.Background(), indexer.PoolListOpts{})
	if err != nil {
		t.Fatalf("ListPools: %v", err)
	}
	if got[0].MarginRatioPerThousand != 10 {
		t.Errorf("expected 10, got %d", got[0].MarginRatioPerThousand)
	}
}

func TestDelegation_NextNonce_StringForm(t *testing.T) {
	raw := `{"delegation_id":"mdelg1abc","pool_id":"mpool1xyz","next_nonce":"7","spend_destination":"mtc1dest","balance":{"atoms":"0","decimal":"0"},"creation_block_height":"10000"}`
	srv := rawHandler(t, raw)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetDelegation(context.Background(), "mdelg1abc")
	if err != nil {
		t.Fatalf("GetDelegation: %v", err)
	}
	if got.NextNonce != 7 {
		t.Errorf("expected nonce 7, got %d", got.NextNonce)
	}
	if got.CreationBlockHeight != 10000 {
		t.Errorf("expected height 10000, got %d", got.CreationBlockHeight)
	}
}

func TestOrder_Nonce_StringForm(t *testing.T) {
	raw := `{"order_id":"mord1abc","conclude_destination":"","give_currency":{},"initially_given":{"atoms":"0","decimal":"0"},"give_balance":{"atoms":"0","decimal":"0"},"ask_currency":{},"initially_asked":{"atoms":"0","decimal":"0"},"ask_balance":{"atoms":"0","decimal":"0"},"nonce":"5"}`
	srv := rawHandler(t, raw)
	defer srv.Close()

	c := indexer.New(srv.URL)
	got, err := c.GetOrder(context.Background(), "mord1abc")
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if got.Nonce != 5 {
		t.Errorf("expected nonce 5, got %d", got.Nonce)
	}
}
