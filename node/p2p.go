package node

import (
	"context"
	"time"
)

// GetPeerCount returns the number of currently connected peers.
func (c *Client) GetPeerCount(ctx context.Context) (uint64, error) {
	var result uint64
	if err := c.call(ctx, "p2p_get_peer_count", struct{}{}, &result); err != nil {
		return 0, err
	}
	return result, nil
}

// GetConnectedPeers returns details about all currently connected peers.
func (c *Client) GetConnectedPeers(ctx context.Context) ([]PeerInfo, error) {
	var result []PeerInfo
	if err := c.call(ctx, "p2p_get_connected_peers", struct{}{}, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetBindAddresses returns the p2p listen addresses (host:port) of this node.
func (c *Client) GetBindAddresses(ctx context.Context) ([]string, error) {
	var result []string
	if err := c.call(ctx, "p2p_get_bind_addresses", struct{}{}, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// AddReservedNode adds an address to the reserved-node list.
// The node maintains a persistent outbound connection to reserved peers.
func (c *Client) AddReservedNode(ctx context.Context, addr string) error {
	params := struct {
		Addr string `json:"addr"`
	}{Addr: addr}
	return c.call(ctx, "p2p_add_reserved_node", params, nil)
}

// RemoveReservedNode removes an address from the reserved-node list.
// The existing connection (if any) is not closed immediately.
func (c *Client) RemoveReservedNode(ctx context.Context, addr string) error {
	params := struct {
		Addr string `json:"addr"`
	}{Addr: addr}
	return c.call(ctx, "p2p_remove_reserved_node", params, nil)
}

// Connect attempts a one-time outbound connection to addr.
// Unlike AddReservedNode the connection is not persistent.
func (c *Client) Connect(ctx context.Context, addr string) error {
	params := struct {
		Addr string `json:"addr"`
	}{Addr: addr}
	return c.call(ctx, "p2p_connect", params, nil)
}

// Disconnect closes the connection to a peer identified by peerID.
// If it was an outbound connection, the address is removed from the peer database.
func (c *Client) Disconnect(ctx context.Context, peerID uint64) error {
	params := struct {
		PeerID uint64 `json:"peer_id"`
	}{PeerID: peerID}
	return c.call(ctx, "p2p_disconnect", params, nil)
}

// ListBanned returns all banned peer addresses together with their ban expiry time.
func (c *Client) ListBanned(ctx context.Context) ([]BannedPeer, error) {
	var result []BannedPeer
	if err := c.call(ctx, "p2p_list_banned", struct{}{}, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Ban bans the peer at address for the given duration.
// The wire format sends duration as [seconds, nanoseconds].
func (c *Client) Ban(ctx context.Context, address string, duration time.Duration) error {
	secs := int64(duration / time.Second)
	nanos := int64(duration % time.Second)
	params := struct {
		Address  string   `json:"address"`
		Duration [2]int64 `json:"duration"`
	}{
		Address:  address,
		Duration: [2]int64{secs, nanos},
	}
	return c.call(ctx, "p2p_ban", params, nil)
}

// Unban removes the ban on a previously banned peer address.
func (c *Client) Unban(ctx context.Context, address string) error {
	params := struct {
		Address string `json:"address"`
	}{Address: address}
	return c.call(ctx, "p2p_unban", params, nil)
}

// P2PSubmitTransaction submits a signed transaction to the mempool AND broadcasts
// it to the P2P network. This is the correct call for propagating a transaction
// to the Mintlayer network.
func (c *Client) P2PSubmitTransaction(ctx context.Context, txHex string, trustPolicy TrustPolicy) error {
	params := struct {
		Tx      string `json:"tx"`
		Options struct {
			TrustPolicy TrustPolicy `json:"trust_policy"`
		} `json:"options"`
	}{}
	params.Tx = txHex
	params.Options.TrustPolicy = trustPolicy
	return c.call(ctx, "p2p_submit_transaction", params, nil)
}
