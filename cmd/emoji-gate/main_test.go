package main

import (
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/shini4i/atlantis-emoji-gate/internal/client"
	"github.com/shini4i/atlantis-emoji-gate/internal/config"
	"github.com/stretchr/testify/assert"
)

type MockProcessor struct {
	CheckApprovalFunc func(r io.Reader, reaction *client.AwardEmoji, cfg config.GitlabConfig) (bool, error)
}

func (m *MockProcessor) CheckApproval(r io.Reader, reaction *client.AwardEmoji, cfg config.GitlabConfig) (bool, error) {
	if m.CheckApprovalFunc != nil {
		return m.CheckApprovalFunc(r, reaction, cfg)
	}
	return false, errors.New("mock CheckApproval not implemented")
}

type MockGitlabClient struct {
	GetProjectFunc               func(repo string) (*client.Project, error)
	GetFileContentFunc           func(projectID int, branch string, path string) (string, error)
	ListAwardEmojisFunc          func(projectID int, pullRequestID int) ([]*client.AwardEmoji, error)
	GetLatestCommitTimestampFunc func(projectID int, mrID int) (time.Time, error)
	GetMrCommitsFunc             func(projectID int, mrID int) ([]*client.Commit, error)
}

func (m *MockGitlabClient) GetProject(repo string) (*client.Project, error) {
	if m.GetProjectFunc != nil {
		return m.GetProjectFunc(repo)
	}
	return nil, errors.New("mock GetProject not implemented")
}

func (m *MockGitlabClient) GetFileContent(pID int, b string, p string) (string, error) {
	if m.GetFileContentFunc != nil {
		return m.GetFileContentFunc(pID, b, p)
	}
	return "", errors.New("mock GetFileContent not implemented")
}

func (m *MockGitlabClient) ListAwardEmojis(pID int, prID int) ([]*client.AwardEmoji, error) {
	if m.ListAwardEmojisFunc != nil {
		return m.ListAwardEmojisFunc(pID, prID)
	}
	return nil, errors.New("mock ListAwardEmojis not implemented")
}

func (m *MockGitlabClient) GetLatestCommitTimestamp(pID int, mrID int) (time.Time, error) {
	if m.GetLatestCommitTimestampFunc != nil {
		return m.GetLatestCommitTimestampFunc(pID, mrID)
	}
	return time.Time{}, errors.New("mock GetLatestCommitTimestamp not implemented")
}

func (m *MockGitlabClient) GetMrCommits(pID int, mrID int) ([]*client.Commit, error) {
	if m.GetMrCommitsFunc != nil {
		return m.GetMrCommitsFunc(pID, mrID)
	}
	return nil, errors.New("mock GetMrCommits not implemented")
}

// -------------------- Unit Tests --------------------

func TestCheckMandatoryApproval_Success(t *testing.T) {
	mockClient := &MockGitlabClient{}
	mockProcessor := &MockProcessor{}
	cfg := config.GitlabConfig{PullRequestID: 123}
	reaction := &client.AwardEmoji{User: client.User{Username: "approver"}}

	// Set the behavior for this test case.
	mockClient.ListAwardEmojisFunc = func(pID int, prID int) ([]*client.AwardEmoji, error) {
		assert.Equal(t, 1, pID)
		assert.Equal(t, 123, prID)
		return []*client.AwardEmoji{reaction}, nil
	}
	mockProcessor.CheckApprovalFunc = func(r io.Reader, reac *client.AwardEmoji, c config.GitlabConfig) (bool, error) {
		assert.Equal(t, reaction, reac)
		return true, nil
	}

	approved, err := CheckMandatoryApproval(mockClient, cfg, 1, "content", mockProcessor)

	assert.NoError(t, err)
	assert.True(t, approved)
}

func TestProcessMR_Failure(t *testing.T) {
	mockClient := &MockGitlabClient{}
	mockProcessor := &MockProcessor{}
	cfg := config.GitlabConfig{BaseRepoOwner: "owner", BaseRepoName: "repo"}

	mockClient.GetProjectFunc = func(repo string) (*client.Project, error) {
		assert.Equal(t, "owner/repo", repo)
		return nil, errors.New("project error")
	}

	_, err := ProcessMR(mockClient, cfg, mockProcessor)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get project")
}

func TestRun_Success(t *testing.T) {
	mockClient := &MockGitlabClient{}
	mockProcessor := &MockProcessor{}
	cfg := config.GitlabConfig{}
	project := &client.Project{ID: 1, DefaultBranch: "main"}
	reaction := &client.AwardEmoji{User: client.User{Username: "approver"}}

	// Mock the entire successful flow
	mockClient.GetProjectFunc = func(repo string) (*client.Project, error) { return project, nil }
	mockClient.GetFileContentFunc = func(pID int, b string, p string) (string, error) { return "content", nil }
	mockClient.ListAwardEmojisFunc = func(pID int, prID int) ([]*client.AwardEmoji, error) { return []*client.AwardEmoji{reaction}, nil }
	mockProcessor.CheckApprovalFunc = func(r io.Reader, reac *client.AwardEmoji, c config.GitlabConfig) (bool, error) { return true, nil }

	exitCode := Run(mockClient, cfg, mockProcessor)
	assert.Equal(t, 0, exitCode)
}

func TestRun_Error(t *testing.T) {
	mockClient := &MockGitlabClient{}
	mockProcessor := &MockProcessor{}
	cfg := config.GitlabConfig{}

	// Mock the single point of failure
	mockClient.GetProjectFunc = func(repo string) (*client.Project, error) {
		return nil, errors.New("gitlab is down")
	}

	// Capture stdout to test the error message
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	exitCode := Run(mockClient, cfg, mockProcessor)

	err := w.Close()
	if err != nil {
		return
	}
	capturedOutputBytes, _ := io.ReadAll(r)
	os.Stdout = originalStdout
	capturedOutput := string(capturedOutputBytes)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, capturedOutput, "Error processing MR: failed to get project: gitlab is down")
}
