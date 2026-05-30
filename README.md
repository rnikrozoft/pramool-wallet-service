# Pramool Wallet Service

Microservice for topup creation, Omise webhook handling, and transaction history.

Database migrations live in **`pramool-core/migrations/wallet/`** — run from pramool-core: `go run . migrate --db wallet` (or `--db all`).

## Endpoints
- `POST /wallet/topup`
- `GET /wallet/transactions`
- `POST /webhooks/omise`
