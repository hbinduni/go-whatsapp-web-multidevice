package utils

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

var (
	// defaultHTTPClient is the singleton HTTP client for general use (30s timeout)
	defaultHTTPClient *http.Client
	clientOnce        sync.Once

	// fastHTTPClient is the singleton HTTP client for quick operations like metadata fetches (15s timeout)
	fastHTTPClient *http.Client
	fastClientOnce sync.Once
)

// HTTPClientConfig holds configuration for the HTTP client
type HTTPClientConfig struct {
	Timeout             time.Duration
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration
	MaxRedirects        int
}

// DefaultHTTPClientConfig returns the default HTTP client configuration
// with a 30-second timeout suitable for media downloads
func DefaultHTTPClientConfig() HTTPClientConfig {
	return HTTPClientConfig{
		Timeout:             30 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		MaxRedirects:        10,
	}
}

// FastHTTPClientConfig returns a configuration for quick operations
// like metadata fetches with a 15-second timeout
func FastHTTPClientConfig() HTTPClientConfig {
	return HTTPClientConfig{
		Timeout:             15 * time.Second,
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     60 * time.Second,
		MaxRedirects:        10,
	}
}

// GetHTTPClient returns the singleton HTTP client instance for media downloads
// This client has a 30-second timeout and is optimized for connection reuse
func GetHTTPClient() *http.Client {
	clientOnce.Do(func() {
		config := DefaultHTTPClientConfig()
		defaultHTTPClient = createHTTPClient(config)
	})
	return defaultHTTPClient
}

// GetFastHTTPClient returns the singleton HTTP client for quick operations
// This client has a 15-second timeout, suitable for metadata fetches
func GetFastHTTPClient() *http.Client {
	fastClientOnce.Do(func() {
		config := FastHTTPClientConfig()
		fastHTTPClient = createHTTPClient(config)
	})
	return fastHTTPClient
}

// createHTTPClient creates an HTTP client with the given configuration
func createHTTPClient(config HTTPClientConfig) *http.Client {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        config.MaxIdleConns,
		MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
		IdleConnTimeout:     config.IdleConnTimeout,
		TLSHandshakeTimeout: 10 * time.Second,
		// Disable compression to avoid issues with some servers
		DisableCompression: false,
	}

	return &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= config.MaxRedirects {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
}

// NewHTTPClientWithTimeout creates a new HTTP client with a custom timeout
// Use this when you need a different timeout than the default
// Note: This creates a new client, so use sparingly
func NewHTTPClientWithTimeout(timeout time.Duration) *http.Client {
	config := DefaultHTTPClientConfig()
	config.Timeout = timeout
	return createHTTPClient(config)
}
