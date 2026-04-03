package session

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"claude-go-code/pkg/types"
)

type GCConfig struct {
	TTL         time.Duration
	IdleTimeout time.Duration
	Interval    time.Duration
}

func DefaultGCConfig() GCConfig {
	return GCConfig{
		TTL:         24 * time.Hour,
		IdleTimeout: 1 * time.Hour,
		Interval:    10 * time.Minute,
	}
}

type GarbageCollector struct {
	store  Store
	config GCConfig
	logger *slog.Logger
	stopCh chan struct{}
}

func NewGarbageCollector(store Store, cfg GCConfig, logger *slog.Logger) *GarbageCollector {
	return &GarbageCollector{
		store:  store,
		config: cfg,
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

func (gc *GarbageCollector) Start(ctx context.Context) {
	go gc.loop(ctx)
}

func (gc *GarbageCollector) Stop() {
	close(gc.stopCh)
}

func (gc *GarbageCollector) loop(ctx context.Context) {
	ticker := time.NewTicker(gc.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-gc.stopCh:
			return
		case <-ticker.C:
			cleaned, err := gc.RunOnce(ctx)
			if err != nil {
				gc.logger.Error("session gc failed", "error", err)
			} else if cleaned > 0 {
				gc.logger.Info("session gc completed", "cleaned", cleaned)
			}
		}
	}
}

func (gc *GarbageCollector) RunOnce(ctx context.Context) (int, error) {
	summaries, err := gc.store.List(ctx)
	if err != nil {
		return 0, err
	}

	now := time.Now()
	cleaned := 0

	for _, s := range summaries {
		expired := false

		if gc.config.TTL > 0 && now.Sub(s.CreatedAt) > gc.config.TTL {
			expired = true
		}
		if gc.config.IdleTimeout > 0 && now.Sub(s.UpdatedAt) > gc.config.IdleTimeout {
			expired = true
		}

		if expired {
			if err := gc.store.Delete(ctx, s.ID); err != nil {
				gc.logger.Warn("failed to delete expired session", "id", s.ID, "error", err)
				continue
			}
			cleaned++
		}
	}

	return cleaned, nil
}

func GCFileStore(dir string, ttl time.Duration, idleTimeout time.Duration) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	now := time.Now()
	cleaned := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var sess types.Session
		if err := json.Unmarshal(data, &sess); err != nil {
			continue
		}

		expired := false
		if ttl > 0 && now.Sub(sess.CreatedAt) > ttl {
			expired = true
		}
		if idleTimeout > 0 && now.Sub(sess.UpdatedAt) > idleTimeout {
			expired = true
		}

		if expired {
			if err := os.Remove(path); err == nil {
				cleaned++
			}
		}
	}

	return cleaned, nil
}
