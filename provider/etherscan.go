package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// rawTx maps the Etherscan txlist response fields.
type RawTx struct {
	BlockNumber      string `json:"blockNumber"`
	TimeStamp        string `json:"timeStamp"`
	Hash             string `json:"hash"`
	From             string `json:"from"`
	To               string `json:"to"`
	Value            string `json:"value"`
	Gas              string `json:"gas"`
	GasPrice         string `json:"gasPrice"`
	GasUsed          string `json:"gasUsed"`
	IsError          string `json:"isError"`
	TxReceiptStatus  string `json:"txreceiptStatus"`
}

// RawTokenTx maps the Etherscan tokentx response fields.
type RawTokenTx struct {
	Hash            string `json:"hash"`
	From            string `json:"from"`
	To              string `json:"to"`
	ContractAddress string `json:"contractAddress"`
	TokenName       string `json:"tokenName"`
	TokenSymbol     string `json:"tokenSymbol"`
	TokenDecimal    string `json:"tokenDecimal"`
	Value           string `json:"value"`
}

type etherscanResponse[T any] struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Result  T      `json:"result"`
}

type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	chainID    int // used for Etherscan V2 multi-chain requests
}

func NewClient(baseURL, apiKey string, chainID int) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		baseURL:    baseURL,
		apiKey:     apiKey,
		chainID:    chainID,
	}
}

func (c *Client) FetchTransactions(address string, limit int) ([]RawTx, error) {
	params := c.baseParams()
	params.Set("module", "account")
	params.Set("action", "txlist")
	params.Set("address", address)
	params.Set("sort", "desc")
	params.Set("page", "1")
	params.Set("offset", fmt.Sprintf("%d", limit))

	slog.Info("fetching transactions", "address", address, "limit", limit, "url", c.maskedURL(params))

	var resp etherscanResponse[json.RawMessage]
	if err := c.get(params, &resp); err != nil {
		slog.Error("txlist request failed", "address", address, "error", err)
		return nil, err
	}

	slog.Debug("txlist response", "status", resp.Status, "message", resp.Message)

	if resp.Status != "1" {
		if resp.Message == "No transactions found" {
			slog.Info("no transactions found", "address", address)
			return nil, nil
		}
		slog.Error("explorer API error", "action", "txlist", "status", resp.Status, "message", resp.Message)
		return nil, fmt.Errorf("explorer API error: %s", resp.Message)
	}

	var txs []RawTx
	if err := json.Unmarshal(resp.Result, &txs); err != nil {
		return nil, fmt.Errorf("failed to parse transactions: %w", err)
	}
	slog.Info("fetched transactions", "address", address, "count", len(txs))
	return txs, nil
}

func (c *Client) FetchTokenTransfers(address string, limit int) ([]RawTokenTx, error) {
	params := c.baseParams()
	params.Set("module", "account")
	params.Set("action", "tokentx")
	params.Set("address", address)
	params.Set("sort", "desc")
	params.Set("page", "1")
	params.Set("offset", fmt.Sprintf("%d", limit))

	slog.Info("fetching token transfers", "address", address, "limit", limit, "url", c.maskedURL(params))

	var resp etherscanResponse[json.RawMessage]
	if err := c.get(params, &resp); err != nil {
		slog.Error("tokentx request failed", "address", address, "error", err)
		return nil, err
	}

	slog.Debug("tokentx response", "status", resp.Status, "message", resp.Message)

	if resp.Status != "1" {
		if resp.Message == "No transactions found" {
			slog.Info("no token transfers found", "address", address)
			return nil, nil
		}
		slog.Error("explorer API error", "action", "tokentx", "status", resp.Status, "message", resp.Message)
		return nil, fmt.Errorf("explorer API error: %s", resp.Message)
	}

	var tokenTxs []RawTokenTx
	if err := json.Unmarshal(resp.Result, &tokenTxs); err != nil {
		return nil, fmt.Errorf("failed to parse token transfers: %w", err)
	}
	slog.Info("fetched token transfers", "address", address, "count", len(tokenTxs))
	return tokenTxs, nil
}

// TxStatus holds the result of the getstatus endpoint.
type TxStatus struct {
	IsError        string `json:"isError"`
	ErrDescription string `json:"errDescription"`
}

func (c *Client) FetchBalance(address string) (string, error) {
	params := c.baseParams()
	params.Set("module", "account")
	params.Set("action", "balance")
	params.Set("address", address)
	params.Set("tag", "latest")

	slog.Info("fetching balance", "address", address, "url", c.maskedURL(params))

	var resp etherscanResponse[string]
	if err := c.get(params, &resp); err != nil {
		slog.Error("balance request failed", "address", address, "error", err)
		return "", err
	}
	if resp.Status != "1" {
		slog.Error("explorer API error", "action", "balance", "status", resp.Status, "message", resp.Message)
		return "", fmt.Errorf("explorer API error: %s", resp.Message)
	}
	slog.Info("fetched balance", "address", address, "raw", resp.Result)
	return resp.Result, nil
}

func (c *Client) FetchTxStatus(txHash string) (*TxStatus, error) {
	params := c.baseParams()
	params.Set("module", "transaction")
	params.Set("action", "getstatus")
	params.Set("txhash", txHash)

	var resp etherscanResponse[TxStatus]
	if err := c.get(params, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "1" {
		return nil, fmt.Errorf("explorer API error: %s", resp.Message)
	}
	return &resp.Result, nil
}

func (c *Client) baseParams() url.Values {
	params := url.Values{}
	params.Set("apikey", c.apiKey)
	// Etherscan V2 requires chainid param; BscScan ignores it.
	if c.chainID != 0 {
		params.Set("chainid", fmt.Sprintf("%d", c.chainID))
	}
	return params
}

func (c *Client) get(params url.Values, out any) error {
	reqURL := fmt.Sprintf("%s?%s", c.baseURL, params.Encode())
	start := time.Now()
	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	slog.Debug("explorer HTTP response", "status", resp.StatusCode, "latency_ms", time.Since(start).Milliseconds())

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		slog.Error("explorer non-200", "http_status", resp.StatusCode, "body", string(body))
		return fmt.Errorf("explorer returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	return nil
}

// maskedURL returns the request URL with the API key replaced by "***".
func (c *Client) maskedURL(params url.Values) string {
	masked := make(url.Values, len(params))
	for k, v := range params {
		if strings.EqualFold(k, "apikey") {
			masked[k] = []string{"***"}
		} else {
			masked[k] = v
		}
	}
	return fmt.Sprintf("%s?%s", c.baseURL, masked.Encode())
}
