package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/shini4i/atlantis-emoji-gate/internal/client"
	"github.com/shini4i/atlantis-emoji-gate/internal/config"
	"github.com/shini4i/atlantis-emoji-gate/internal/gate"
	"github.com/shini4i/atlantis-emoji-gate/internal/processor"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	cfg, err := config.NewGitlabConfig()
	if err != nil {
		slog.Error("Error parsing GitLab config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	gitlabClient := client.NewGitlabClient(cfg.URL, cfg.Token)
	proc := processor.NewProcessor()

	os.Exit(gate.Run(ctx, gitlabClient, cfg, proc))
}
