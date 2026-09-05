package config

import (
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// allVars lists every environment variable NewGitlabConfig reads.
var allVars = []string{
	"ATLANTIS_GITLAB_HOSTNAME", "ATLANTIS_GITLAB_TOKEN", "APPROVE_EMOJI",
	"BASE_REPO_OWNER", "BASE_REPO_NAME", "PULL_NUM", "REPO_REL_DIR",
	"CODEOWNERS_PATH", "CODEOWNERS_REPO", "PULL_AUTHOR", "INSECURE", "RESTRICTED",
}

// clearEnv unsets every variable the config reads so the ambient shell cannot
// leak into a test, and restores them when the test ends.
func clearEnv(t *testing.T) {
	t.Helper()
	if fields := reflect.TypeOf(GitlabConfig{}).NumField(); len(allVars) != fields {
		t.Fatalf("allVars lists %d variables but GitlabConfig has %d fields; update the list", len(allVars), fields)
	}
	for _, name := range allVars {
		if value, ok := os.LookupEnv(name); ok {
			t.Cleanup(func() { _ = os.Setenv(name, value) })
			_ = os.Unsetenv(name)
		}
	}
}

// setRequired sets the variables Atlantis always provides.
func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("ATLANTIS_GITLAB_HOSTNAME", "gitlab.example.com")
	t.Setenv("ATLANTIS_GITLAB_TOKEN", "example-token")
	t.Setenv("BASE_REPO_OWNER", "example-owner")
	t.Setenv("BASE_REPO_NAME", "example-repo")
	t.Setenv("PULL_NUM", "123")
	t.Setenv("PULL_AUTHOR", "example-author")
	t.Setenv("REPO_REL_DIR", "terraform/provision")
}

func TestNewGitlabConfig(t *testing.T) {
	t.Run("missing required environment variables", func(t *testing.T) {
		clearEnv(t)

		_, err := NewGitlabConfig()
		assert.Error(t, err)
	})

	t.Run("security toggles default to the safe value", func(t *testing.T) {
		clearEnv(t)
		setRequired(t)

		cfg, err := NewGitlabConfig()
		assert.NoError(t, err)

		assert.False(t, cfg.Insecure, "self-approval must stay disabled unless INSECURE is set")
		assert.False(t, cfg.Restricted)
		assert.Equal(t, "thumbsup", cfg.ApproveEmoji)
		assert.Equal(t, "CODEOWNERS", cfg.CodeOwnersPath)
		assert.Empty(t, cfg.CodeOwnersRepo)
	})

	t.Run("all environment variables set", func(t *testing.T) {
		clearEnv(t)
		setRequired(t)
		t.Setenv("APPROVE_EMOJI", "rocket")
		t.Setenv("CODEOWNERS_PATH", ".test/CODEOWNERS")
		t.Setenv("CODEOWNERS_REPO", "shared/codeowners")
		t.Setenv("INSECURE", "true")
		t.Setenv("RESTRICTED", "true")

		cfg, err := NewGitlabConfig()
		assert.NoError(t, err)

		assert.Equal(t, "gitlab.example.com", cfg.URL)
		assert.Equal(t, "example-token", cfg.Token)
		assert.Equal(t, "rocket", cfg.ApproveEmoji)
		assert.Equal(t, "example-author", cfg.MrAuthor)
		assert.Equal(t, "example-owner", cfg.BaseRepoOwner)
		assert.Equal(t, "example-repo", cfg.BaseRepoName)
		assert.Equal(t, 123, cfg.PullRequestID)
		assert.Equal(t, ".test/CODEOWNERS", cfg.CodeOwnersPath)
		assert.Equal(t, "shared/codeowners", cfg.CodeOwnersRepo)
		assert.Equal(t, "terraform/provision", cfg.TerraformPath)
		assert.True(t, cfg.Insecure)
		assert.True(t, cfg.Restricted)
	})
}
