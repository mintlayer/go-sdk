// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sdk

import (
	"context"
	"sync"

	"github.com/mintlayer/go-sdk/indexer"
	"github.com/mintlayer/go-sdk/node"
	mintlayer "github.com/mintlayer/go-sdk/wasm"
	"github.com/mintlayer/go-sdk/wallet"
)

// ── Convenience re-exports ────────────────────────────────────────────────────
//
// Callers that only need the top-level client can import "github.com/mintlayer/go-sdk"
// and use these aliases without also importing the sub-packages.

type (
	Amount           = mintlayer.Amount
	Network          = mintlayer.Network
	SignatureHashType = mintlayer.SignatureHashType
	SourceId         = mintlayer.SourceId
	TotalSupply      = mintlayer.TotalSupply
	FreezableToken   = mintlayer.FreezableToken
	TokenUnfreezable = mintlayer.TokenUnfreezable
	TxAdditionalInfo = mintlayer.TxAdditionalInfo
	PoolInfo         = mintlayer.PoolInfo
	OrderInfo        = mintlayer.OrderInfo
)

// Network constants.
const (
	Mainnet = mintlayer.Mainnet
	Testnet = mintlayer.Testnet
	Regtest = mintlayer.Regtest
	Signet  = mintlayer.Signet
)

// Amount constructors.
var (
	NewAmount     = mintlayer.NewAmount
	NewAmountZero = mintlayer.NewAmountZero
)

// ── Config ────────────────────────────────────────────────────────────────────

// Config holds the top-level SDK configuration.
// Only sub-clients whose URL field is non-empty are constructed.
type Config struct {
	// NodeURL is the base URL of the Mintlayer node daemon
	// (e.g. "http://127.0.0.1:3030"). Leave empty to skip.
	NodeURL string

	// IndexerURL is the base URL of the indexer (api-web-server)
	// (e.g. "http://127.0.0.1:3000"). Leave empty to skip.
	IndexerURL string

	// WalletURL is the base URL of the wallet RPC daemon
	// (e.g. "http://127.0.0.1:3034"). Leave empty to skip.
	WalletURL string

	// Username and Password are used for HTTP Basic Auth on Node and Wallet.
	Username string
	Password string
}

// ── Client ────────────────────────────────────────────────────────────────────

// Client is the top-level Mintlayer SDK client.
//
// Each sub-client field is nil when its corresponding URL was not set in Config.
// WASM is nil until [Client.InitWASM] is called.
type Client struct {
	Node    *node.Client
	Indexer *indexer.Client
	Wallet  *wallet.Client
	WASM    *mintlayer.Client // nil until InitWASM

	mu   sync.Mutex
	wctx context.Context
}

// New creates a Client, constructing only the sub-clients whose URL is set in cfg.
func New(cfg Config) *Client {
	c := &Client{}

	if cfg.NodeURL != "" {
		var opts []node.Option
		if cfg.Username != "" {
			opts = append(opts, node.WithBasicAuth(cfg.Username, cfg.Password))
		}
		c.Node = node.New(cfg.NodeURL, opts...)
	}

	if cfg.IndexerURL != "" {
		c.Indexer = indexer.New(cfg.IndexerURL)
	}

	if cfg.WalletURL != "" {
		var opts []wallet.Option
		if cfg.Username != "" {
			opts = append(opts, wallet.WithBasicAuth(cfg.Username, cfg.Password))
		}
		c.Wallet = wallet.New(cfg.WalletURL, opts...)
	}

	return c
}

// InitWASM initialises the embedded WASM cryptography runtime (~400ms).
// Subsequent calls are no-ops. Safe for concurrent use.
func (c *Client) InitWASM(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.WASM != nil {
		return nil
	}
	wc, err := mintlayer.New(ctx)
	if err != nil {
		return err
	}
	c.WASM = wc
	c.wctx = ctx
	return nil
}

// Close releases WASM resources if [Client.InitWASM] was called.
// HTTP sub-clients (Node, Indexer, Wallet) require no teardown.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.WASM == nil {
		return nil
	}
	err := c.WASM.Close()
	c.WASM = nil
	return err
}
