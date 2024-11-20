package main

import (
	"fmt"
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
}

func TestGitlabClient_GetProjectIDFromPath(t *testing.T) {
	mockClient := &MockGitlabClient{
		ProjectID:      123,
		ProjectIDError: nil,
	}

	client := &GitlabClient{client: mockClient}
	projectID, err := client.GetProjectIDFromPath("test-owner/test-repo")

	assert.NoError(t, err)
	assert.Equal(t, 123, projectID)

	// Test case with error
	mockClient.ProjectIDError = fmt.Errorf("project not found")
	projectID, err = client.GetProjectIDFromPath("test-owner/test-repo")

	assert.Error(t, err)
	assert.Equal(t, 0, projectID)
}

func TestGitlabClient_FindDefaultBranch(t *testing.T) {
	mockClient := &MockGitlabClient{
		DefaultBranch:      "main",
		DefaultBranchError: nil,
	}

	client := &GitlabClient{client: mockClient}
	branch, err := client.FindDefaultBranch(123)

	assert.NoError(t, err)
	assert.Equal(t, "main", branch)

	// Test case with error
	mockClient.DefaultBranchError = fmt.Errorf("branch not found")
	branch, err = client.FindDefaultBranch(123)

	assert.Error(t, err)
	assert.Empty(t, branch)
}

func TestGitlabClient_GetFileContentFromBranch(t *testing.T) {
	mockContent := "mock file content"
	mockClient := &MockGitlabClient{
		FileContent:      mockContent,
		FileContentError: nil,
	}

	client := &GitlabClient{client: mockClient}
	content, err := client.GetFileContentFromBranch(123, "main", "README.md")

	assert.NoError(t, err)
	assert.Equal(t, mockContent, content)

	// Test case with error
	mockClient.FileContentError = fmt.Errorf("file not found")
	content, err = client.GetFileContentFromBranch(123, "main", "README.md")

	assert.Error(t, err)
	assert.Empty(t, content)
}

func TestGitlabClient_ListAwardEmoji(t *testing.T) {
	mockEmojis := []*gitlab.AwardEmoji{
		{ID: 1, Name: "thumbsup"},
		{ID: 2, Name: "thumbsdown"},
	}
	mockClient := &MockGitlabClient{
		AwardEmojiList:      mockEmojis,
		AwardEmojiListError: nil,
	}

	client := &GitlabClient{client: mockClient}
	emojis, err := client.ListAwardEmoji()

	assert.NoError(t, err)
	assert.Equal(t, mockEmojis, emojis)

	// Test case with error
	mockClient.AwardEmojiListError = fmt.Errorf("cannot get emojis")
	emojis, err = client.ListAwardEmoji()

	assert.Error(t, err)
	assert.Nil(t, emojis)
}
