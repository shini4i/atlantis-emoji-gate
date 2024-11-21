package main

import (
	"bufio"
	"fmt"
	"os"
	"slices"
	"strings"
)

// ParseCodeOwners extracts owners for the global pattern '*' from the CODEOWNERS content.
func ParseCodeOwners(content string) ([]string, error) {
	var owners []string
	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[0] == "*" {
			for _, owner := range parts[1:] {
				owners = append(owners, strings.TrimPrefix(owner, "@"))
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading CODEOWNERS content: %w", err)
	}
	return owners, nil
}

// fetchCodeOwnersContent retrieves the CODEOWNERS file content based on configuration.
func fetchCodeOwnersContent(client GitlabClientInterface, cfg GitlabConfig, project *Project) (string, error) {
	if cfg.CodeOwnersRepo != "" {
		codeOwnersRepo, err := client.GetProject(cfg.CodeOwnersRepo)
		if err != nil {
			return "", fmt.Errorf("failed to get codeowners project: %w", err)
		}
		return client.GetFileContent(codeOwnersRepo.ID, project.DefaultBranch, cfg.CodeOwnersPath)
	}
	return client.GetFileContent(project.ID, project.DefaultBranch, cfg.CodeOwnersPath)
}

// CheckMandatoryApproval validates approvals against CODEOWNERS.
func CheckMandatoryApproval(client GitlabClientInterface, cfg GitlabConfig, projectID int, codeOwnersContent string) (bool, error) {
	owners, err := ParseCodeOwners(codeOwnersContent)
	if err != nil {
		return false, fmt.Errorf("failed to parse CODEOWNERS: %w", err)
	}

	reactions, err := client.ListAwardEmojis(projectID, cfg.PullRequestID)
	if err != nil {
		return false, fmt.Errorf("failed to fetch reactions: %w", err)
	}

	approvedBy := filterApprovals(owners, reactions, cfg)
	if len(approvedBy) > 0 {
		fmt.Printf("Mandatory approval provided by: %v", approvedBy)
		return true, nil
	}

	fmt.Println("Mandatory approval not found")
	return false, nil
}

// filterApprovals identifies valid approvers from reactions.
func filterApprovals(owners []string, reactions []*AwardEmoji, cfg GitlabConfig) []string {
	var approvedBy []string

	for _, reaction := range reactions {
		if slices.Contains(owners, reaction.User.Username) && reaction.Name == cfg.ApproveEmoji {
			if reaction.User.Username == cfg.MrAuthor && !cfg.Insecure {
				fmt.Printf("MR author '%s' cannot approve their own MR", cfg.MrAuthor)
				continue
			}
			approvedBy = append(approvedBy, reaction.User.Username)
		}
	}

	return approvedBy
}

// ProcessMR handles the overall MR processing workflow.
func ProcessMR(client GitlabClientInterface, cfg GitlabConfig) (bool, error) {
	project, err := client.GetProject(fmt.Sprintf("%s/%s", cfg.BaseRepoOwner, cfg.BaseRepoName))
	if err != nil {
		return false, fmt.Errorf("failed to get project: %w", err)
	}

	codeOwnersContent, err := fetchCodeOwnersContent(client, cfg, project)
	if err != nil {
		return false, fmt.Errorf("failed to fetch CODEOWNERS file: %w", err)
	}

	return CheckMandatoryApproval(client, cfg, project.ID, codeOwnersContent)
}

// Run handles the program's main logic.
func Run(client GitlabClientInterface, cfg GitlabConfig) int {
	if cfg.Insecure {
		fmt.Println("Insecure mode enabled: MR author can approve their own MR if they are in CODEOWNERS")
	}

	approved, err := ProcessMR(client, cfg)
	if err != nil {
		fmt.Printf("Error processing MR: %v", err)
		return 1
	}

	return map[bool]int{true: 0, false: 1}[approved]
}

func main() {
	cfg, err := NewGitlabConfig()
	if err != nil {
		panic(fmt.Sprintf("Error parsing GitLab config: %v", err))
	}

	client := NewGitlabClient(cfg.Url, cfg.Token)
	os.Exit(Run(client, cfg))
}
