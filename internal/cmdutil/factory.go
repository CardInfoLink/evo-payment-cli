package cmdutil

import (
	"net/http"
	"sync"
	"time"

	"github.com/evopayment/evo-cli/internal/core"
	"github.com/evopayment/evo-cli/internal/keychain"
	"github.com/evopayment/evo-cli/internal/registry"
)

// DefaultTimeout is the default HTTP client timeout (45s for POST-heavy workloads).
const DefaultTimeout = 45 * time.Second

// Factory provides lazy-loaded shared dependencies for CLI commands.
type Factory interface {
	Config() (*core.CliConfig, error)
	HttpClient() (*http.Client, error)
	EvoClient() (*EvoClient, error)
	IOStreams() *IOStreams
	Registry() (*registry.Registry, error)
}

// DefaultFactory is the standard Factory implementation with sync.Once lazy loading.
type DefaultFactory struct {
	io *IOStreams

	configOnce sync.Once
	config     *core.CliConfig
	configErr  error

	httpClientOnce sync.Once
	httpClient     *http.Client
	httpClientErr  error

	evoClientOnce sync.Once
	evoClient     *EvoClient
	evoClientErr  error

	registryOnce sync.Once
	reg          *registry.Registry
	registryErr  error

	keychainOnce sync.Once
	kc           keychain.KeychainAccess
}

// NewFactory creates a new DefaultFactory with the given IOStreams.
func NewFactory(io *IOStreams) *DefaultFactory {
	return &DefaultFactory{io: io}
}

// Config returns the lazily-loaded CLI configuration.
func (f *DefaultFactory) Config() (*core.CliConfig, error) {
	f.configOnce.Do(func() {
		f.config, f.configErr = core.LoadConfig("")
	})
	return f.config, f.configErr
}

// Keychain returns the lazily-loaded platform keychain accessor.
func (f *DefaultFactory) Keychain() keychain.KeychainAccess {
	f.keychainOnce.Do(func() {
		f.kc = keychain.New()
	})
	return f.kc
}

// HttpClient returns the lazily-loaded HTTP client with the full Transport chain:
// SignatureTransport → RetryTransport → UserAgentTransport → http.DefaultTransport
func (f *DefaultFactory) HttpClient() (*http.Client, error) {
	f.httpClientOnce.Do(func() {
		// 1. UserAgentTransport wrapping http.DefaultTransport
		uaTransport := &UserAgentTransport{
			Base: http.DefaultTransport,
		}

		// 2. RetryTransport wrapping UserAgentTransport (MaxRetries=3)
		retryTransport := &RetryTransport{
			Base:       uaTransport,
			MaxRetries: 3,
		}

		// 3. SignatureTransport wrapping RetryTransport
		sigTransport := &SignatureTransport{
			Base: retryTransport,
			ConfigFunc: func() (*core.CliConfig, error) {
				return f.Config()
			},
			KeychainResolver: f.keychainResolver(),
		}

		// 4. Create http.Client with SignatureTransport and 45s timeout
		f.httpClient = &http.Client{
			Transport: sigTransport,
			Timeout:   DefaultTimeout,
		}
	})
	return f.httpClient, f.httpClientErr
}

// keychainResolver returns a KeychainResolver that lazily loads the keychain.
func (f *DefaultFactory) keychainResolver() core.KeychainResolver {
	return f.Keychain()
}

// EvoClient returns the lazily-loaded Evo Payment API client.
func (f *DefaultFactory) EvoClient() (*EvoClient, error) {
	f.evoClientOnce.Do(func() {
		cfg, err := f.Config()
		if err != nil {
			f.evoClientErr = err
			return
		}
		httpClient, err := f.HttpClient()
		if err != nil {
			f.evoClientErr = err
			return
		}
		f.evoClient = NewEvoClient(httpClient, cfg, f.IOStreams())
	})
	return f.evoClient, f.evoClientErr
}

// IOStreams returns the IOStreams instance.
func (f *DefaultFactory) IOStreams() *IOStreams {
	return f.io
}

// Registry returns the lazily-loaded API metadata registry.
func (f *DefaultFactory) Registry() (*registry.Registry, error) {
	f.registryOnce.Do(func() {
		f.reg, f.registryErr = registry.LoadRegistry()
	})
	return f.reg, f.registryErr
}
