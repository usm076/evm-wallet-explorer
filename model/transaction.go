package model

type ERC20Transfer struct {
	ContractAddress string `json:"contractAddress"`
	TokenName       string `json:"tokenName"`
	TokenSymbol     string `json:"tokenSymbol"`
	Value           string `json:"value"`
	Decimals        int    `json:"decimals"`
}

type Transaction struct {
	Hash             string          `json:"hash"`
	BlockNumber      string          `json:"blockNumber"`
	Timestamp        string          `json:"timestamp"`
	From             string          `json:"from"`
	To               string          `json:"to"`
	Value            string          `json:"value"`
	ValueRaw         string          `json:"valueRaw"`
	GasUsed          string          `json:"gasUsed"`
	GasFee           string          `json:"gasFee"`
	GasFeeRaw        string          `json:"gasFeeRaw"`
	Status           string          `json:"status"`
	ErrorDescription string          `json:"errorDescription,omitempty"`
	ERC20Transfers   []ERC20Transfer `json:"erc20Transfers,omitempty"`
}

type BalanceResponse struct {
	Address      string `json:"address"`
	ChainID      int    `json:"chainId"`
	ChainName    string `json:"chainName"`
	Balance      string `json:"balance"`
	BalanceRaw   string `json:"balanceRaw"`
	NativeSymbol string `json:"nativeSymbol"`
}

type TransactionsResponse struct {
	Address   string        `json:"address"`
	ChainID   int           `json:"chainId"`
	ChainName string        `json:"chainName"`
	Total     int           `json:"total"`
	Incoming  []Transaction `json:"incoming"`
	Outgoing  []Transaction `json:"outgoing"`
}
