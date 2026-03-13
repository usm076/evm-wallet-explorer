package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"text/tabwriter"
	"txn-info/model"
	"txn-info/service"
)

const (
	defaultLimit = 50
	maxLimit     = 500
)

type TransactionHandler struct {
	svc *service.TransactionService
}

func New(svc *service.TransactionService) *TransactionHandler {
	return &TransactionHandler{svc: svc}
}

func (h *TransactionHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	address := strings.TrimSpace(r.URL.Query().Get("address"))
	chainIDStr := strings.TrimSpace(r.URL.Query().Get("chainId"))

	if address == "" {
		writeError(w, http.StatusBadRequest, "missing required param: address")
		return
	}
	if chainIDStr == "" {
		writeError(w, http.StatusBadRequest, "missing required param: chainId")
		return
	}
	chainID, err := strconv.Atoi(chainIDStr)
	if err != nil || chainID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid chainId")
		return
	}

	result, err := h.svc.GetBalance(address, chainID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(result)
}

func (h *TransactionHandler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	address := strings.TrimSpace(r.URL.Query().Get("address"))
	chainIDStr := strings.TrimSpace(r.URL.Query().Get("chainId"))
	limitStr := strings.TrimSpace(r.URL.Query().Get("limit"))
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))

	if address == "" {
		writeError(w, http.StatusBadRequest, "missing required param: address")
		return
	}
	if chainIDStr == "" {
		writeError(w, http.StatusBadRequest, "missing required param: chainId")
		return
	}

	chainID, err := strconv.Atoi(chainIDStr)
	if err != nil || chainID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid chainId")
		return
	}

	limit := defaultLimit
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			writeError(w, http.StatusBadRequest, "invalid limit: must be a positive integer")
			return
		}
		if limit > maxLimit {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("limit exceeds maximum of %d", maxLimit))
			return
		}
	}

	result, err := h.svc.GetTransactions(address, chainID, limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if format == "table" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(renderTable(result)))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(result)
}

func renderTable(r *model.TransactionsResponse) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Address : %s\n", r.Address)
	fmt.Fprintf(&buf, "Chain   : %s (chainId=%d)\n", r.ChainName, r.ChainID)
	fmt.Fprintf(&buf, "Total   : %d txns (%d incoming, %d outgoing)\n\n", r.Total, len(r.Incoming), len(r.Outgoing))

	writeTxTable(&buf, "INCOMING", r.Incoming)
	writeTxTable(&buf, "OUTGOING", r.Outgoing)
	return buf.String()
}

func writeTxTable(buf *bytes.Buffer, title string, txs []model.Transaction) {
	fmt.Fprintf(buf, "── %s (%d) ──\n", title, len(txs))
	if len(txs) == 0 {
		fmt.Fprintln(buf, "  (none)")
		fmt.Fprintln(buf)
		return
	}

	tw := tabwriter.NewWriter(buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "HASH\tTIMESTAMP\tFROM\tTO\tVALUE\tGAS FEE\tSTATUS\tERC20 TRANSFERS")
	fmt.Fprintln(tw, "────\t─────────\t────\t──\t─────\t───────\t──────\t───────────────")

	for _, tx := range txs {
		erc20 := formatERC20Summary(tx.ERC20Transfers)
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			shortHash(tx.Hash),
			tx.Timestamp,
			shortAddr(tx.From),
			shortAddr(tx.To),
			tx.Value,
			tx.GasFee,
			tx.Status,
			erc20,
		)
	}
	tw.Flush()
	fmt.Fprintln(buf)
}

func formatERC20Summary(transfers []model.ERC20Transfer) string {
	if len(transfers) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(transfers))
	for _, t := range transfers {
		parts = append(parts, fmt.Sprintf("%s %s (%s)", t.Value, t.TokenSymbol, shortAddr(t.ContractAddress)))
	}
	return strings.Join(parts, " | ")
}

func shortHash(h string) string {
	if len(h) > 16 {
		return h[:8] + "..." + h[len(h)-6:]
	}
	return h
}

func shortAddr(a string) string {
	if len(a) > 12 {
		return a[:6] + "..." + a[len(a)-4:]
	}
	return a
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorResponse{Error: msg})
}
