package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/rnikrozoft/pramool-wallet-service/model/entity"
)

var (
	ErrInsufficientCredit = errors.New("insufficient credit")
	ErrMissingBankAccount = errors.New("missing bank account")
	ErrWithdrawalBlocked  = errors.New("withdrawal blocked")
)

// GetUserPayoutProfile loads credit and bank fields for payout.
func (r *WalletRepository) GetUserPayoutProfile(ctx context.Context, userID string) (*entity.UserPayoutProfile, error) {
	userID = strings.TrimSpace(userID)
	var p entity.UserPayoutProfile
	var bankCode sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT u.user_id, COALESCE(u.credit, 0),
		       COALESCE(u.bank_id, 0),
		       COALESCE(b.bank_code, ''),
		       COALESCE(u.bank_account_name, ''),
		       COALESCE(u.bank_account_number, ''),
		       COALESCE(u.omise_recipient_id, '')
		FROM users u
		LEFT JOIN banks b ON b.bank_id = u.bank_id AND b.is_active = TRUE
		WHERE u.user_id = $1
	`, userID).Scan(
		&p.UserID, &p.Credit, &p.BankID, &bankCode,
		&p.BankAccountName, &p.BankAccountNumber, &p.OmiseRecipientID,
	)
	if err != nil {
		return nil, err
	}
	if bankCode.Valid {
		p.BankCode = bankCode.String
	}
	return &p, nil
}

// CountUserFulfillmentBlocks mirrors pramool-core escrow gating for withdrawals.
func (r *WalletRepository) CountUserFulfillmentBlocks(ctx context.Context, userID string) (pendingSellerShip, pendingBuyerConfirm int, err error) {
	err = r.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*)::int FROM auctions a
			 WHERE a.status = 'closed'
			   AND a.seller_payout_at IS NULL
			   AND COALESCE(NULLIF(TRIM(a.winner_id), ''), '') <> ''
			   AND a.seller_id = $1
			   AND a.seller_shipped_at IS NULL),
			(SELECT COUNT(*)::int FROM auctions a
			 WHERE a.status = 'closed'
			   AND a.seller_payout_at IS NULL
			   AND COALESCE(NULLIF(TRIM(a.winner_id), ''), '') <> ''
			   AND a.winner_id = $1
			   AND a.buyer_received_at IS NULL)
	`, userID).Scan(&pendingSellerShip, &pendingBuyerConfirm)
	return pendingSellerShip, pendingBuyerConfirm, err
}

// ReserveWithdrawal deducts credit and inserts a pending withdrawal row.
func (r *WalletRepository) ReserveWithdrawal(ctx context.Context, userID string, amount, feeAmount, transferAmount int64) (withdrawalID int64, balanceBefore, balanceAfter int64, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(credit, 0) FROM users WHERE user_id = $1 FOR UPDATE`, userID).Scan(&balanceBefore); err != nil {
		return 0, 0, 0, err
	}
	if balanceBefore < amount {
		return 0, 0, 0, ErrInsufficientCredit
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE users SET credit = credit - $1, updated_at = NOW()
		WHERE user_id = $2 AND credit >= $1
	`, amount, userID)
	if err != nil {
		return 0, 0, 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, 0, 0, err
	}
	if n == 0 {
		return 0, 0, 0, ErrInsufficientCredit
	}
	balanceAfter = balanceBefore - amount

	err = tx.QueryRowContext(ctx, `
		INSERT INTO withdrawals (user_id, amount, fee_amount, transfer_amount, status, balance_before, balance_after)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING withdrawal_id
	`, userID, amount, feeAmount, transferAmount, entity.WithdrawStatusPendingReview, balanceBefore, balanceAfter).Scan(&withdrawalID)
	if err != nil {
		return 0, 0, 0, err
	}

	if err = tx.Commit(); err != nil {
		return 0, 0, 0, err
	}
	return withdrawalID, balanceBefore, balanceAfter, nil
}

