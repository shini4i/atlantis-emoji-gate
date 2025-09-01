package processor

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/shini4i/atlantis-emoji-gate/internal/client"
	"github.com/shini4i/atlantis-emoji-gate/internal/config"
)

// Processor defines the contract for checking approvals against a CODEOWNERS file.
type Processor interface {
	CheckApproval(codeowners io.Reader, reaction *client.AwardEmoji, cfg config.GitlabConfig) (bool, error)
}

// approvalProcessor provides a concrete implementation of the Processor interface.
type approvalProcessor struct{}

// NewProcessor creates and returns a new instance of the approval processor.
func NewProcessor() Processor {
	return &approvalProcessor{}
}

// isPathMatch is a helper to check if a path matches a CODEOWNERS pattern.
// This reduces the complexity of the main CheckApproval function.
func isPathMatch(pattern, path string) (bool, error) {
	if pattern == "*" {
		return true, nil
	}
	return filepath.Match(pattern, path)
}

// CheckApproval determines if a given user reaction constitutes a valid approval.
func (p *approvalProcessor) CheckApproval(codeowners io.Reader, reaction *client.AwardEmoji, cfg config.GitlabConfig) (bool, error) {
	if reaction.Name != cfg.ApproveEmoji {
		return false, nil
	}
	if !cfg.Insecure && reaction.User.Username == cfg.MrAuthor {
		return false, nil
	}

	var applicableOwners []string
	scanner := bufio.NewScanner(codeowners)
	cleanedTerraformPath := filepath.Clean(cfg.TerraformPath)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		pathPattern := parts[0]
		matched, err := isPathMatch(pathPattern, cleanedTerraformPath)
		if err != nil {
			return false, fmt.Errorf("invalid pattern '%s' in CODEOWNERS: %w", pathPattern, err)
		}

		if matched {
			applicableOwners = make([]string, 0, len(parts)-1)
			for _, owner := range parts[1:] {
				applicableOwners = append(applicableOwners, strings.TrimPrefix(owner, "@"))
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("error reading CODEOWNERS content: %w", err)
	}

	if applicableOwners == nil {
		return false, nil
	}

	for _, owner := range applicableOwners {
		if owner == reaction.User.Username {
			return true, nil
		}
	}

	return false, nil
}
