package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
)

// ParseCodeOwners parses the content of a CODEOWNERS file and extracts owners for the global pattern '*'.
func ParseCodeOwners(content string) ([]string, error) {
	reader := strings.NewReader(content)
	scanner := bufio.NewScanner(reader)

	var owners []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		// Check for the global '*' pattern.
		if parts[0] == "*" {
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

// CheckMandatoryApproval verifies if mandatory approvals are present based on CODEOWNERS.
func CheckMandatoryApproval(client GitlabClientInterface, cfg *GitlabConfig, codeOwnersContent string) (bool, error) {
	owners, err := ParseCodeOwners(codeOwnersContent)
	if err != nil {
		return false, fmt.Errorf("failed to parse CODEOWNERS: %w", err)
	}

	reactions, err := client.ListAwardEmoji()
	if err != nil {
		return false, fmt.Errorf("failed to fetch reactions: %w", err)
	}

	var approvedBy []string
	for _, reaction := range reactions {
		if slices.Contains(owners, reaction.User.Username) && reaction.Name == cfg.ApproveEmoji {
			if reaction.User.Username == cfg.MrAuthor && !cfg.Insecure {
				log.Printf("MR author '%s' cannot approve their own MR", cfg.MrAuthor)
				continue
			}
			approvedBy = append(approvedBy, reaction.User.Username)
		}
	}

	if len(approvedBy) > 0 {
		log.Printf("Mandatory approval provided by: %v", approvedBy)
		return true, nil
	}

	log.Println("Mandatory approval not found")
	return false, nil
}

// ProcessMR handles the MR processing, including approval checks.
func ProcessMR(client GitlabClientInterface, cfg *GitlabConfig) (bool, error) {
	projectPath := fmt.Sprintf("%s/%s", cfg.BaseRepoOwner, cfg.BaseRepoName)

	projectID, err := client.GetProjectIDFromPath(projectPath)
	if err != nil {
		return false, fmt.Errorf("failed to get project ID: %w", err)
	}

	branch, err := client.FindDefaultBranch(projectID)
	if err != nil {
		return false, fmt.Errorf("failed to find default branch: %w", err)
	}

	codeOwnersContent, err := client.GetFileContentFromBranch(projectID, branch, cfg.CodeOwnersPath)
	if err != nil {
		return false, fmt.Errorf("failed to fetch CODEOWNERS file: %w", err)
	}

	return CheckMandatoryApproval(client, cfg, codeOwnersContent)
}

// Run handles the main logic of the program and returns an exit code.
func Run(client GitlabClientInterface, cfg *GitlabConfig) int {
	if err := client.Init(cfg); err != nil {
		log.Printf("Error initializing GitLab client: %v", err)
		return 1
	}

	if cfg.Insecure {
		log.Println("Insecure mode enabled: MR author can approve their own MR")
	}

	approved, err := ProcessMR(client, cfg)
	if err != nil {
		log.Printf("Error processing MR: %v", err)
		return 1
	}

	if approved {
		return 0
	}
	return 1
}

func main() {
	cfg := NewGitlabConfig()
	client := &GitlabClient{}

	// Run the application and use the returned exit code.
	exitCode := Run(client, &cfg)
	os.Exit(exitCode)
}
