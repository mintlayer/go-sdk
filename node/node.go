package node

import "context"

// NodeVersion returns the node software version string (e.g. "1.3.0").
func (c *Client) NodeVersion(ctx context.Context) (string, error) {
	var result string
	if err := c.call(ctx, "node_version", struct{}{}, &result); err != nil {
		return "", err
	}
	return result, nil
}

// NodeShutdown orders the node daemon to shut down gracefully.
func (c *Client) NodeShutdown(ctx context.Context) error {
	return c.call(ctx, "node_shutdown", struct{}{}, nil)
}
