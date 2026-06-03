package repository

import (
	"database/sql"
	"strings"

	"github.com/rnikrozoft/pramool-wallet-service/model/entity"
)

type WalletRepository struct {
	db *sql.DB
}

func NewWalletRepository(db *sql.DB) *WalletRepository {
	return &WalletRepository{db: db}
}

func (r *WalletRepository) InsertTransaction(chargeID, userID string, gross, fee, credit int64, status string, paid, credited bool, qrCodeURL string) error {
	_, err := r.db.Exec(`
		INSERT INTO transactions (charge_id, user_id, amount, fee_amount, credit_amount, status, paid, credited, qr_code_url)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (charge_id) DO NOTHING
	`, chargeID, userID, gross, fee, credit, status, paid, credited, nullIfEmptyQR(qrCodeURL))
	return err
}

func nullIfEmptyQR(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.TrimSpace(s)
}

// FindLatestPendingTopup returns the newest unpaid top-up for user+amount (may still be expired on Omise).
func (r *WalletRepository) FindLatestPendingTopup(userID string, gross int64) (*entity.Transaction, error) {
	var item entity.Transaction
	var qr sql.NullString
	err := r.db.QueryRow(`
		SELECT charge_id, amount, COALESCE(fee_amount, 0), COALESCE(NULLIF(credit_amount, 0), amount),
		       status, paid, credited, qr_code_url, created_at, updated_at
		FROM transactions
		WHERE user_id = $1 AND amount = $2 AND paid = false AND credited = false
		  AND LOWER(TRIM(status)) NOT IN ('expired', 'failed', 'reversed', 'cancelled', 'disputed', 'dispute_lost')
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, gross).Scan(
		&item.ChargeID, &item.Amount, &item.FeeAmount, &item.CreditAmount,
		&item.Status, &item.Paid, &item.Credited, &qr, &item.CreatedAt, &item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item.UserID = userID
	if qr.Valid {
		item.QRCodeURL = qr.String
	}
	return &item, nil
}

func (r *WalletRepository) UpdateTransactionQRCode(chargeID, qrCodeURL string) error {
	_, err := r.db.Exec(`
		UPDATE transactions SET qr_code_url = $1, updated_at = NOW() WHERE charge_id = $2
	`, strings.TrimSpace(qrCodeURL), chargeID)
	return err
}

// GetTopupTransaction loads a top-up row owned by userID.
func (r *WalletRepository) GetTopupTransaction(userID, chargeID string) (*entity.Transaction, error) {
	chargeID = strings.TrimSpace(chargeID)
	userID = strings.TrimSpace(userID)
	if chargeID == "" || userID == "" {
		return nil, nil
	}
	var item entity.Transaction
	var qr sql.NullString
	err := r.db.QueryRow(`
		SELECT charge_id, amount, COALESCE(fee_amount, 0), COALESCE(NULLIF(credit_amount, 0), amount),
		       status, paid, credited, COALESCE(credit_reversed, false),
		       COALESCE(dispute_id, ''), COALESCE(dispute_status, ''), qr_code_url, created_at, updated_at
		FROM transactions
		WHERE charge_id = $1 AND user_id = $2
	`, chargeID, userID).Scan(
		&item.ChargeID, &item.Amount, &item.FeeAmount, &item.CreditAmount,
		&item.Status, &item.Paid, &item.Credited, &item.CreditReversed,
		&item.DisputeID, &item.DisputeStatus, &qr, &item.CreatedAt, &item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item.UserID = userID
	if qr.Valid {
		item.QRCodeURL = qr.String
	}
	return &item, nil
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
	case "withdraw":
		q = `SELECT COUNT(*)::int FROM withdrawals WHERE user_id = $1`
	default:
		q = `
SELECT COUNT(*)::int FROM (
  SELECT 1 FROM transactions WHERE user_id = $1
  UNION ALL
  SELECT 1 FROM bid_transactions WHERE user_id = $1
  UNION ALL
  SELECT 1 FROM withdrawals WHERE user_id = $1
) x`
	}
	var n int
	err := r.db.QueryRow(q, args...).Scan(&n)
	return n, err
}

// ListCreditActivity returns merged credit activity with server-side sort and pagination.
func (r *WalletRepository) ListCreditActivity(userID string, limit, offset int, filter, sortKey, sortOrder string) ([]entity.CreditActivityRow, error) {
	orderBy := creditActivityOrderSQL(sortKey, sortOrder, filter)
	var q string
	args := []interface{}{userID}
	switch filter {
	case "topup":
		q = `
SELECT 'topup'::text AS entry_type, t.created_at, t.updated_at,
  t.charge_id, t.credit_amount, t.amount, t.fee_amount, t.status, t.paid, t.credited,
  NULL::bigint, NULL::text, NULL::text, NULL::text, NULL::bigint AS ledger_amount, NULL::bigint, NULL::text
FROM transactions t WHERE t.user_id = $1
ORDER BY ` + orderBy + `
LIMIT $2 OFFSET $3`
		args = append(args, limit, offset)
	case "auction":
		q = `
SELECT bt.tx_type::text AS entry_type, bt.created_at, NULL::timestamptz AS updated_at,
  NULL::text, NULL::bigint, NULL::bigint, NULL::bigint, NULL::text, NULL::bool, NULL::bool,
  bt.bid_tx_id, bt.auction_id, COALESCE(a.title, ''), COALESCE(a.cover_image_url, ''),
  bt.amount, bt.bid_amount, bt.note
FROM bid_transactions bt
LEFT JOIN auctions a ON a.auction_id = bt.auction_id
WHERE bt.user_id = $1
ORDER BY ` + orderBy + `
LIMIT $2 OFFSET $3`
		args = append(args, limit, offset)
	case "withdraw":
		q = `
SELECT 'withdraw'::text AS entry_type, w.created_at, w.updated_at,
  NULL::text, NULL::bigint, NULL::bigint, NULL::bigint, w.status, NULL::bool, NULL::bool,
  w.withdrawal_id, NULL::text, NULL::text, NULL::text,
  (-w.amount) AS ledger_amount, NULL::bigint,
  CONCAT(
    'ถอนเครดิต ', w.amount::text, ' บาท · ค่าธรรมเนียมโอน ', w.fee_amount::text,
    ' บาท · รับเข้าบัญชีประมาณ ', w.transfer_amount::text, ' บาท — ',
    CASE w.status
      WHEN 'pending_review' THEN 'รอการตรวจสอบ'
      WHEN 'processing' THEN 'กำลังดำเนินการ'
      WHEN 'completed' THEN 'เสร็จสิ้น'
      WHEN 'failed' THEN 'ไม่สำเร็จ'
      WHEN 'paid' THEN 'เสร็จสิ้น'
      ELSE COALESCE(w.failure_reason, w.status)
    END
  )
FROM withdrawals w
WHERE w.user_id = $1
ORDER BY ` + orderBy + `
LIMIT $2 OFFSET $3`
		args = append(args, limit, offset)
	default:
		q = `
SELECT * FROM (
  SELECT 'topup'::text AS entry_type, t.created_at, t.updated_at,
    t.charge_id, t.credit_amount, t.amount, t.fee_amount, t.status, t.paid, t.credited,
    NULL::bigint, NULL::text, NULL::text, NULL::text, NULL::bigint AS ledger_amount, NULL::bigint, NULL::text
  FROM transactions t WHERE t.user_id = $1
  UNION ALL
  SELECT bt.tx_type::text, bt.created_at, NULL::timestamptz,
    NULL::text, NULL::bigint, NULL::bigint, NULL::bigint, NULL::text, NULL::bool, NULL::bool,
    bt.bid_tx_id, bt.auction_id, COALESCE(a.title, ''), COALESCE(a.cover_image_url, ''),
    bt.amount AS ledger_amount, bt.bid_amount, bt.note
  FROM bid_transactions bt
  LEFT JOIN auctions a ON a.auction_id = bt.auction_id
  WHERE bt.user_id = $1
  UNION ALL
  SELECT 'withdraw'::text, w.created_at, w.updated_at,
    NULL::text, NULL::bigint, NULL::bigint, NULL::bigint, w.status, NULL::bool, NULL::bool,
    w.withdrawal_id, NULL::text, NULL::text, NULL::text,
    -w.amount, NULL::bigint,
  CONCAT(
    'ถอนเครดิต ', w.amount::text, ' บาท · ค่าธรรมเนียมโอน ', w.fee_amount::text,
    ' บาท · รับเข้าบัญชีประมาณ ', w.transfer_amount::text, ' บาท — ',
    CASE w.status
      WHEN 'pending_review' THEN 'รอการตรวจสอบ'
      WHEN 'processing' THEN 'กำลังดำเนินการ'
      WHEN 'completed' THEN 'เสร็จสิ้น'
      WHEN 'failed' THEN 'ไม่สำเร็จ'
      WHEN 'paid' THEN 'เสร็จสิ้น'
      ELSE COALESCE(w.failure_reason, w.status)
    END
  )
  FROM withdrawals w WHERE w.user_id = $1
) sub
ORDER BY ` + orderBy + `
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
			&row.ChargeID, &row.TopupAmount, &row.TopupPaid, &row.TopupFee,
			&row.Status, &row.Paid, &row.Credited,
			&row.BidTxID, &row.AuctionID, &row.AuctionTitle, &row.AuctionCoverImageURL,
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
	err := r.db.QueryRow(`
		SELECT user_id, amount, COALESCE(fee_amount, 0), COALESCE(NULLIF(credit_amount, 0), amount), credited
		FROM transactions WHERE charge_id=$1
	`, chargeID).Scan(&item.UserID, &item.Amount, &item.FeeAmount, &item.CreditAmount, &item.Credited)
	return item, err
}

// GetTransactionByChargeID loads a top-up row by Omise charge id.
func (r *WalletRepository) GetTransactionByChargeID(chargeID string) (*entity.Transaction, error) {
	chargeID = strings.TrimSpace(chargeID)
	if chargeID == "" {
		return nil, nil
	}
	var item entity.Transaction
	var qr sql.NullString
	err := r.db.QueryRow(`
		SELECT charge_id, user_id, amount, COALESCE(fee_amount, 0), COALESCE(NULLIF(credit_amount, 0), amount),
		       status, paid, credited, COALESCE(credit_reversed, false),
		       COALESCE(dispute_id, ''), COALESCE(dispute_status, ''), qr_code_url, created_at, updated_at
		FROM transactions
		WHERE charge_id = $1
	`, chargeID).Scan(
		&item.ChargeID, &item.UserID, &item.Amount, &item.FeeAmount, &item.CreditAmount,
		&item.Status, &item.Paid, &item.Credited, &item.CreditReversed,
		&item.DisputeID, &item.DisputeStatus, &qr, &item.CreatedAt, &item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if qr.Valid {
		item.QRCodeURL = qr.String
	}
	return &item, nil
}

func (r *WalletRepository) UpdateTransactionDispute(chargeID, disputeID, disputeStatus string) error {
	_, err := r.db.Exec(`
		UPDATE transactions
		SET dispute_id = NULLIF($1, ''), dispute_status = NULLIF($2, ''), updated_at = NOW()
		WHERE charge_id = $3
	`, strings.TrimSpace(disputeID), strings.TrimSpace(disputeStatus), chargeID)
	return err
}

// ClaimTopupCreditReversal marks a credited top-up as reversed and returns credit to claw back (once).
func (r *WalletRepository) ClaimTopupCreditReversal(chargeID string) (userID string, credit int64, claimed bool, err error) {
	err = r.db.QueryRow(`
		UPDATE transactions
		SET credit_reversed = true, credited = false, dispute_status = 'lost', status = 'dispute_lost', updated_at = NOW()
		WHERE charge_id = $1 AND credited = true AND credit_reversed = false
		RETURNING user_id, COALESCE(NULLIF(credit_amount, 0), amount)
	`, chargeID).Scan(&userID, &credit)
	if err == sql.ErrNoRows {
		return "", 0, false, nil
	}
	if err != nil {
		return "", 0, false, err
	}
	return userID, credit, true, nil
}

func (r *WalletRepository) MarkTopupDisputeResolved(chargeID, disputeStatus, txStatus string) error {
	_, err := r.db.Exec(`
		UPDATE transactions
		SET dispute_status = $1, status = $2, updated_at = NOW()
		WHERE charge_id = $3
	`, disputeStatus, txStatus, chargeID)
	return err
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

// DeductUserCredit subtracts credit even when balance would go negative (dispute clawback).
func (r *WalletRepository) DeductUserCredit(userID string, amount int64) error {
	_, err := r.db.Exec(`
		UPDATE users SET credit = credit - $1, updated_at = NOW() WHERE user_id = $2
	`, amount, userID)
	return err
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
