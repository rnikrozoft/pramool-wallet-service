package repository

import (
	"database/sql"

	"github.com/rnikrozoft/pramool-wallet-service/model/entity"
)

type WalletRepository struct {
	db *sql.DB
}

func NewWalletRepository(db *sql.DB) *WalletRepository {
	return &WalletRepository{db: db}
}

func (r *WalletRepository) InsertTransaction(chargeID, userID string, amount int64, status string, paid, credited bool) error {
	_, err := r.db.Exec(`
		INSERT INTO transactions (charge_id, user_id, amount, status, paid, credited)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (charge_id) DO NOTHING
	`, chargeID, userID, amount, status, paid, credited)
	return err
}

func (r *WalletRepository) ListTransactionsByUser(userID string, limit int) ([]entity.Transaction, error) {
	rows, err := r.db.Query(`
		SELECT charge_id, amount, status, paid, credited, created_at, updated_at
		FROM transactions WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]entity.Transaction, 0)
	for rows.Next() {
		var item entity.Transaction
		if err := rows.Scan(&item.ChargeID, &item.Amount, &item.Status, &item.Paid, &item.Credited, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// CountCreditActivity returns how many rows match the filter (all | topup | auction).
func (r *WalletRepository) CountCreditActivity(userID string, filter string) (int, error) {
	var q string
	args := []interface{}{userID}
	switch filter {
	case "topup":
		q = `SELECT COUNT(*)::int FROM transactions WHERE user_id = $1`
	case "auction":
		q = `SELECT COUNT(*)::int FROM bid_transactions WHERE user_id = $1`
	default:
		q = `
SELECT COUNT(*)::int FROM (
  SELECT 1 FROM transactions WHERE user_id = $1
  UNION ALL
  SELECT 1 FROM bid_transactions WHERE user_id = $1
) x`
	}
	var n int
	err := r.db.QueryRow(q, args...).Scan(&n)
	return n, err
}

// ListCreditActivity returns merged credit activity ordered by created_at DESC.
func (r *WalletRepository) ListCreditActivity(userID string, limit, offset int, filter string) ([]entity.CreditActivityRow, error) {
	var q string
	args := []interface{}{userID}
	switch filter {
	case "topup":
		q = `
SELECT 'topup'::text AS entry_type, t.created_at, t.updated_at,
  t.charge_id, t.amount, t.status, t.paid, t.credited,
  NULL::bigint, NULL::text, NULL::text, NULL::bigint, NULL::bigint, NULL::text
FROM transactions t WHERE t.user_id = $1
ORDER BY t.created_at DESC
LIMIT $2 OFFSET $3`
		args = append(args, limit, offset)
	case "auction":
		q = `
SELECT bt.tx_type::text AS entry_type, bt.created_at, NULL::timestamptz AS updated_at,
  NULL::text, NULL::bigint, NULL::text, NULL::bool, NULL::bool,
  bt.bid_tx_id, bt.auction_id, COALESCE(a.title, ''),
  bt.amount, bt.bid_amount, bt.note
FROM bid_transactions bt
LEFT JOIN auctions a ON a.auction_id = bt.auction_id
WHERE bt.user_id = $1
ORDER BY bt.created_at DESC
LIMIT $2 OFFSET $3`
		args = append(args, limit, offset)
	default:
		q = `
SELECT * FROM (
  SELECT 'topup'::text AS entry_type, t.created_at, t.updated_at,
    t.charge_id, t.amount, t.status, t.paid, t.credited,
    NULL::bigint, NULL::text, NULL::text, NULL::bigint, NULL::bigint, NULL::text
  FROM transactions t WHERE t.user_id = $1
  UNION ALL
  SELECT bt.tx_type::text, bt.created_at, NULL::timestamptz,
    NULL::text, NULL::bigint, NULL::text, NULL::bool, NULL::bool,
    bt.bid_tx_id, bt.auction_id, COALESCE(a.title, ''),
    bt.amount, bt.bid_amount, bt.note
  FROM bid_transactions bt
  LEFT JOIN auctions a ON a.auction_id = bt.auction_id
  WHERE bt.user_id = $1
) sub
ORDER BY created_at DESC
LIMIT $2 OFFSET $3`
		args = append(args, limit, offset)
	}

	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]entity.CreditActivityRow, 0)
	for rows.Next() {
		var row entity.CreditActivityRow
		if err := rows.Scan(
			&row.EntryType, &row.CreatedAt, &row.UpdatedAt,
			&row.ChargeID, &row.TopupAmount, &row.Status, &row.Paid, &row.Credited,
			&row.BidTxID, &row.AuctionID, &row.AuctionTitle,
			&row.LedgerAmount, &row.BidAmount, &row.Note,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *WalletRepository) UpdateTransactionStatus(chargeID, status string, paid bool) error {
	_, err := r.db.Exec(`UPDATE transactions SET status=$1, paid=$2, updated_at=NOW() WHERE charge_id=$3`, status, paid, chargeID)
	return err
}

func (r *WalletRepository) GetTransactionCreditFields(chargeID string) (entity.Transaction, error) {
	var item entity.Transaction
	err := r.db.QueryRow(`SELECT user_id, amount, credited FROM transactions WHERE charge_id=$1`, chargeID).Scan(&item.UserID, &item.Amount, &item.Credited)
	return item, err
}

func (r *WalletRepository) AddUserCredit(userID string, amount int64) error {
	_, err := r.db.Exec(`UPDATE users SET credit = credit + $1, updated_at=NOW() WHERE user_id=$2`, amount, userID)
	return err
}

func (r *WalletRepository) DeductUserCreditIfEnough(userID string, amount int64) (int64, error) {
	res, err := r.db.Exec(`
		UPDATE users
		SET credit = credit - $1, updated_at = NOW()
		WHERE user_id = $2 AND credit >= $1
	`, amount, userID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// UpdateTransactionSetCredited runs UPDATE … WHERE credited = false and returns sql.RowsAffected().
func (r *WalletRepository) UpdateTransactionSetCredited(chargeID string) (int64, error) {
	result, err := r.db.Exec(`
		UPDATE transactions
		SET credited=true, updated_at=NOW()
		WHERE charge_id=$1 AND credited=false
	`, chargeID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
