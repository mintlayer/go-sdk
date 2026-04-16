package wallet_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mintlayer/go-sdk/wallet"
)

// rpcHandler returns an httptest.Server that replies with the given JSON result.
func rpcHandler(t *testing.T, result any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	c := wallet.New("http://127.0.0.1:3034")
	if c == nil {
		t.Fatal("New returned nil")
	}
}

func TestWithTimeout(t *testing.T) {
	c := wallet.New("http://127.0.0.1:3034", wallet.WithTimeout(5*time.Second))
	if c == nil {
		t.Fatal("New returned nil")
	}
}

func TestWithBasicAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		resp := map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{
			"height": 100,
			"id":     "aabbccdd",
		}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := wallet.New(srv.URL, wallet.WithBasicAuth("alice", "secret"))
	_, err := c.BestBlock(context.Background())
	if err != nil {
		t.Fatalf("BestBlock: %v", err)
	}
	if gotAuth == "" {
		t.Error("expected Authorization header, got none")
	}
}

// --- Error path ---

func TestRPCError(t *testing.T) {
	srv := rpcErrorHandler(t, -32601, "method not found")
	defer srv.Close()

	c := wallet.New(srv.URL)
	_, err := c.BestBlock(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	rpcErr, ok := err.(*wallet.RPCError)
	if !ok {
		t.Fatalf("expected *wallet.RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != -32601 {
		t.Errorf("expected code -32601, got %d", rpcErr.Code)
	}
}

// --- Wallet management ---

func TestCreateWallet(t *testing.T) {
	mnemonic := "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10 word11 word12"
	result := map[string]any{
		"mnemonic": map[string]any{
			"type":    "NewlyGenerated",
			"content": map[string]any{"mnemonic": mnemonic},
		},
	}
	srv := rpcHandler(t, result)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.CreateWallet(context.Background(), wallet.CreateWalletParams{
		Path:            "/tmp/test.db",
		StoreSeedPhrase: true,
	})
	if err != nil {
		t.Fatalf("CreateWallet: %v", err)
	}
	if got.Mnemonic == nil {
		t.Fatal("expected mnemonic, got nil")
	}
	if got.Mnemonic.Type != "NewlyGenerated" {
		t.Errorf("expected NewlyGenerated, got %q", got.Mnemonic.Type)
	}
	if got.Mnemonic.Content.Mnemonic != mnemonic {
		t.Errorf("unexpected mnemonic: %q", got.Mnemonic.Content.Mnemonic)
	}
}

func TestRecoverWallet(t *testing.T) {
	srv := rpcHandler(t, nil)
	defer srv.Close()

	c := wallet.New(srv.URL)
	err := c.RecoverWallet(context.Background(), wallet.RecoverWalletParams{
		Path:            "/tmp/recovered.db",
		StoreSeedPhrase: false,
		Mnemonic:        "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10 word11 word12",
	})
	if err != nil {
		t.Fatalf("RecoverWallet: %v", err)
	}
}

func TestOpenWallet(t *testing.T) {
	srv := rpcHandler(t, nil)
	defer srv.Close()

	c := wallet.New(srv.URL)
	if err := c.OpenWallet(context.Background(), "/tmp/test.db", ""); err != nil {
		t.Fatalf("OpenWallet: %v", err)
	}
}

func TestCloseWallet(t *testing.T) {
	srv := rpcHandler(t, nil)
	defer srv.Close()

	c := wallet.New(srv.URL)
	if err := c.CloseWallet(context.Background()); err != nil {
		t.Fatalf("CloseWallet: %v", err)
	}
}

func TestGetWalletInfo(t *testing.T) {
	info := wallet.WalletInfo{
		WalletID:     "aabb1234",
		AccountNames: []string{"Main", "Savings"},
		ExtraInfo:    wallet.WalletExtraInfo{Type: "SoftwareWallet"},
	}
	srv := rpcHandler(t, info)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.GetWalletInfo(context.Background())
	if err != nil {
		t.Fatalf("GetWalletInfo: %v", err)
	}
	if got.WalletID != "aabb1234" {
		t.Errorf("unexpected wallet id: %q", got.WalletID)
	}
	if len(got.AccountNames) != 2 {
		t.Errorf("expected 2 accounts, got %d", len(got.AccountNames))
	}
	if got.ExtraInfo.Type != "SoftwareWallet" {
		t.Errorf("unexpected wallet type: %q", got.ExtraInfo.Type)
	}
}

func TestSyncWallet(t *testing.T) {
	srv := rpcHandler(t, nil)
	defer srv.Close()

	c := wallet.New(srv.URL)
	if err := c.SyncWallet(context.Background()); err != nil {
		t.Fatalf("SyncWallet: %v", err)
	}
}

func TestRescanWallet(t *testing.T) {
	srv := rpcHandler(t, nil)
	defer srv.Close()

	c := wallet.New(srv.URL)
	if err := c.RescanWallet(context.Background()); err != nil {
		t.Fatalf("RescanWallet: %v", err)
	}
}

func TestBestBlock(t *testing.T) {
	block := wallet.BestBlock{Height: 42000, ID: "deadbeef"}
	srv := rpcHandler(t, block)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.BestBlock(context.Background())
	if err != nil {
		t.Fatalf("BestBlock: %v", err)
	}
	if got.Height != 42000 {
		t.Errorf("expected height 42000, got %d", got.Height)
	}
	if got.ID != "deadbeef" {
		t.Errorf("expected id deadbeef, got %q", got.ID)
	}
}

func TestCreateAccount(t *testing.T) {
	srv := rpcHandler(t, wallet.AccountInfo{Account: 1, Name: "Savings"})
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.CreateAccount(context.Background(), "Savings")
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if got.Account != 1 {
		t.Errorf("expected account 1, got %d", got.Account)
	}
}

func TestRenameAccount(t *testing.T) {
	srv := rpcHandler(t, nil)
	defer srv.Close()

	c := wallet.New(srv.URL)
	if err := c.RenameAccount(context.Background(), 0, "Main"); err != nil {
		t.Fatalf("RenameAccount: %v", err)
	}
}

func TestGetBalance(t *testing.T) {
	balance := wallet.Balance{
		Coins:  wallet.Amount{Atoms: "1000000000000", Decimal: "10000.0"},
		Tokens: map[string]wallet.Amount{},
	}
	srv := rpcHandler(t, balance)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.GetBalance(context.Background(), 0)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if got.Coins.Atoms != "1000000000000" {
		t.Errorf("unexpected atoms: %q", got.Coins.Atoms)
	}
}

func TestNewAddress(t *testing.T) {
	srv := rpcHandler(t, map[string]string{"address": "tmltool1abc"})
	defer srv.Close()

	c := wallet.New(srv.URL)
	addr, err := c.NewAddress(context.Background(), 0)
	if err != nil {
		t.Fatalf("NewAddress: %v", err)
	}
	if addr != "tmltool1abc" {
		t.Errorf("unexpected address: %q", addr)
	}
}

func TestShowReceiveAddresses(t *testing.T) {
	addrs := []wallet.AddressWithUsage{
		{Address: "tmltool1abc", Used: false, Coins: wallet.Amount{Atoms: "0"}},
		{Address: "tmltool1def", Used: true, Coins: wallet.Amount{Atoms: "5000000000"}},
	}
	srv := rpcHandler(t, addrs)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.ShowReceiveAddresses(context.Background(), 0)
	if err != nil {
		t.Fatalf("ShowReceiveAddresses: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 addresses, got %d", len(got))
	}
	if got[0].Address != "tmltool1abc" {
		t.Errorf("unexpected first address: %q", got[0].Address)
	}
}

func TestRevealPublicKey(t *testing.T) {
	result := wallet.RevealPublicKeyResult{
		PublicKeyHex:     "02aabbccdd",
		PublicKeyAddress: "tmltool1pubkey",
	}
	srv := rpcHandler(t, result)
	defer srv.Close()

	c := wallet.New(srv.URL)
	hex, err := c.RevealPublicKey(context.Background(), 0, "tmltool1abc")
	if err != nil {
		t.Fatalf("RevealPublicKey: %v", err)
	}
	if hex != "02aabbccdd" {
		t.Errorf("unexpected hex: %q", hex)
	}
}

func TestEncryptPrivateKeys(t *testing.T) {
	srv := rpcHandler(t, nil)
	defer srv.Close()

	c := wallet.New(srv.URL)
	if err := c.EncryptPrivateKeys(context.Background(), "s3cr3t"); err != nil {
		t.Fatalf("EncryptPrivateKeys: %v", err)
	}
}

func TestUnlockPrivateKeys(t *testing.T) {
	srv := rpcHandler(t, nil)
	defer srv.Close()

	c := wallet.New(srv.URL)
	if err := c.UnlockPrivateKeys(context.Background(), "s3cr3t"); err != nil {
		t.Fatalf("UnlockPrivateKeys: %v", err)
	}
}

func TestLockPrivateKeys(t *testing.T) {
	srv := rpcHandler(t, nil)
	defer srv.Close()

	c := wallet.New(srv.URL)
	if err := c.LockPrivateKeys(context.Background()); err != nil {
		t.Fatalf("LockPrivateKeys: %v", err)
	}
}

// --- Transactions ---

func TestAddressSend(t *testing.T) {
	result := wallet.SendResult{
		TxID: "cafebabe",
		Fees: wallet.FeesBreakdown{
			Coins:  wallet.Amount{Atoms: "10000", Decimal: "0.0001"},
			Tokens: map[string]wallet.Amount{},
		},
		Broadcasted: true,
	}
	srv := rpcHandler(t, result)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.AddressSend(context.Background(), wallet.SendParams{
		Account:       0,
		Address:       "tmltool1dest",
		Amount:        wallet.Amount{Decimal: "10.5"},
		SelectedUTXOs: []wallet.Outpoint{},
	})
	if err != nil {
		t.Fatalf("AddressSend: %v", err)
	}
	if got.TxID != "cafebabe" {
		t.Errorf("unexpected tx_id: %q", got.TxID)
	}
	if !got.Broadcasted {
		t.Error("expected broadcasted=true")
	}
}

func TestTokenSend(t *testing.T) {
	result := wallet.SendResult{TxID: "aabbccdd", Broadcasted: true}
	srv := rpcHandler(t, result)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.TokenSend(context.Background(), wallet.TokenSendParams{
		Account: 0,
		TokenID: "mytoken1abc",
		Address: "tmltool1dest",
		Amount:  wallet.Amount{Decimal: "100"},
	})
	if err != nil {
		t.Fatalf("TokenSend: %v", err)
	}
	if got.TxID != "aabbccdd" {
		t.Errorf("unexpected tx_id: %q", got.TxID)
	}
}

func TestSweepSpendable(t *testing.T) {
	srv := rpcHandler(t, wallet.SendResult{TxID: "sweep01", Broadcasted: true})
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.SweepSpendable(context.Background(), wallet.SweepParams{
		Account:            0,
		DestinationAddress: "tmltool1dest",
		FromAddresses:      []string{},
		All:                true,
	})
	if err != nil {
		t.Fatalf("SweepSpendable: %v", err)
	}
	if got.TxID != "sweep01" {
		t.Errorf("unexpected tx_id: %q", got.TxID)
	}
}

func TestSubmitTransaction(t *testing.T) {
	srv := rpcHandler(t, wallet.SubmitResult{TxID: "submitted01"})
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.SubmitTransaction(context.Background(), "cafebabe01020304", false)
	if err != nil {
		t.Fatalf("SubmitTransaction: %v", err)
	}
	if got.TxID != "submitted01" {
		t.Errorf("unexpected tx_id: %q", got.TxID)
	}
}

func TestListPendingTransactions(t *testing.T) {
	srv := rpcHandler(t, []string{"tx1", "tx2"})
	defer srv.Close()

	c := wallet.New(srv.URL)
	txs, err := c.ListPendingTransactions(context.Background(), 0)
	if err != nil {
		t.Fatalf("ListPendingTransactions: %v", err)
	}
	if len(txs) != 2 {
		t.Fatalf("expected 2 pending txs, got %d", len(txs))
	}
}

func TestListTransactionsByAddress(t *testing.T) {
	txs := []wallet.WalletTx{
		{ID: "tx1", Height: 100, Timestamp: wallet.Timestamp{Timestamp: 1700000000}},
	}
	srv := rpcHandler(t, txs)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.ListTransactionsByAddress(context.Background(), 0, nil, 20)
	if err != nil {
		t.Fatalf("ListTransactionsByAddress: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 tx, got %d", len(got))
	}
	if got[0].ID != "tx1" {
		t.Errorf("unexpected tx id: %q", got[0].ID)
	}
}

func TestGetTransaction(t *testing.T) {
	raw := json.RawMessage(`{"id":"tx1","height":100}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct{ ID uint64 `json:"id"` }
		_ = json.NewDecoder(r.Body).Decode(&req)
		resp := map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": raw}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.GetTransaction(context.Background(), 0, "tx1")
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if string(got) == "" {
		t.Error("expected non-empty result")
	}
}

func TestAbandonTransaction(t *testing.T) {
	srv := rpcHandler(t, nil)
	defer srv.Close()

	c := wallet.New(srv.URL)
	if err := c.AbandonTransaction(context.Background(), 0, "tx1"); err != nil {
		t.Fatalf("AbandonTransaction: %v", err)
	}
}

func TestComposeTransaction(t *testing.T) {
	result := wallet.ComposedTx{
		Hex:  "deadbeef",
		Fees: wallet.FeesBreakdown{Coins: wallet.Amount{Atoms: "5000"}},
	}
	srv := rpcHandler(t, result)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.ComposeTransaction(context.Background(), wallet.ComposeParams{
		Inputs:          []wallet.Outpoint{},
		Outputs:         []json.RawMessage{},
		OnlyTransaction: false,
	})
	if err != nil {
		t.Fatalf("ComposeTransaction: %v", err)
	}
	if got.Hex != "deadbeef" {
		t.Errorf("unexpected hex: %q", got.Hex)
	}
}

func TestSignRawTransaction(t *testing.T) {
	result := wallet.SignedTx{
		Hex:               "signed01",
		CurrentSignatures: json.RawMessage(`[]`),
	}
	srv := rpcHandler(t, result)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.SignRawTransaction(context.Background(), 0, "unsigned01")
	if err != nil {
		t.Fatalf("SignRawTransaction: %v", err)
	}
	if got.Hex != "signed01" {
		t.Errorf("unexpected hex: %q", got.Hex)
	}
}

func TestInspectTransaction(t *testing.T) {
	result := wallet.TxInspection{
		Stats: wallet.TxStats{NumInputs: 2, TotalSignatures: 2},
	}
	srv := rpcHandler(t, result)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.InspectTransaction(context.Background(), "cafebabe")
	if err != nil {
		t.Fatalf("InspectTransaction: %v", err)
	}
	if got.Stats.NumInputs != 2 {
		t.Errorf("expected 2 inputs, got %d", got.Stats.NumInputs)
	}
}

func TestDepositData(t *testing.T) {
	srv := rpcHandler(t, wallet.SendResult{TxID: "data01", Broadcasted: true})
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.DepositData(context.Background(), 0, "68656c6c6f")
	if err != nil {
		t.Fatalf("DepositData: %v", err)
	}
	if got.TxID != "data01" {
		t.Errorf("unexpected tx_id: %q", got.TxID)
	}
}

// --- Staking ---

func TestCreateStakePool(t *testing.T) {
	srv := rpcHandler(t, wallet.SendResult{TxID: "pool01", Broadcasted: true})
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.CreateStakePool(context.Background(), wallet.CreatePoolParams{
		Account:                0,
		Amount:                 wallet.Amount{Decimal: "40000"},
		CostPerBlock:           wallet.Amount{Decimal: "1"},
		MarginRatioPerThousand: "5%",
		DecommissionAddress:    "tmltool1decom",
	})
	if err != nil {
		t.Fatalf("CreateStakePool: %v", err)
	}
	if got.TxID != "pool01" {
		t.Errorf("unexpected tx_id: %q", got.TxID)
	}
}

func TestDecommissionStakePool(t *testing.T) {
	srv := rpcHandler(t, wallet.SendResult{TxID: "decom01", Broadcasted: true})
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.DecommissionStakePool(context.Background(), wallet.DecommissionParams{
		Account:       0,
		PoolID:        "pool1abc",
		OutputAddress: "tmltool1dest",
	})
	if err != nil {
		t.Fatalf("DecommissionStakePool: %v", err)
	}
	if got.TxID != "decom01" {
		t.Errorf("unexpected tx_id: %q", got.TxID)
	}
}

func TestListOwnedPools(t *testing.T) {
	pools := []wallet.OwnedPool{
		{
			PoolID:                 "pool1abc",
			Pledge:                 wallet.Amount{Atoms: "40000000000000"},
			Balance:                wallet.Amount{Atoms: "50000000000000"},
			MarginRatioPerThousand: "5%",
			CostPerBlock:           wallet.Amount{Atoms: "100000000"},
		},
	}
	srv := rpcHandler(t, pools)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.ListOwnedPools(context.Background(), 0)
	if err != nil {
		t.Fatalf("ListOwnedPools: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(got))
	}
	if got[0].PoolID != "pool1abc" {
		t.Errorf("unexpected pool_id: %q", got[0].PoolID)
	}
}

func TestGetPoolBalance(t *testing.T) {
	balance := wallet.Amount{Atoms: "50000000000000", Decimal: "500000.0"}
	srv := rpcHandler(t, map[string]any{"balance": balance})
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.GetPoolBalance(context.Background(), 0, "pool1abc")
	if err != nil {
		t.Fatalf("GetPoolBalance: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil balance")
	}
	if got.Atoms != "50000000000000" {
		t.Errorf("unexpected atoms: %q", got.Atoms)
	}
}

func TestStartStaking(t *testing.T) {
	srv := rpcHandler(t, nil)
	defer srv.Close()

	c := wallet.New(srv.URL)
	if err := c.StartStaking(context.Background(), 0); err != nil {
		t.Fatalf("StartStaking: %v", err)
	}
}

func TestStopStaking(t *testing.T) {
	srv := rpcHandler(t, nil)
	defer srv.Close()

	c := wallet.New(srv.URL)
	if err := c.StopStaking(context.Background(), 0); err != nil {
		t.Fatalf("StopStaking: %v", err)
	}
}

func TestGetStakingStatus(t *testing.T) {
	srv := rpcHandler(t, "Staking")
	defer srv.Close()

	c := wallet.New(srv.URL)
	status, err := c.GetStakingStatus(context.Background(), 0)
	if err != nil {
		t.Fatalf("GetStakingStatus: %v", err)
	}
	if *status != wallet.StakingStatusActive {
		t.Errorf("expected Staking, got %q", *status)
	}
}

func TestCreateDelegation(t *testing.T) {
	result := wallet.CreateDelegationResult{DelegationID: "deleg1abc", TxID: "delegtx01"}
	srv := rpcHandler(t, result)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.CreateDelegation(context.Background(), wallet.CreateDelegationParams{
		Account: 0,
		Address: "tmltool1owner",
		PoolID:  "pool1abc",
	})
	if err != nil {
		t.Fatalf("CreateDelegation: %v", err)
	}
	if got.DelegationID != "deleg1abc" {
		t.Errorf("unexpected delegation_id: %q", got.DelegationID)
	}
}

func TestDelegateStaking(t *testing.T) {
	srv := rpcHandler(t, wallet.SendResult{TxID: "stake01", Broadcasted: true})
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.DelegateStaking(context.Background(), wallet.DelegateParams{
		Account:      0,
		Amount:       wallet.Amount{Decimal: "1000"},
		DelegationID: "deleg1abc",
	})
	if err != nil {
		t.Fatalf("DelegateStaking: %v", err)
	}
	if got.TxID != "stake01" {
		t.Errorf("unexpected tx_id: %q", got.TxID)
	}
}

func TestWithdrawFromDelegation(t *testing.T) {
	srv := rpcHandler(t, wallet.SendResult{TxID: "withdraw01", Broadcasted: true})
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.WithdrawFromDelegation(context.Background(), wallet.WithdrawParams{
		Account:      0,
		Address:      "tmltool1dest",
		Amount:       wallet.Amount{Decimal: "500"},
		DelegationID: "deleg1abc",
	})
	if err != nil {
		t.Fatalf("WithdrawFromDelegation: %v", err)
	}
	if got.TxID != "withdraw01" {
		t.Errorf("unexpected tx_id: %q", got.TxID)
	}
}

func TestListDelegations(t *testing.T) {
	delegations := []wallet.DelegationInfo{
		{DelegationID: "deleg1abc", PoolID: "pool1abc", Balance: wallet.Amount{Atoms: "1000000000000"}},
	}
	srv := rpcHandler(t, delegations)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.ListDelegations(context.Background(), 0)
	if err != nil {
		t.Fatalf("ListDelegations: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 delegation, got %d", len(got))
	}
	if got[0].DelegationID != "deleg1abc" {
		t.Errorf("unexpected delegation_id: %q", got[0].DelegationID)
	}
}

// --- Tokens ---

func TestIssueToken(t *testing.T) {
	result := wallet.IssueTokenResult{TokenID: "tok1abc", TxID: "issue01"}
	srv := rpcHandler(t, result)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.IssueToken(context.Background(), wallet.IssueTokenParams{
		Account:            0,
		DestinationAddress: "tmltool1auth",
		Metadata: wallet.TokenMetadata{
			TokenTicker:      "MYTKN",
			NumberOfDecimals: 8,
			MetadataURI:      "https://example.com/token",
			TokenSupply:      wallet.TokenSupply{Type: "Lockable"},
			IsFreezable:      false,
		},
	})
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if got.TokenID != "tok1abc" {
		t.Errorf("unexpected token_id: %q", got.TokenID)
	}
}

func TestIssueNFT(t *testing.T) {
	result := wallet.IssueTokenResult{TokenID: "nft1abc", TxID: "nftissue01"}
	srv := rpcHandler(t, result)
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.IssueNFT(context.Background(), wallet.IssueNFTParams{
		Account:            0,
		DestinationAddress: "tmltool1dest",
		Metadata: wallet.NFTMetadata{
			MediaHash:   "a3f1e2d9c4",
			Name:        "Sunset #1",
			Description: "A photograph of a sunset",
			Ticker:      "SUNST",
		},
	})
	if err != nil {
		t.Fatalf("IssueNFT: %v", err)
	}
	if got.TokenID != "nft1abc" {
		t.Errorf("unexpected token_id: %q", got.TokenID)
	}
}

func TestMintTokens(t *testing.T) {
	srv := rpcHandler(t, wallet.SendResult{TxID: "mint01", Broadcasted: true})
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.MintTokens(context.Background(), wallet.MintParams{
		Account: 0,
		TokenID: "tok1abc",
		Address: "tmltool1dest",
		Amount:  wallet.Amount{Decimal: "1000000"},
	})
	if err != nil {
		t.Fatalf("MintTokens: %v", err)
	}
	if got.TxID != "mint01" {
		t.Errorf("unexpected tx_id: %q", got.TxID)
	}
}

func TestUnmintTokens(t *testing.T) {
	srv := rpcHandler(t, wallet.SendResult{TxID: "unmint01", Broadcasted: true})
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.UnmintTokens(context.Background(), wallet.UnmintParams{
		Account: 0,
		TokenID: "tok1abc",
		Amount:  wallet.Amount{Decimal: "5000"},
	})
	if err != nil {
		t.Fatalf("UnmintTokens: %v", err)
	}
	if got.TxID != "unmint01" {
		t.Errorf("unexpected tx_id: %q", got.TxID)
	}
}

func TestLockTokenSupply(t *testing.T) {
	srv := rpcHandler(t, wallet.SendResult{TxID: "lock01", Broadcasted: true})
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.LockTokenSupply(context.Background(), wallet.LockSupplyParams{
		AccountIndex: 0,
		TokenID:      "tok1abc",
	})
	if err != nil {
		t.Fatalf("LockTokenSupply: %v", err)
	}
	if got.TxID != "lock01" {
		t.Errorf("unexpected tx_id: %q", got.TxID)
	}
}

func TestFreezeToken(t *testing.T) {
	srv := rpcHandler(t, wallet.SendResult{TxID: "freeze01", Broadcasted: true})
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.FreezeToken(context.Background(), wallet.FreezeParams{
		Account:       0,
		TokenID:       "tok1abc",
		IsUnfreezable: true,
	})
	if err != nil {
		t.Fatalf("FreezeToken: %v", err)
	}
	if got.TxID != "freeze01" {
		t.Errorf("unexpected tx_id: %q", got.TxID)
	}
}

func TestUnfreezeToken(t *testing.T) {
	srv := rpcHandler(t, wallet.SendResult{TxID: "unfreeze01", Broadcasted: true})
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.UnfreezeToken(context.Background(), wallet.UnfreezeParams{
		Account: 0,
		TokenID: "tok1abc",
	})
	if err != nil {
		t.Fatalf("UnfreezeToken: %v", err)
	}
	if got.TxID != "unfreeze01" {
		t.Errorf("unexpected tx_id: %q", got.TxID)
	}
}

func TestChangeTokenAuthority(t *testing.T) {
	srv := rpcHandler(t, wallet.SendResult{TxID: "chauth01", Broadcasted: true})
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.ChangeTokenAuthority(context.Background(), wallet.ChangeAuthorityParams{
		Account: 0,
		TokenID: "tok1abc",
		Address: "tmltool1newauth",
	})
	if err != nil {
		t.Fatalf("ChangeTokenAuthority: %v", err)
	}
	if got.TxID != "chauth01" {
		t.Errorf("unexpected tx_id: %q", got.TxID)
	}
}

func TestSendToken(t *testing.T) {
	srv := rpcHandler(t, wallet.SendResult{TxID: "sendtok01", Broadcasted: true})
	defer srv.Close()

	c := wallet.New(srv.URL)
	got, err := c.SendToken(context.Background(), wallet.TokenSendParams{
		Account: 0,
		TokenID: "tok1abc",
		Address: "tmltool1dest",
		Amount:  wallet.Amount{Decimal: "100"},
	})
	if err != nil {
		t.Fatalf("SendToken: %v", err)
	}
	if got.TxID != "sendtok01" {
		t.Errorf("unexpected tx_id: %q", got.TxID)
	}
}

// --- Concurrent ID generation ---

func TestConcurrentCalls(t *testing.T) {
	srv := rpcHandler(t, wallet.BestBlock{Height: 1, ID: "aa"})
	defer srv.Close()

	c := wallet.New(srv.URL)
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := c.BestBlock(context.Background())
			done <- err
		}()
	}
	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent BestBlock: %v", err)
		}
	}
}
