package indexer

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// GetCoinStatistics returns aggregate ML coin supply statistics.
func (c *Client) GetCoinStatistics(ctx context.Context) (*CoinStats, error) {
	var result CoinStats
	if err := c.get(ctx, "/statistics/coin", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetTokenStatistics returns aggregate supply statistics for a specific token (bech32 id).
func (c *Client) GetTokenStatistics(ctx context.Context, tokenID string) (*CoinStats, error) {
	var result CoinStats
	if err := c.get(ctx, fmt.Sprintf("/statistics/token/%s", tokenID), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetFeeRate returns the current fee rate (atoms per kilobyte) required to be in the
// top inTopXMb megabytes of the mempool. Pass 0 to use the server default (5 MB).
func (c *Client) GetFeeRate(ctx context.Context, inTopXMb uint32) (string, error) {
	q := url.Values{}
	if inTopXMb > 0 {
		q.Set("in_top_x_mb", strconv.FormatUint(uint64(inTopXMb), 10))
	}
	var result string
	if err := c.get(ctx, "/feerate", q, &result); err != nil {
		return "", err
	}
	return result, nil
}
