package node

import "encoding/json"

// Amount represents a Mintlayer token/coin amount expressed in atoms.
// The atoms field is a decimal string to avoid JavaScript integer overflow.
type Amount struct {
	Atoms string `json:"atoms"`
}

// Timestamp wraps a Unix seconds timestamp returned by the node.
type Timestamp struct {
	Timestamp int64 `json:"timestamp"`
}

// ChainstateInfo is returned by ChainstateInfo.
type ChainstateInfo struct {
	BestBlockHeight        uint64    `json:"best_block_height"`
	BestBlockID            string    `json:"best_block_id"`
	BestBlockTimestamp     Timestamp `json:"best_block_timestamp"`
	MedianTime             Timestamp `json:"median_time"`
	IsInitialBlockDownload bool      `json:"is_initial_block_download"`
}

// OutpointSourceID identifies the transaction or block reward that produced a UTXO.
// Set Type to "Transaction" and Content to a JSON object {"tx_id": "hex…"},
// or Type to "BlockReward" and Content to {"block_id": "hex…"}.
type OutpointSourceID struct {
	Type    string          `json:"type"`
	Content json.RawMessage `json:"content"`
}

// Outpoint identifies a specific output within a transaction or block reward.
type Outpoint struct {
	SourceID OutpointSourceID `json:"source_id"`
	Index    uint32           `json:"index"`
}

// TxSourceContent is a helper for constructing Transaction-type OutpointSourceIDs.
type TxSourceContent struct {
	TxID string `json:"tx_id"`
}

// BlockSourceContent is a helper for constructing BlockReward-type OutpointSourceIDs.
type BlockSourceContent struct {
	BlockID string `json:"block_id"`
}

// TokenInfo is returned by TokenInfo and TokensInfo.
// The Type field is "FungibleToken" or "NonFungibleToken".
// Content holds the raw JSON payload specific to each token type.
type TokenInfo struct {
	Type    string          `json:"type"`
	Content json.RawMessage `json:"content"`
}

// OrderInfo is returned by OrderInfo and OrdersInfoByCurrencies.
type OrderInfo struct {
	ConcludeKey    string          `json:"conclude_key"`
	InitiallyAsked json.RawMessage `json:"initially_asked"`
	InitiallyGiven json.RawMessage `json:"initially_given"`
	AskBalance     Amount          `json:"ask_balance"`
	GiveBalance    Amount          `json:"give_balance"`
	Nonce          uint64          `json:"nonce"`
	IsFrozen       bool            `json:"is_frozen"`
}

// Currency is used as a query parameter for OrdersInfoByCurrencies.
// Set Type to "Coin" (Content omitted) or "Token" (Content = token-id bech32 string).
type Currency struct {
	Type    string  `json:"type"`
	Content *string `json:"content,omitempty"`
}

// TrustPolicy controls fee-check strictness when submitting transactions.
type TrustPolicy string

const (
	// TrustPolicyTrusted skips some fee checks. Use only when you control the transaction.
	TrustPolicyTrusted TrustPolicy = "Trusted"
	// TrustPolicyUntrusted applies full validation. Recommended for all external transactions.
	TrustPolicyUntrusted TrustPolicy = "Untrusted"
)

// MempoolTx is returned by GetTransaction (mempool).
type MempoolTx struct {
	// ID is the transaction id as a hex string.
	ID string `json:"id"`
	// Status is one of: "InMempool", "InMempoolDuplicate", "InOrphanPool", "InOrphanPoolDuplicate".
	Status      string `json:"status"`
	Transaction string `json:"transaction"` // hex-encoded raw bytes
}

// FeeRate represents a fee rate as atoms per kilobyte.
type FeeRate struct {
	AmountPerKB Amount `json:"amount_per_kb"`
}

// FeeRatePoint is one point on the mempool fee-rate curve.
// Size is the cumulative transaction size in bytes; Rate is the fee rate at that point.
type FeeRatePoint struct {
	Size uint64
	Rate FeeRate
}

// UnmarshalJSON implements json.Unmarshaler.
// The wire format is a 2-element array: [size, {"amount_per_kb": {...}}].
func (f *FeeRatePoint) UnmarshalJSON(data []byte) error {
	var raw [2]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[0], &f.Size); err != nil {
		return err
	}
	return json.Unmarshal(raw[1], &f.Rate)
}

// PeerInfo describes a connected peer.
type PeerInfo struct {
	PeerID uint64 `json:"peer_id"`
	// Address is the remote address in "host:port" form.
	Address string `json:"address"`
	// PeerRole is one of: "Inbound", "OutboundFullRelay", "OutboundBlockRelay",
	// "OutboundReserved", "OutboundManual", "Feeler".
	PeerRole        string `json:"peer_role"`
	BanScore        uint32 `json:"ban_score"`
	UserAgent       string `json:"user_agent"`
	SoftwareVersion string `json:"software_version"`
	// PingWait, PingLast, PingMin are milliseconds; nil when not yet measured.
	PingWait         *int64 `json:"ping_wait"`
	PingLast         *int64 `json:"ping_last"`
	PingMin          *int64 `json:"ping_min"`
	LastTipBlockTime *int64 `json:"last_tip_block_time"`
}

// BannedPeer is one entry from ListBanned.
type BannedPeer struct {
	// Address is the banned IP address.
	Address string
	// BanTime is [seconds, nanoseconds] Unix epoch of ban expiry.
	BanTime [2]int64
}

// UnmarshalJSON implements json.Unmarshaler.
// Wire format: ["address", {"time": [secs, nanos]}].
func (b *BannedPeer) UnmarshalJSON(data []byte) error {
	var raw [2]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[0], &b.Address); err != nil {
		return err
	}
	var tw struct {
		Time [2]int64 `json:"time"`
	}
	if err := json.Unmarshal(raw[1], &tw); err != nil {
		return err
	}
	b.BanTime = tw.Time
	return nil
}
