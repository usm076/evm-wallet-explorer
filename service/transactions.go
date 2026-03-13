package service

import (
	"fmt"
	"log/slog"
	"math/big"
	"strconv"
	"strings"
	"time"
	"txn-info/config"
	"txn-info/model"
	"txn-info/provider"
)

type TransactionService struct {
	cfg *config.Config
}

func New(cfg *config.Config) *TransactionService {
	return &TransactionService{cfg: cfg}
}

func (s *TransactionService) GetTransactions(address string, chainID, limit int) (*model.TransactionsResponse, error) {
	slog.Info("get transactions", "address", address, "chainId", chainID, "limit", limit)

	chain, err := s.cfg.GetChain(chainID)
	if err != nil {
		slog.Error("unsupported chain", "chainId", chainID, "error", err)
		return nil, err
	}

	client := provider.NewClient(chain.APIBase, chain.APIKey(), chainID)
	normalizedAddress := strings.ToLower(address)

	// Fetch both normal txs and ERC20 transfers concurrently.
	type txResult struct {
		txs []provider.RawTx
		err error
	}
	type tokenResult struct {
		txs []provider.RawTokenTx
		err error
	}

	txCh := make(chan txResult, 1)
	tokenCh := make(chan tokenResult, 1)

	go func() {
		txs, err := client.FetchTransactions(address, limit)
		txCh <- txResult{txs, err}
	}()
	go func() {
		txs, err := client.FetchTokenTransfers(address, limit)
		tokenCh <- tokenResult{txs, err}
	}()

	txRes := <-txCh
	tokenRes := <-tokenCh

	if txRes.err != nil {
		slog.Error("failed to fetch transactions", "address", address, "chainId", chainID, "error", txRes.err)
		return nil, fmt.Errorf("fetching transactions: %w", txRes.err)
	}
	if tokenRes.err != nil {
		slog.Error("failed to fetch token transfers", "address", address, "chainId", chainID, "error", tokenRes.err)
		return nil, fmt.Errorf("fetching token transfers: %w", tokenRes.err)
	}

	// Build a map of hash → []ERC20Transfer for quick lookup.
	erc20ByHash := buildERC20Map(tokenRes.txs, normalizedAddress)

	var incoming, outgoing []model.Transaction
	for _, raw := range txRes.txs {
		tx := convertTx(raw, chain, erc20ByHash)
		if strings.ToLower(raw.To) == normalizedAddress {
			incoming = append(incoming, tx)
		} else {
			outgoing = append(outgoing, tx)
		}
	}

	// Some ERC20 transfers may not have a corresponding normal tx entry
	// (e.g. internal calls). Attach orphaned token transfers as synthetic txs.
	knownHashes := make(map[string]bool, len(txRes.txs))
	for _, raw := range txRes.txs {
		knownHashes[strings.ToLower(raw.Hash)] = true
	}
	for hash, transfers := range erc20ByHash {
		if knownHashes[hash] {
			continue
		}
		// Determine direction from the first transfer.
		if len(transfers) == 0 {
			continue
		}
		tx := model.Transaction{
			Hash:           hash,
			Status:         "success",
			ERC20Transfers: transfers,
		}
		first := tokenRes.txs[0]
		for _, raw := range tokenRes.txs {
			if strings.ToLower(raw.Hash) == hash {
				first = raw
				break
			}
		}
		tx.From = strings.ToLower(first.From)
		tx.To = strings.ToLower(first.To)
		if strings.ToLower(first.To) == normalizedAddress {
			incoming = append(incoming, tx)
		} else {
			outgoing = append(outgoing, tx)
		}
	}

	// Enrich failed transactions with error descriptions from getstatus.
	enrichFailedTxs(client, incoming)
	enrichFailedTxs(client, outgoing)

	total := len(incoming) + len(outgoing)
	slog.Info("transactions categorized", "address", address, "chainId", chainID, "incoming", len(incoming), "outgoing", len(outgoing), "total", total)
	return &model.TransactionsResponse{
		Address:   address,
		ChainID:   chainID,
		ChainName: chain.Name,
		Total:     total,
		Incoming:  nilToEmpty(incoming),
		Outgoing:  nilToEmpty(outgoing),
	}, nil
}

func (s *TransactionService) GetBalance(address string, chainID int) (*model.BalanceResponse, error) {
	slog.Info("get balance", "address", address, "chainId", chainID)

	chain, err := s.cfg.GetChain(chainID)
	if err != nil {
		slog.Error("unsupported chain", "chainId", chainID, "error", err)
		return nil, err
	}

	client := provider.NewClient(chain.APIBase, chain.APIKey(), chainID)
	raw, err := client.FetchBalance(address)
	if err != nil {
		return nil, fmt.Errorf("fetching balance: %w", err)
	}

	return &model.BalanceResponse{
		Address:      address,
		ChainID:      chainID,
		ChainName:    chain.Name,
		Balance:      formatNative(raw, chain.NativeSymbol, chain.NativeDecimals),
		BalanceRaw:   raw,
		NativeSymbol: chain.NativeSymbol,
	}, nil
}

