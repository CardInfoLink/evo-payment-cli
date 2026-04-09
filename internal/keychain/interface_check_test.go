package keychain_test

import (
	"github.com/evopayment/evo-cli/internal/core"
	"github.com/evopayment/evo-cli/internal/keychain"
)

// Compile-time check: KeychainAccess satisfies core.KeychainResolver.
var _ core.KeychainResolver = (keychain.KeychainAccess)(nil)
