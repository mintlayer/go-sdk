// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mintlayer

// EncodeLockForBlockCount encodes a "lock until N more blocks have passed" timelock.
func (c *Client) EncodeLockForBlockCount(blockCount uint64) ([]byte, error) {
	return c.callReturnBytesNoErr("encode_lock_for_block_count", blockCount)
}

// EncodeLockForSeconds encodes a "lock for N more seconds" timelock.
func (c *Client) EncodeLockForSeconds(seconds uint64) ([]byte, error) {
	return c.callReturnBytesNoErr("encode_lock_for_seconds", seconds)
}

// EncodeLockUntilHeight encodes a "lock until absolute block height" timelock.
func (c *Client) EncodeLockUntilHeight(blockHeight uint64) ([]byte, error) {
	return c.callReturnBytesNoErr("encode_lock_until_height", blockHeight)
}

// EncodeLockUntilTime encodes a "lock until absolute UNIX timestamp" timelock.
func (c *Client) EncodeLockUntilTime(timestampSeconds uint64) ([]byte, error) {
	return c.callReturnBytesNoErr("encode_lock_until_time", timestampSeconds)
}
