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

func TestGitlabClient_GetProjectIDFromPath(t *testing.T) {
	mockClient := &MockGitlabClient{
		ProjectID: 123,
	}

	projectID, err := mockClient.GetProjectIDFromPath("test-path")
	assert.NoError(t, err)
	assert.Equal(t, 123, projectID)
}

func TestGitlabClient_FindDefaultBranch(t *testing.T) {
	mockClient := &MockGitlabClient{
		DefaultBranch: "main",
	}

	branch, err := mockClient.FindDefaultBranch(123)
	assert.NoError(t, err)
	assert.Equal(t, "main", branch)
}

func TestGitlabClient_GetFileContentFromBranch(t *testing.T) {
	mockClient := &MockGitlabClient{
		FileContent: "mock-content",
	}

	content, err := mockClient.GetFileContentFromBranch(123, "main", "test-path")
	assert.NoError(t, err)
	assert.Equal(t, "mock-content", content)
}

func TestGitlabClient_ListAwardEmoji(t *testing.T) {
	// Mocking the exact inline struct for AwardEmoji.User
	mockClient := &MockGitlabClient{
		AwardEmojiList: []*gitlab.AwardEmoji{
			{
				Name: "thumbsup",
				User: struct {
					Name      string `json:"name"`
					Username  string `json:"username"`
					ID        int    `json:"id"`
					State     string `json:"state"`
					AvatarURL string `json:"avatar_url"`
					WebURL    string `json:"web_url"`
				}{
					Username: "test-user",
				},
			},
		},
	}

	emojis, err := mockClient.ListAwardEmoji()
	assert.NoError(t, err)
	assert.Len(t, emojis, 1)
	assert.Equal(t, "thumbsup", emojis[0].Name)
	assert.Equal(t, "test-user", emojis[0].User.Username)
}
