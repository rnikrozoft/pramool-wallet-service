package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/rnikrozoft/pramool-wallet-service/internal/config"
)

type PlatformSettingsRepository struct {
	db *sql.DB
}

func NewPlatformSettingsRepository(db *sql.DB) *PlatformSettingsRepository {
	return &PlatformSettingsRepository{db: db}
}

func (r *PlatformSettingsRepository) LoadWalletFeesStrict(ctx context.Context) (config.WalletFeesConfig, error) {
	var raw []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT setting_value FROM platform_settings WHERE setting_key = 'fees' LIMIT 1
	`).Scan(&raw)
	if err != nil {
		return config.WalletFeesConfig{}, fmt.Errorf("platform_settings.fees: %w", err)
	}
	if len(raw) == 0 {
		return config.WalletFeesConfig{}, fmt.Errorf("platform_settings.fees is empty")
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return config.WalletFeesConfig{}, fmt.Errorf("platform_settings.fees invalid json: %w", err)
	}
	return config.ParseWalletFees(m)
}