// enrichFailedTxs fetches getstatus for each failed tx and attaches the error description.
// Calls are made concurrently, one goroutine per failed tx.
func enrichFailedTxs(client *provider.Client, txs []model.Transaction) {
	type result struct {
		idx  int
		desc string
	}
	ch := make(chan result, len(txs))
	count := 0

	for i, tx := range txs {
		if tx.Status != "failed" {
			continue
		}
		count++
		go func(idx int, hash string) {
			status, err := client.FetchTxStatus(hash)
			if err != nil {
				slog.Warn("could not fetch tx status", "hash", hash, "error", err)
				ch <- result{idx, ""}
				return
			}
			ch <- result{idx, status.ErrDescription}
		}(i, tx.Hash)
	}

	for range count {
		r := <-ch
		if r.desc != "" {
			txs[r.idx].ErrorDescription = r.desc
		}
	}
}

func buildERC20Map(raws []provider.RawTokenTx, address string) map[string][]model.ERC20Transfer {
	m := make(map[string][]model.ERC20Transfer)
	for _, raw := range raws {
		hash := strings.ToLower(raw.Hash)
		decimals, _ := strconv.Atoi(raw.TokenDecimal)
		transfer := model.ERC20Transfer{
			ContractAddress: raw.ContractAddress,
			TokenName:       raw.TokenName,
			TokenSymbol:     raw.TokenSymbol,
			Value:           formatTokenValue(raw.Value, decimals),
			Decimals:        decimals,
		}
		m[hash] = append(m[hash], transfer)
	}
	return m
}

func convertTx(raw provider.RawTx, chain config.ChainConfig, erc20ByHash map[string][]model.ERC20Transfer) model.Transaction {
	hash := strings.ToLower(raw.Hash)

	status := "success"
	if raw.IsError == "1" || raw.TxReceiptStatus == "0" {
		status = "failed"
	}

	// Gas fee = gasUsed * gasPrice
	gasUsed, _ := new(big.Int).SetString(raw.GasUsed, 10)
	gasPrice, _ := new(big.Int).SetString(raw.GasPrice, 10)
	gasFeeWei := new(big.Int).Mul(gasUsed, gasPrice)

	ts := parseTimestamp(raw.TimeStamp)

	return model.Transaction{
		Hash:           hash,
		BlockNumber:    raw.BlockNumber,
		Timestamp:      ts,
		From:           strings.ToLower(raw.From),
		To:             strings.ToLower(raw.To),
		Value:          formatNative(raw.Value, chain.NativeSymbol, chain.NativeDecimals),
		ValueRaw:       raw.Value,
		GasUsed:        raw.GasUsed,
		GasFee:         formatNative(gasFeeWei.String(), chain.NativeSymbol, chain.NativeDecimals),
		GasFeeRaw:      gasFeeWei.String(),
		Status:         status,
		ERC20Transfers: erc20ByHash[hash],
	}
}

// formatNative converts a wei string to a human-readable value with symbol.
func formatNative(weiStr, symbol string, decimals int) string {
	if weiStr == "" || weiStr == "0" {
		return "0 " + symbol
	}
	wei, ok := new(big.Int).SetString(weiStr, 10)
	if !ok {
		return weiStr + " " + symbol
	}
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)

	// Use big.Float for precision.
	fWei := new(big.Float).SetInt(wei)
	fDiv := new(big.Float).SetInt(divisor)
	result := new(big.Float).Quo(fWei, fDiv)

	// Format: trim trailing zeros, keep up to 8 decimal places.
	formatted := fmt.Sprintf("%.8f", result)
	formatted = strings.TrimRight(formatted, "0")
	formatted = strings.TrimRight(formatted, ".")
	return formatted + " " + symbol
}

// formatTokenValue converts raw token amount to human-readable using token decimals.
func formatTokenValue(rawValue string, decimals int) string {
	if rawValue == "" || rawValue == "0" {
		return "0"
	}
	val, ok := new(big.Int).SetString(rawValue, 10)
	if !ok {
		return rawValue
	}
	if decimals == 0 {
		return val.String()
	}
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	fVal := new(big.Float).SetInt(val)
	fDiv := new(big.Float).SetInt(divisor)
	result := new(big.Float).Quo(fVal, fDiv)

	formatted := fmt.Sprintf("%.8f", result)
	formatted = strings.TrimRight(formatted, "0")
	formatted = strings.TrimRight(formatted, ".")
	return formatted
}

func parseTimestamp(ts string) string {
	unix, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return ts
	}
	return time.Unix(unix, 0).UTC().Format(time.RFC3339)
}

func nilToEmpty(txs []model.Transaction) []model.Transaction {
	if txs == nil {
		return []model.Transaction{}
	}
	return txs
}
