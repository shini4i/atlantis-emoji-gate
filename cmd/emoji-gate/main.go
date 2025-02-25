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

// fetchCodeOwnersContent retrieves the CODEOWNERS file content based on configuration.
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

// CheckMandatoryApproval validates approvals against CODEOWNERS.
func CheckMandatoryApproval(client client.GitlabClientInterface, cfg config.GitlabConfig, projectID int, codeOwnersContent string, processor processor.CodeOwnersProcessorInterface) (bool, error) {
	owners, err := processor.ParseCodeOwners(strings.NewReader(codeOwnersContent))
	if err != nil {
		return false, fmt.Errorf("failed to parse CODEOWNERS: %w", err)
	}

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

	approvedBy := filterApprovals(owners, reactions, cfg, lastCommitTimestamp, processor)
	if len(approvedBy) > 0 {
		fmt.Printf("--> Mandatory approval provided by: %v\n", approvedBy)
		return true, nil
	}

	fmt.Println("Mandatory approval not found")
	return false, nil
}

// filterApprovals identifies valid approvers from reactions.
func filterApprovals(owners []processor.CodeOwner, reactions []*client.AwardEmoji, cfg config.GitlabConfig, lastCommitTimestamp time.Time, processor processor.CodeOwnersProcessorInterface) []string {
	var approvedBy []string

	for _, reaction := range reactions {
		for _, owner := range owners {
			if processor.CanApprove(owner, reaction, cfg) {
				if cfg.Restricted && reaction.UpdatedAt.Before(lastCommitTimestamp) {
					fmt.Printf("--> Skipping outdated approval by %s\n", reaction.User.Username)
					continue
				}
				approvedBy = append(approvedBy, reaction.User.Username)
			}
		}
	}

	return approvedBy
}

// ProcessMR handles the overall MR processing workflow.
func ProcessMR(client client.GitlabClientInterface, cfg config.GitlabConfig, processor processor.CodeOwnersProcessorInterface) (bool, error) {
	project, err := client.GetProject(fmt.Sprintf("%s/%s", cfg.BaseRepoOwner, cfg.BaseRepoName))
	if err != nil {
		return false, fmt.Errorf("failed to get project: %w", err)
	}

	codeOwnersContent, err := fetchCodeOwnersContent(client, cfg, project)
	if err != nil {
		return false, fmt.Errorf("failed to fetch CODEOWNERS file: %w", err)
	}

	return CheckMandatoryApproval(client, cfg, project.ID, codeOwnersContent, processor)
}

// Run handles the program's main logic.
func Run(client client.GitlabClientInterface, cfg config.GitlabConfig, processor processor.CodeOwnersProcessorInterface) int {
	if cfg.Insecure {
		fmt.Println("Insecure mode enabled: MR author can approve their own MR if they are in CODEOWNERS")
	}

	approved, err := ProcessMR(client, cfg, processor)
	if err != nil {
		fmt.Printf("Error processing MR: %v\n", err)
		return 1
	}

	return map[bool]int{true: 0, false: 1}[approved]
}

func main() {
	cfg, err := config.NewGitlabConfig()
	if err != nil {
		panic(fmt.Sprintf("Error parsing GitLab config: %v", err))
	}

	codeOwnersProcessor := &processor.CodeOwnersProcessor{}
	client := client.NewGitlabClient(cfg.Url, cfg.Token)
	os.Exit(Run(client, cfg, codeOwnersProcessor))
}
