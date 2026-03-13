package config

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type ChainConfig struct {
	Name           string
	APIBase        string
	NativeSymbol   string
	NativeDecimals int
	// APIKeyEnv is the env var name holding the API key for this chain's explorer.
	// If empty, falls back to EXPLORER_API_KEY.
	APIKeyEnv string
}

// chainRegistry maps chainID → chain configuration.
// To add a new chain, just add an entry here.
var chainRegistry = map[int]ChainConfig{
	56: {
		Name:           "BNB Smart Chain",
		APIBase:        "https://api.etherscan.io/v2/api",
		NativeSymbol:   "BNB",
		NativeDecimals: 18,
		APIKeyEnv:      "ETHERSCAN_API_KEY",
	},
	1: {
		Name:           "Ethereum",
		APIBase:        "https://api.etherscan.io/v2/api",
		NativeSymbol:   "ETH",
		NativeDecimals: 18,
		APIKeyEnv:      "ETHERSCAN_API_KEY",
	},
	137: {
		Name:           "Polygon",
		APIBase:        "https://api.etherscan.io/v2/api",
		NativeSymbol:   "POL",
		NativeDecimals: 18,
		APIKeyEnv:      "ETHERSCAN_API_KEY",
	},
	42161: {
		Name:           "Arbitrum One",
		APIBase:        "https://api.etherscan.io/v2/api",
		NativeSymbol:   "ETH",
		NativeDecimals: 18,
		APIKeyEnv:      "ETHERSCAN_API_KEY",
	},
	10: {
		Name:           "Optimism",
		APIBase:        "https://api.etherscan.io/v2/api",
		NativeSymbol:   "ETH",
		NativeDecimals: 18,
		APIKeyEnv:      "ETHERSCAN_API_KEY",
	},
	8453: {
		Name:           "Base",
		APIBase:        "https://api.etherscan.io/v2/api",
		NativeSymbol:   "ETH",
		NativeDecimals: 18,
		APIKeyEnv:      "ETHERSCAN_API_KEY",
	},
}

type Config struct {
	Port   string
	Chains map[int]ChainConfig
}

// loadDotEnv reads a .env file and sets any unset environment variables from it.
// Already-set env vars take precedence (i.e. real env wins over .env).
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // .env is optional
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// Strip surrounding quotes if present.
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

func Load() (*Config, error) {
	loadDotEnv(".env")
	slog.Debug("loaded .env file")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Validate that at least one chain has an API key configured.
	hasAnyKey := false
	for _, chain := range chainRegistry {
		if getAPIKey(chain) != "" {
			hasAnyKey = true
			break
		}
	}
	if !hasAnyKey {
		return nil, fmt.Errorf("no explorer API key found; set ETHERSCAN_API_KEY or EXPLORER_API_KEY")
	}

	return &Config{
		Port:   port,
		Chains: chainRegistry,
	}, nil
}

func (c *Config) GetChain(chainID int) (ChainConfig, error) {
	chain, ok := c.Chains[chainID]
	if !ok {
		return ChainConfig{}, fmt.Errorf("chainId %d is not supported", chainID)
	}
	if getAPIKey(chain) == "" {
		return ChainConfig{}, fmt.Errorf("no API key configured for chainId %d (%s); set %s or EXPLORER_API_KEY", chainID, chain.Name, chain.APIKeyEnv)
	}
	return chain, nil
}

func (c ChainConfig) APIKey() string {
	return getAPIKey(c)
}

func getAPIKey(chain ChainConfig) string {
	if chain.APIKeyEnv != "" {
		if key := os.Getenv(chain.APIKeyEnv); key != "" {
			return key
		}
	}
	return os.Getenv("EXPLORER_API_KEY")
}
