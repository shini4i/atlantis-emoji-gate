package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type CodeOwnersProcessor struct{}

type CodeOwner struct {
	Owner string
	Path  string
}

// ParseCodeOwners extracts all owners from the CODEOWNERS content.
func (co *CodeOwnersProcessor) ParseCodeOwners(reader io.Reader) ([]CodeOwner, error) {
	var owners []CodeOwner
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 {
			for _, owner := range parts[1:] {
				owners = append(owners, CodeOwner{
					Owner: strings.TrimPrefix(owner, "@"),
					Path:  parts[0],
				})
			}
		} else {
			fmt.Printf("Warning: Ignored malformed line: %s\n", line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading CODEOWNERS content: %w", err)
	}

	return owners, nil
}

// CanApprove checks if a user can approve based on CODEOWNERS rules.
func (co *CodeOwnersProcessor) CanApprove(owner CodeOwner, reaction *AwardEmoji, cfg GitlabConfig) bool {
	ownerPath := strings.TrimPrefix(owner.Path, "/")
	terraformPath := strings.TrimPrefix(cfg.TerraformPath, "/")

	if owner.Owner != reaction.User.Username {
		return false
	}
	if reaction.Name != cfg.ApproveEmoji {
		return false
	}
	if reaction.User.Username == cfg.MrAuthor && !cfg.Insecure {
		fmt.Printf("MR author '%s' cannot approve their own MR\n", cfg.MrAuthor)
		return false
	}
	if ownerPath == "*" || strings.HasPrefix(terraformPath, ownerPath) {
		return true
	}
	return false
}
