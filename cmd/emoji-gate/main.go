package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/xanzy/go-gitlab"
)

func ParseCodeOwners(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatalf("Failed to close file: %v", err)
		}
	}(file)

	var owners []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		path := parts[0]
		if path == "*" {
			for _, owner := range parts[1:] {
				owners = append(owners, strings.TrimPrefix(owner, "@"))
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	fmt.Println("Owners found in CODEOWNERS file:", owners)

	return owners, nil
}

func checkMandatoryApproval(cfg GitlabConfig) (bool, error) {
	gitlabUrl := fmt.Sprintf("%s://%s", "https", cfg.Url)
	client, err := gitlab.NewClient(cfg.Token, gitlab.WithBaseURL(gitlabUrl))
	if err != nil {
		return false, err
	}

	projectPath := fmt.Sprintf("%s/%s", cfg.BaseRepoOwner, cfg.BaseRepoName)
	project, _, err := client.Projects.GetProject(projectPath, nil)
	if err != nil {
		return false, err
	}

	reactions, _, err := client.AwardEmoji.ListMergeRequestAwardEmoji(project.ID, cfg.PullRequestID, nil)
	if err != nil {
		return false, err
	}

	owners, err := ParseCodeOwners(cfg.CodeOwnersPath)
	if err != nil {
		return false, err
	}

	var approvedBy []string

	for _, reaction := range reactions {
		if slices.Contains(owners, reaction.User.Username) && reaction.Name == cfg.ApproveEmoji {
			approvedBy = append(approvedBy, reaction.User.Username)
		}
	}

	if len(approvedBy) > 0 {
		fmt.Println("Mandatory approval was provided by:", approvedBy)
		return true, nil
	}

	fmt.Println("Mandatory approval was not found")
	return false, nil
}

func main() {
	cfg := NewGitlabConfig()
	approved, err := checkMandatoryApproval(cfg)
	if err != nil {
		log.Fatalf("Error checking mandatory approval: %v", err)
	}

	if approved {
		os.Exit(0)
	} else {
		os.Exit(1)
	}
}
