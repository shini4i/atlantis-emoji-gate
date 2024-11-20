package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xanzy/go-gitlab"
)

// MockGitlabClient implements GitlabClientInterface for testing purposes.
type MockGitlabClient struct {
	InitError           error
	ProjectID           int
	ProjectIDError      error
	DefaultBranch       string
	DefaultBranchError  error
	FileContent         string
	FileContentError    error
	AwardEmojiList      []*gitlab.AwardEmoji
	AwardEmojiListError error
}

func (m *MockGitlabClient) Init(cfg *GitlabConfig) error {
	return m.InitError
}

func (m *MockGitlabClient) GetProjectIDFromPath(path string) (int, error) {
	if m.ProjectIDError != nil {
		return 0, m.ProjectIDError
	}
	return m.ProjectID, nil
}

func (m *MockGitlabClient) FindDefaultBranch(projectID int) (string, error) {
	if m.DefaultBranchError != nil {
		return "", m.DefaultBranchError
	}
	return m.DefaultBranch, nil
}

func (m *MockGitlabClient) GetFileContentFromBranch(projectID int, branch, filePath string) (string, error) {
	if m.FileContentError != nil {
		return "", m.FileContentError
	}
	return m.FileContent, nil
}

func (m *MockGitlabClient) ListAwardEmoji() ([]*gitlab.AwardEmoji, error) {
	if m.AwardEmojiListError != nil {
		return nil, m.AwardEmojiListError
	}
	return m.AwardEmojiList, nil
}

func TestGitlabClient_Init(t *testing.T) {
	client := &GitlabClient{}
	cfg := &GitlabConfig{
		Url:           "gitlab.example.com",
		Token:         "mock-token",
		BaseRepoOwner: "test-owner",
		BaseRepoName:  "test-repo",
		PullRequestID: 1,
	}

	err := client.Init(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, client.client)
	assert.Equal(t, 1, client.mrId)
	assert.Equal(t, "test-owner/test-repo", client.projectPath)
}
