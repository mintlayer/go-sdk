// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package indexer

import (
	"context"
	"fmt"
)

// GetDelegation returns a single delegation by id (bech32).
func (c *Client) GetDelegation(ctx context.Context, id string) (*Delegation, error) {
	var result Delegation
	if err := c.get(ctx, fmt.Sprintf("/delegation/%s", id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
