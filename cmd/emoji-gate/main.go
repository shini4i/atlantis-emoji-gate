package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/shini4i/atlantis-emoji-gate/internal/client"
	"github.com/shini4i/atlantis-emoji-gate/internal/config"
	"github.com/shini4i/atlantis-emoji-gate/internal/gate"
	"github.com/shini4i/atlantis-emoji-gate/internal/processor"
)

// runTimeout bounds the whole gate run so a stalled GitLab API cannot hang the
// Atlantis apply step indefinitely.
const runTimeout = 2 * time.Minute

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	cfg, err := config.NewGitlabConfig()
	if err != nil {
		slog.Error("Error parsing GitLab config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	gitlabClient := client.NewGitlabClient(cfg.URL, cfg.Token)
	proc := processor.NewProcessor()

	exitCode := gate.Run(ctx, gitlabClient, cfg, proc)
	cancel()
	os.Exit(exitCode)
}
