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