// FailWithdrawal refunds credit and marks the withdrawal failed.
func (r *WalletRepository) FailWithdrawal(ctx context.Context, withdrawalID int64, reason string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var userID string
	var amount int64
	var status string
	err = tx.QueryRowContext(ctx, `
		SELECT user_id, amount, status FROM withdrawals WHERE withdrawal_id = $1 FOR UPDATE
	`, withdrawalID).Scan(&userID, &amount, &status)
	if err != nil {
		return err
	}
	if status == entity.WithdrawStatusFailed || status == entity.WithdrawStatusCompleted || status == "paid" {
		return tx.Commit()
	}

	if _, err = tx.ExecContext(ctx, `
		UPDATE users SET credit = credit + $1, updated_at = NOW() WHERE user_id = $2
	`, amount, userID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE withdrawals
		SET status = 'failed', failure_reason = $1, updated_at = NOW()
		WHERE withdrawal_id = $2
	`, reason, withdrawalID); err != nil {
		return err
	}
	return tx.Commit()
}

// CompleteWithdrawal attaches Omise ids and advances status after transfer is created.
func (r *WalletRepository) CompleteWithdrawal(ctx context.Context, withdrawalID int64, status, omiseTransferID, omiseRecipientID string) error {
	status = strings.TrimSpace(status)
	if status == "" {
		status = entity.WithdrawStatusProcessing
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE withdrawals
		SET status = $1,
		    omise_transfer_id = NULLIF($2, ''),
		    omise_recipient_id = NULLIF($3, ''),
		    updated_at = NOW()
		WHERE withdrawal_id = $4 AND status NOT IN ($5, $6, $7)
	`, status, omiseTransferID, omiseRecipientID, withdrawalID,
		entity.WithdrawStatusCompleted, entity.WithdrawStatusFailed, "paid")
	return err
}

// WithdrawalIDByOmiseTransfer returns our withdrawal row for an Omise transfer id.
func (r *WalletRepository) WithdrawalIDByOmiseTransfer(ctx context.Context, transferID string) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		SELECT withdrawal_id FROM withdrawals WHERE omise_transfer_id = $1
	`, transferID).Scan(&id)
	return id, err
}

// UpdateWithdrawalStatusByTransfer sets status on a withdrawal matched by Omise transfer id.
func (r *WalletRepository) UpdateWithdrawalStatusByTransfer(ctx context.Context, transferID, status, failureReason string) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE withdrawals
		SET status = $1,
		    failure_reason = NULLIF($2, ''),
		    updated_at = NOW()
		WHERE omise_transfer_id = $3 AND status NOT IN ($4, $5, $6)
	`, status, failureReason, transferID,
		entity.WithdrawStatusCompleted, entity.WithdrawStatusFailed, "paid")
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// FailWithdrawalByTransferID refunds credit when Omise marks a transfer failed.
func (r *WalletRepository) FailWithdrawalByTransferID(ctx context.Context, transferID, reason string) error {
	id, err := r.WithdrawalIDByOmiseTransfer(ctx, transferID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	return r.FailWithdrawal(ctx, id, reason)
}

// SetUserOmiseRecipientID caches Omise recipient id on the user row.
func (r *WalletRepository) SetUserOmiseRecipientID(ctx context.Context, userID, recipientID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users SET omise_recipient_id = $1, updated_at = NOW() WHERE user_id = $2
	`, recipientID, userID)
	return err
}

// WithdrawalBlockedReason builds a Thai message from fulfillment block counts.
func WithdrawalBlockedReason(sellerN, buyerN int) string {
	if sellerN == 0 && buyerN == 0 {
		return ""
	}
	var parts []string
	if sellerN > 0 {
		parts = append(parts, "กรุณาบันทึกการจัดส่งสินค้าให้ครบในฐานะผู้ขาย")
	}
	if buyerN > 0 {
		parts = append(parts, "กรุณายืนยันรับของให้ครบในฐานะผู้ชนะประมูล")
	}
	return strings.Join(parts, " และ ") + " ก่อนจึงจะถอนเงินได้"
}

// ValidatePayoutProfile checks bank fields required for Omise transfer.
func ValidatePayoutProfile(p *entity.UserPayoutProfile) error {
	if p == nil {
		return ErrMissingBankAccount
	}
	if p.BankID <= 0 || strings.TrimSpace(p.BankCode) == "" {
		return fmt.Errorf("%w: bank not selected", ErrMissingBankAccount)
	}
	if strings.TrimSpace(p.BankAccountName) == "" {
		return fmt.Errorf("%w: account name required", ErrMissingBankAccount)
	}
	num := strings.TrimSpace(p.BankAccountNumber)
	if len(num) < 10 || len(num) > 16 {
		return fmt.Errorf("%w: invalid account number", ErrMissingBankAccount)
	}
	return nil
}
