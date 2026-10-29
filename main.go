package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/boavizta/helloasso-renew-contribution/internal/application"
	"github.com/boavizta/helloasso-renew-contribution/internal/config"
	"github.com/boavizta/helloasso-renew-contribution/services/baserow"
	"github.com/boavizta/helloasso-renew-contribution/services/brevo"
	"github.com/boavizta/helloasso-renew-contribution/services/helloasso"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	if err := run(logger); err != nil {
		logger.Error("Membership renewal failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	if cfg.DryRun {
		logger.Warn("DRY_RUN enabled: no Baserow update and no email will be sent")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	renewal := application.NewRenewal(
		helloasso.NewClient(cfg.HelloAsso, client),
		baserow.NewClient(cfg.Baserow, client),
		brevo.NewClient(cfg.Brevo, client),
		time.Now, logger, cfg.Email,
	)
	return renewal.Run()
}
