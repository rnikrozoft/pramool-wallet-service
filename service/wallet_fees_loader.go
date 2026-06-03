package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rnikrozoft/pramool-wallet-service/internal/config"
	"github.com/rnikrozoft/pramool-wallet-service/repository"
)

const walletFeesCacheTTL = 60 * time.Second

type WalletFeesLoader struct {
	repo    *repository.PlatformSettingsRepository
	mu      sync.Mutex
	cached  config.WalletFeesConfig
	expires time.Time
}

func NewWalletFeesLoader(ctx context.Context, repo *repository.PlatformSettingsRepository) (*WalletFeesLoader, error) {
	if repo == nil {
		return nil, fmt.Errorf("platform_settings repository is nil")
	}
	l := &WalletFeesLoader{repo: repo}
	if err := l.reload(ctx); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *WalletFeesLoader) Get(ctx context.Context) (config.WalletFeesConfig, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if time.Now().After(l.expires) {
		if err := l.reload(ctx); err != nil {
			return config.WalletFeesConfig{}, err
		}
	}
	return l.cached, nil
}

func (l *WalletFeesLoader) MustGet(ctx context.Context) config.WalletFeesConfig {
	cfg, err := l.Get(ctx)
	if err != nil {
		panic("platform_settings.fees reload failed: " + err.Error())
	}
	return cfg
}

func (l *WalletFeesLoader) reload(ctx context.Context) error {
	cfg, err := l.repo.LoadWalletFeesStrict(ctx)
	if err != nil {
		return err
	}
	l.cached = cfg
	l.expires = time.Now().Add(walletFeesCacheTTL)
	return nil
}
