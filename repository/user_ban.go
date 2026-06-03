package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

func (r *WalletRepository) IsUserBanned(ctx context.Context, userID string) (bool, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false, nil
	}
	var banned bool
	err := r.db.QueryRowContext(ctx, `
		SELECT (
			suspended_at IS NOT NULL
			OR (restricted_until IS NOT NULL AND restricted_until > NOW())
		)
		FROM users
		WHERE user_id = $1
	`, userID).Scan(&banned)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return banned, err
}
