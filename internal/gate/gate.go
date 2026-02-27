package gate

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/shini4i/atlantis-emoji-gate/internal/client"
	"github.com/shini4i/atlantis-emoji-gate/internal/config"
	"github.com/shini4i/atlantis-emoji-gate/internal/processor"
)

// fetchCodeOwnersContent retrieves the CODEOWNERS file content.
// If a separate CODEOWNERS repository is configured, it fetches from there;
// otherwise, it fetches from the merge request's project.
func fetchCodeOwnersContent(ctx context.Context, gc client.GitlabClientInterface, cfg config.GitlabConfig, project *client.Project) (string, error) {
	if cfg.CodeOwnersRepo != "" {
		codeOwnersRepo, err := gc.GetProject(ctx, cfg.CodeOwnersRepo)
		if err != nil {
			return "", fmt.Errorf("failed to get codeowners project: %w", err)
		}
		return gc.GetFileContent(ctx, codeOwnersRepo.ID, codeOwnersRepo.DefaultBranch, cfg.CodeOwnersPath)
	}
	return gc.GetFileContent(ctx, project.ID, project.DefaultBranch, cfg.CodeOwnersPath)
}

// CheckMandatoryApproval validates that a merge request has received an
// approval emoji from a user listed in the CODEOWNERS file.
// In restricted mode, only approvals made after the latest commit are considered.
func CheckMandatoryApproval(ctx context.Context, gc client.GitlabClientInterface, cfg config.GitlabConfig, projectID int, codeOwnersContent string, proc processor.Processor) (bool, error) {
	reactions, err := gc.ListAwardEmojis(ctx, projectID, cfg.PullRequestID)
	if err != nil {
		return false, fmt.Errorf("failed to fetch reactions: %w", err)
	}

	if len(reactions) == 0 {
		slog.Warn("Mandatory approval not found")
		return false, nil
	}

	var lastCommitTimestamp time.Time
	if cfg.Restricted {
		lastCommitTimestamp, err = gc.GetLatestCommitTimestamp(ctx, projectID, cfg.PullRequestID)
		if err != nil {
			return false, fmt.Errorf("failed to fetch latest commit timestamp: %w", err)
		}
	}

	for _, reaction := range reactions {
		if cfg.Restricted && reaction.UpdatedAt.Before(lastCommitTimestamp) {
			slog.Info("Skipping outdated approval", "user", reaction.User.Username, "updated_at", reaction.UpdatedAt)
			continue
		}

		isApproved, err := proc.CheckApproval(strings.NewReader(codeOwnersContent), reaction, cfg)
		if err != nil {
			return false, fmt.Errorf("error during approval check: %w", err)
		}

		if isApproved {
			slog.Info("Mandatory approval provided", "user", reaction.User.Username)
			return true, nil
		}
	}

	slog.Warn("Mandatory approval not found")
	return false, nil
}

// ProcessMR orchestrates the high-level workflow for processing a merge request.
// It fetches the project, retrieves the CODEOWNERS file, and checks for mandatory approval.
func ProcessMR(ctx context.Context, gc client.GitlabClientInterface, cfg config.GitlabConfig, proc processor.Processor) (bool, error) {
	project, err := gc.GetProject(ctx, fmt.Sprintf("%s/%s", cfg.BaseRepoOwner, cfg.BaseRepoName))
	if err != nil {
		return false, fmt.Errorf("failed to get project: %w", err)
	}

	codeOwnersContent, err := fetchCodeOwnersContent(ctx, gc, cfg, project)
	if err != nil {
		return false, fmt.Errorf("failed to fetch CODEOWNERS file: %w", err)
	}

	return CheckMandatoryApproval(ctx, gc, cfg, project.ID, codeOwnersContent, proc)
}

// Run is the primary entrypoint for the application logic.
// It returns 0 on successful approval, 1 otherwise.
func Run(ctx context.Context, gc client.GitlabClientInterface, cfg config.GitlabConfig, proc processor.Processor) int {
	if cfg.Insecure {
		slog.Warn("Insecure mode enabled: MR author can approve their own MR if they are in CODEOWNERS")
	}

	approved, err := ProcessMR(ctx, gc, cfg, proc)
	if err != nil {
		slog.Error("Error processing MR", "error", err)
		return 1
	}

	if approved {
		return 0
	}
	return 1
}
