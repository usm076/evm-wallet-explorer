# EVM Wallet Explorer

A lightweight, zero-dependency Go web app that fetches and displays wallet transaction history and balances across major EVM-compatible blockchains using Etherscan-compatible APIs.

## Features

- Query incoming and outgoing transactions for any wallet address
- View ERC-20 token transfers within each transaction
- Fetch native token balances
- Supports 6 EVM chains out of the box
- Web UI and JSON REST API
- No external dependencies — pure Go standard library

## Supported Chains

| Chain | Chain ID | Native Token |
|-------|----------|--------------|
| Ethereum | 1 | ETH |
| BNB Smart Chain | 56 | BNB |
| Polygon | 137 | POL |
| Arbitrum One | 42161 | ETH |
| Optimism | 10 | ETH |
| Base | 8453 | ETH |

## Prerequisites

- Go 1.25+
- An [Etherscan API key](https://etherscan.io/myapikey) (the v2 API supports all chains with a single key)

## Setup

1. Clone the repository:
   ```bash
   git clone git@github.com:usm076/evm-wallet-explorer.git
   cd evm-wallet-explorer
   ```

2. Create a `.env` file in the project root:
   ```env
   ETHERSCAN_API_KEY=your_api_key_here
   PORT=8080
   ```

3. Run the server:
   ```bash
   go run main.go
   ```

4. Open your browser at `http://localhost:8080`

## API Endpoints

### Get Transactions

```
GET /api/transactions?address=<wallet>&chainId=<id>&limit=<n>&format=<fmt>
```

| Parameter | Required | Description |
|-----------|----------|-------------|
| `address` | Yes | Wallet address (0x...) |
| `chainId` | Yes | Chain ID (e.g. 1 for Ethereum) |
| `limit` | No | Max transactions to return (default: 10, max: 100) |
| `format` | No | `json` (default) or `table` |

**Example response (JSON):**
```json
{
  "address": "0xabc...",
  "chainId": 1,
  "chainName": "Ethereum",
  "total": 4,
  "incoming": [...],
  "outgoing": [...]
}
```

Each transaction includes:
- `hash`, `blockNumber`, `timestamp`
- `from`, `to`
- `value` (formatted), `valueRaw` (wei)
- `gasUsed`, `gasFee` (formatted), `gasFeeRaw` (wei)
- `status` (`success` or `failed`)
- `errorDescription` (for failed transactions)
- `erc20Transfers` — list of token transfers within the transaction

### Get Balance

```
GET /api/balance?address=<wallet>&chainId=<id>
```

| Parameter | Required | Description |
|-----------|----------|-------------|
| `address` | Yes | Wallet address (0x...) |
| `chainId` | Yes | Chain ID |

**Example response:**
```json
{
  "address": "0xabc...",
  "chainId": 1,
  "chainName": "Ethereum",
  "balance": "1.23 ETH",
  "balanceRaw": "1230000000000000000",
  "nativeSymbol": "ETH"
}
```

### Web UI

```
GET /
```

Serves the single-page frontend at `static/index.html`.

## Project Structure

```
evm-wallet-explorer/
├── main.go                 # Entry point — wires up routes and starts server
├── config/
│   └── config.go          # Loads env vars, defines chain registry
├── handler/
│   └── transactions.go    # HTTP handlers — parses requests, writes responses
├── service/
│   └── transactions.go    # Business logic — correlates txs and ERC20 transfers
├── provider/
│   └── etherscan.go       # Etherscan API client
├── model/
│   └── transaction.go     # Shared data types
├── static/
│   └── index.html         # Frontend SPA (vanilla JS, no framework)
└── .env                    # Local config (not committed)
```

## How It Works

```
Browser / API Client
       │
       ▼
   HTTP Handler          Parses and validates query params
       │
       ▼
   Service Layer         Fetches txs and ERC20 transfers concurrently,
       │                 correlates them, enriches failed tx details,
       │                 and formats wei values to human-readable form
       ▼
   Provider Layer        Makes HTTP requests to Etherscan v2 API
       │                 (txlist, tokentx, balance, getstatus)
       ▼
  Etherscan API
```

**Key processing steps in the service layer:**

1. Fetch normal transactions (`txlist`) and ERC-20 transfers (`tokentx`) in parallel
2. Build a map of `txHash → []ERC20Transfer`
3. Categorize transactions as incoming or outgoing relative to the queried address
4. Handle "orphaned" ERC-20 transfers (token-only transactions with no corresponding normal tx entry)
5. Fetch error descriptions for failed transactions concurrently
6. Format all raw wei values using `big.Float` arithmetic to avoid floating-point errors

## Configuration

| Env Variable | Required | Description |
|--------------|----------|-------------|
| `ETHERSCAN_API_KEY` | Yes* | Etherscan v2 API key |
| `EXPLORER_API_KEY` | Yes* | Alternative API key name |
| `PORT` | No | Server port (default: `8080`) |

*At least one API key must be set. The app will fail to start otherwise.

## License

MIT
