package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGitlabConfig(t *testing.T) {
	// Set environment variables for the test
	t.Setenv("ATLANTIS_GITLAB_HOSTNAME", "gitlab.example.com")
	t.Setenv("ATLANTIS_GITLAB_TOKEN", "example-token")
	t.Setenv("APPROVE_EMOJI_CODE", "thumbsup")
	t.Setenv("BASE_REPO_OWNER", "example-owner")
	t.Setenv("BASE_REPO_NAME", "example-repo")
	t.Setenv("PULL_NUM", "123")
	t.Setenv("CODEOWNERS_PATH", ".test/CODEOWNERS")

	// Call the function to test
	cfg := NewGitlabConfig()

	// Assert the expected values
	assert.Equal(t, "gitlab.example.com", cfg.Url)
	assert.Equal(t, "example-token", cfg.Token)
	assert.Equal(t, "thumbsup", cfg.ApproveEmoji)
	assert.Equal(t, "example-owner", cfg.BaseRepoOwner)
	assert.Equal(t, "example-repo", cfg.BaseRepoName)
	assert.Equal(t, 123, cfg.PullRequestID)
	assert.Equal(t, ".test/CODEOWNERS", cfg.CodeOwnersPath)
}
