package wallet

import "context"

// CreateWallet creates a new wallet file at the given path.
// If no mnemonic is supplied in params, the daemon generates one and returns it in the result.
func (c *Client) CreateWallet(ctx context.Context, params CreateWalletParams) (*CreateWalletResult, error) {
	var result CreateWalletResult
	if err := c.call(ctx, "wallet_create", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RecoverWallet restores a wallet from a BIP-39 mnemonic phrase.
func (c *Client) RecoverWallet(ctx context.Context, params RecoverWalletParams) error {
	return c.call(ctx, "wallet_recover", params, nil)
}

// OpenWallet opens an existing wallet file. Pass an empty password for unencrypted wallets.
func (c *Client) OpenWallet(ctx context.Context, path, password string) error {
	var pass *string
	if password != "" {
		pass = &password
	}
	params := struct {
		Path                  string  `json:"path"`
		Password              *string `json:"password"`
		ForceMigrateWalletType *string `json:"force_migrate_wallet_type"`
		HardwareWallet        *string `json:"hardware_wallet"`
	}{
		Path:     path,
		Password: pass,
	}
	return c.call(ctx, "wallet_open", params, nil)
}

// CloseWallet closes the currently open wallet.
func (c *Client) CloseWallet(ctx context.Context) error {
	return c.call(ctx, "wallet_close", struct{}{}, nil)
}

// GetWalletInfo returns information about the open wallet: id, account names, and wallet type.
func (c *Client) GetWalletInfo(ctx context.Context) (*WalletInfo, error) {
	var result WalletInfo
	if err := c.call(ctx, "wallet_info", struct{}{}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SyncWallet scans remaining blocks from the node until the chain tip.
func (c *Client) SyncWallet(ctx context.Context) error {
	return c.call(ctx, "wallet_sync", struct{}{}, nil)
}

// RescanWallet rescans the entire blockchain from genesis.
func (c *Client) RescanWallet(ctx context.Context) error {
	return c.call(ctx, "wallet_rescan", struct{}{}, nil)
}

// BestBlock returns the best block the wallet is currently synced to.
func (c *Client) BestBlock(ctx context.Context) (*BestBlock, error) {
	var result BestBlock
	if err := c.call(ctx, "wallet_best_block", struct{}{}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateAccount creates a new account. Fails if the last account has no transaction history.
func (c *Client) CreateAccount(ctx context.Context, name string) (*AccountInfo, error) {
	var result AccountInfo
	params := struct {
		Name string `json:"name"`
	}{Name: name}
	if err := c.call(ctx, "account_create", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RenameAccount renames an account. Pass an empty name to remove the name.
func (c *Client) RenameAccount(ctx context.Context, account uint32, name string) error {
	var n *string
	if name != "" {
		n = &name
	}
	params := struct {
		Account uint32  `json:"account"`
		Name    *string `json:"name"`
	}{Account: account, Name: n}
	return c.call(ctx, "account_rename", params, nil)
}

// GetBalance returns the confirmed coin and token balances for an account.
func (c *Client) GetBalance(ctx context.Context, account uint32) (*Balance, error) {
	var result Balance
	params := struct {
		Account    uint32   `json:"account"`
		UTXOStates []string `json:"utxo_states"`
		WithLocked *string  `json:"with_locked"`
	}{
		Account:    account,
		UTXOStates: []string{"Confirmed"},
	}
	if err := c.call(ctx, "account_balance", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// NewAddress generates a new unused receive address for the account.
func (c *Client) NewAddress(ctx context.Context, account uint32) (string, error) {
	var result struct {
		Address string `json:"address"`
	}
	params := struct {
		Account uint32 `json:"account"`
	}{Account: account}
	if err := c.call(ctx, "address_new", params, &result); err != nil {
		return "", err
	}
	return result.Address, nil
}

// ShowReceiveAddresses lists receive addresses for the account with their usage state and balance.
func (c *Client) ShowReceiveAddresses(ctx context.Context, account uint32) ([]AddressWithUsage, error) {
	var result []AddressWithUsage
	params := struct {
		Account               uint32 `json:"account"`
		IncludeChangeAddresses bool   `json:"include_change_addresses"`
	}{Account: account, IncludeChangeAddresses: false}
	if err := c.call(ctx, "address_show", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// RevealPublicKey reveals the public key hex behind a given address.
func (c *Client) RevealPublicKey(ctx context.Context, account uint32, address string) (string, error) {
	var result RevealPublicKeyResult
	params := struct {
		Account uint32 `json:"account"`
		Address string `json:"address"`
	}{Account: account, Address: address}
	if err := c.call(ctx, "address_reveal_public_key", params, &result); err != nil {
		return "", err
	}
	return result.PublicKeyHex, nil
}

// EncryptPrivateKeys encrypts the wallet's private keys with a password.
func (c *Client) EncryptPrivateKeys(ctx context.Context, password string) error {
	params := struct {
		Password string `json:"password"`
	}{Password: password}
	return c.call(ctx, "wallet_encrypt_private_keys", params, nil)
}

// UnlockPrivateKeys unlocks an encrypted wallet for use.
func (c *Client) UnlockPrivateKeys(ctx context.Context, password string) error {
	params := struct {
		Password string `json:"password"`
	}{Password: password}
	return c.call(ctx, "wallet_unlock_private_keys", params, nil)
}

// LockPrivateKeys locks the wallet, preventing private key usage until unlocked again.
func (c *Client) LockPrivateKeys(ctx context.Context) error {
	return c.call(ctx, "wallet_lock_private_keys", struct{}{}, nil)
}
