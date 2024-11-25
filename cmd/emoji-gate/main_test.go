package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockGitlabClient is a mock implementation of the GitlabClientInterface.
type MockGitlabClient struct {
	Project        *Project
	AwardEmojis    []*AwardEmoji
	FileContent    string
	ProjectErr     error
	EmojisErr      error
	FileContentErr error
}

func (m *MockGitlabClient) GetProject(projectPath string) (*Project, error) {
	return m.Project, m.ProjectErr
}

func (m *MockGitlabClient) ListAwardEmojis(projectID, mrID int) ([]*AwardEmoji, error) {
	return m.AwardEmojis, m.EmojisErr
}

func (m *MockGitlabClient) GetFileContent(projectID int, branch, filePath string) (string, error) {
	return m.FileContent, m.FileContentErr
}

// TestParseCodeOwners tests the parsing of the CODEOWNERS file.
func TestParseCodeOwners(t *testing.T) {
	content := "* @user1\n"
	expectedOwners := []CodeOwner{{Owner: "user1", Path: "*"}}
	codeOwnersProcessor := CodeOwnersProcessor{}

	owners, err := codeOwnersProcessor.ParseCodeOwners(content)
	assert.NoError(t, err)
	assert.Equal(t, expectedOwners, owners)
}

// TestCheckMandatoryApproval tests the CheckMandatoryApproval function.
func TestCheckMandatoryApproval(t *testing.T) {
	cfg := GitlabConfig{
		ApproveEmoji:  "thumbsup",
		MrAuthor:      "author",
		Insecure:      false,
		PullRequestID: 1,
		TerraformPath: ".",
	}

	mockClient := &MockGitlabClient{
		AwardEmojis: []*AwardEmoji{
			{Name: "thumbsup", User: struct {
				Username string `json:"username"`
			}{Username: "user1"}},
		},
	}

	codeOwnersContent := "* @user1\n"

	approved, err := CheckMandatoryApproval(mockClient, cfg, 1, codeOwnersContent)
	assert.NoError(t, err)
	assert.True(t, approved)
}

// TestProcessMR tests the ProcessMR function.
func TestProcessMR(t *testing.T) {
	cfg := GitlabConfig{
		BaseRepoOwner:  "owner",
		BaseRepoName:   "repo",
		CodeOwnersPath: "CODEOWNERS",
		Insecure:       false,
		PullRequestID:  1,
		MrAuthor:       "author",
		ApproveEmoji:   "thumbsup",
		TerraformPath:  ".",
	}

	mockClient := &MockGitlabClient{
		Project: &Project{
			ID:            1,
			DefaultBranch: "main",
		},
		FileContent: "* @user1\n",
		AwardEmojis: []*AwardEmoji{
			{
				Name: "thumbsup",
				User: struct {
					Username string `json:"username"`
				}{
					Username: "user1",
				}},
		},
	}

	approved, err := ProcessMR(mockClient, cfg)
	assert.NoError(t, err)
	assert.True(t, approved)
}

// TestRun tests the Run function.
func TestRun(t *testing.T) {
	t.Run("MR author can approve their own MR", func(t *testing.T) {
		cfg := GitlabConfig{
			BaseRepoOwner:  "owner",
			BaseRepoName:   "repo",
			CodeOwnersPath: "CODEOWNERS",
			Insecure:       true,
			MrAuthor:       "author",
			ApproveEmoji:   "thumbsup",
			PullRequestID:  1,
			TerraformPath:  ".",
		}

		mockClient := &MockGitlabClient{
			Project: &Project{
				ID:            1,
				DefaultBranch: "main",
			},
			FileContent: "* @user1\n",
			AwardEmojis: []*AwardEmoji{
				{
					Name: "thumbsup",
					User: struct {
						Username string `json:"username"`
					}{
						Username: "user1",
					}},
			},
		}

		exitCode := Run(mockClient, cfg)
		assert.Equalf(t, 0, exitCode, "Expected exit code 0, got %d", exitCode)
	})

	t.Run("MR author cannot approve their own MR", func(t *testing.T) {
		cfg := GitlabConfig{
			BaseRepoOwner:  "owner",
			BaseRepoName:   "repo",
			CodeOwnersPath: "CODEOWNERS",
			Insecure:       false,
			MrAuthor:       "user1",
			ApproveEmoji:   "thumbsup",
			PullRequestID:  1,
			TerraformPath:  ".",
		}

		mockClient := &MockGitlabClient{
			Project: &Project{
				ID:            1,
				DefaultBranch: "main",
			},
			FileContent: "* @user1\n",
			AwardEmojis: []*AwardEmoji{
				{
					Name: "thumbsup",
					User: struct {
						Username string `json:"username"`
					}{
						Username: "user1",
					}},
			},
		}

		exitCode := Run(mockClient, cfg)
		assert.Equalf(t, 1, exitCode, "Expected exit code 1, got %d", exitCode)
	})
}
