package repository

import (
	"context"
	"database/sql"
	"time"
)

type PlatformFeeSummary struct {
	CompletedPayoutAuctions int64
	TotalPlatformShareBaht  int64
}

type PlatformWithdrawRow struct {
	WithdrawalID    int64
	AdminID         int
	AmountBaht      int64
	OmiseTransferID sql.NullString
	OmiseStatus     sql.NullString
	Status          string
	ErrorMessage    sql.NullString
	CreatedAt       time.Time
}

func (r *WalletRepository) SummarizePlatformFees(ctx context.Context) (*PlatformFeeSummary, error) {
	var completed, total int64
	err := r.db.QueryRowContext(ctx, `
SELECT
  COUNT(*)::bigint,
  COALESCE(SUM(platform_fee_amount), 0)::bigint
FROM platform_sale_fees
`).Scan(&completed, &total)
	if err != nil {
		return nil, err
	}
	return &PlatformFeeSummary{
		CompletedPayoutAuctions: completed,
		TotalPlatformShareBaht:  total,
	}, nil
}

func (r *WalletRepository) SumPlatformWithdrawn(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.QueryRowContext(ctx, `
SELECT COALESCE(SUM(amount_baht), 0)::bigint
FROM platform_withdrawals
WHERE status IN ('completed', 'pending')
`).Scan(&n)
	return n, err
}

func (r *WalletRepository) InsertPlatformWithdrawal(ctx context.Context, adminID int, amount int64) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
INSERT INTO platform_withdrawals (admin_id, amount_baht, status)
VALUES ($1, $2, 'pending')
RETURNING withdrawal_id
`, adminID, amount).Scan(&id)
	return id, err
}

func (r *WalletRepository) CompletePlatformWithdrawal(ctx context.Context, withdrawalID int64, status, transferID, omiseStatus, errMsg string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE platform_withdrawals
SET status = $2,
    omise_transfer_id = NULLIF($3, ''),
    omise_status = NULLIF($4, ''),
    error_message = NULLIF($5, ''),
    updated_at = NOW()
WHERE withdrawal_id = $1
`, withdrawalID, status, transferID, omiseStatus, errMsg)
	return err
}

func (r *WalletRepository) ListPlatformWithdrawals(ctx context.Context, limit int) ([]PlatformWithdrawRow, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT withdrawal_id, admin_id, amount_baht, omise_transfer_id, omise_status, status, error_message, created_at
FROM platform_withdrawals
ORDER BY created_at DESC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]PlatformWithdrawRow, 0, limit)
	for rows.Next() {
		var row PlatformWithdrawRow
		if err := rows.Scan(
			&row.WithdrawalID, &row.AdminID, &row.AmountBaht,
			&row.OmiseTransferID, &row.OmiseStatus, &row.Status, &row.ErrorMessage, &row.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
