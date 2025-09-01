package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/shini4i/atlantis-emoji-gate/internal/client"
	"github.com/shini4i/atlantis-emoji-gate/internal/config"
	"github.com/shini4i/atlantis-emoji-gate/internal/processor"
)

// fetchCodeOwnersContent retrieves the CODEOWNERS file content.
func fetchCodeOwnersContent(client client.GitlabClientInterface, cfg config.GitlabConfig, project *client.Project) (string, error) {
	if cfg.CodeOwnersRepo != "" {
		codeOwnersRepo, err := client.GetProject(cfg.CodeOwnersRepo)
		if err != nil {
			return "", fmt.Errorf("failed to get codeowners project: %w", err)
		}
		return client.GetFileContent(codeOwnersRepo.ID, codeOwnersRepo.DefaultBranch, cfg.CodeOwnersPath)
	}
	return client.GetFileContent(project.ID, project.DefaultBranch, cfg.CodeOwnersPath)
}

// CheckMandatoryApproval validates approvals against the CODEOWNERS file.
func CheckMandatoryApproval(client client.GitlabClientInterface, cfg config.GitlabConfig, projectID int, codeOwnersContent string, proc processor.Processor) (bool, error) {
	reactions, err := client.ListAwardEmojis(projectID, cfg.PullRequestID)
	if err != nil {
		return false, fmt.Errorf("failed to fetch reactions: %w", err)
	}

	var lastCommitTimestamp time.Time
	if cfg.Restricted {
		lastCommitTimestamp, err = client.GetLatestCommitTimestamp(projectID, cfg.PullRequestID)
		if err != nil {
			return false, fmt.Errorf("failed to fetch latest commit timestamp: %w", err)
		}
	}

	for _, reaction := range reactions {
		if cfg.Restricted && reaction.UpdatedAt.Before(lastCommitTimestamp) {
			fmt.Printf("--> Skipping outdated approval by %s from %v\n", reaction.User.Username, reaction.UpdatedAt)
			continue
		}

		isApproved, err := proc.CheckApproval(strings.NewReader(codeOwnersContent), reaction, cfg)
		if err != nil {
			return false, fmt.Errorf("error during approval check: %w", err)
		}

		if isApproved {
			fmt.Printf("--> Mandatory approval provided by: %s\n", reaction.User.Username)
			return true, nil
		}
	}

	fmt.Println("Mandatory approval not found")
	return false, nil
}

// ProcessMR orchestrates the high-level workflow for processing a merge request.
func ProcessMR(client client.GitlabClientInterface, cfg config.GitlabConfig, proc processor.Processor) (bool, error) {
	project, err := client.GetProject(fmt.Sprintf("%s/%s", cfg.BaseRepoOwner, cfg.BaseRepoName))
	if err != nil {
		return false, fmt.Errorf("failed to get project: %w", err)
	}

	codeOwnersContent, err := fetchCodeOwnersContent(client, cfg, project)
	if err != nil {
		return false, fmt.Errorf("failed to fetch CODEOWNERS file: %w", err)
	}

	return CheckMandatoryApproval(client, cfg, project.ID, codeOwnersContent, proc)
}

// Run is the primary entrypoint for the application logic.
func Run(client client.GitlabClientInterface, cfg config.GitlabConfig, proc processor.Processor) int {
	if cfg.Insecure {
		fmt.Println("Insecure mode enabled: MR author can approve their own MR if they are in CODEOWNERS")
	}

	approved, err := ProcessMR(client, cfg, proc)
	if err != nil {
		fmt.Printf("Error processing MR: %v\n", err)
		return 1
	}

	if approved {
		return 0
	}
	return 1
}

func main() {
	cfg, err := config.NewGitlabConfig()
	if err != nil {
		panic(fmt.Sprintf("Error parsing GitLab config: %v", err))
	}

	gitlabClient := client.NewGitlabClient(cfg.Url, cfg.Token)
	proc := processor.NewProcessor()

	os.Exit(Run(gitlabClient, cfg, proc))
}
