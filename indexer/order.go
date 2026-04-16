package indexer

import (
	"context"
	"fmt"
)

// ListOrders returns active orders with pagination.
func (c *Client) ListOrders(ctx context.Context, opts PageOpts) ([]Order, error) {
	var result []Order
	if err := c.get(ctx, "/order", pageQuery(opts), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetOrder returns a single order by id (bech32).
func (c *Client) GetOrder(ctx context.Context, id string) (*Order, error) {
	var result Order
	if err := c.get(ctx, fmt.Sprintf("/order/%s", id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListOrdersByPair returns orders for a trading pair with pagination.
// askCurrency and giveCurrency are either the coin ticker (e.g. "ML") or a token id (bech32).
func (c *Client) ListOrdersByPair(ctx context.Context, askCurrency, giveCurrency string, opts PageOpts) ([]Order, error) {
	var result []Order
	path := fmt.Sprintf("/order/pair/%s_%s", askCurrency, giveCurrency)
	if err := c.get(ctx, path, pageQuery(opts), &result); err != nil {
		return nil, err
	}
	return result, nil
}
