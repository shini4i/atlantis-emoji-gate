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

// -------------------- Manual Mock Implementations --------------------

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

func TestFetchCodeOwnersContent_ErrorPaths(t *testing.T) {
	t.Run("Failure on GetProject", func(t *testing.T) {
		mockClient := &MockGitlabClient{}
		cfg := config.GitlabConfig{CodeOwnersRepo: "codeowners/repo"}
		mockClient.GetProjectFunc = func(repo string) (*client.Project, error) {
			return nil, errors.New("project not found")
		}

		_, err := fetchCodeOwnersContent(mockClient, cfg, &client.Project{ID: 1})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get codeowners project")
	})

	t.Run("Failure on GetFileContent", func(t *testing.T) {
		mockClient := &MockGitlabClient{}
		cfg := config.GitlabConfig{}
		project := &client.Project{ID: 1, DefaultBranch: "main"}
		mockClient.GetFileContentFunc = func(pID int, b string, p string) (string, error) {
			return "", errors.New("file not found")
		}

		_, err := fetchCodeOwnersContent(mockClient, cfg, project)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})
}

func TestCheckMandatoryApproval(t *testing.T) {
	t.Run("Success on second reaction", func(t *testing.T) {
		mockClient := &MockGitlabClient{}
		mockProcessor := &MockProcessor{}
		cfg := config.GitlabConfig{PullRequestID: 123}
		reactionValid := &client.AwardEmoji{User: client.User{Username: "approver"}}
		reactionInvalid := &client.AwardEmoji{User: client.User{Username: "non-approver"}}

		mockClient.ListAwardEmojisFunc = func(pID int, prID int) ([]*client.AwardEmoji, error) {
			return []*client.AwardEmoji{reactionInvalid, reactionValid}, nil
		}

		// This function will be called twice.
		callCount := 0
		mockProcessor.CheckApprovalFunc = func(r io.Reader, reac *client.AwardEmoji, c config.GitlabConfig) (bool, error) {
			callCount++
			if reac.User.Username == "approver" {
				return true, nil
			}
			return false, nil
		}

		approved, err := CheckMandatoryApproval(mockClient, cfg, 1, "content", mockProcessor)
		assert.NoError(t, err)
		assert.True(t, approved)
		assert.Equal(t, 2, callCount, "CheckApproval should have been called twice")
	})

	t.Run("Restricted mode skips old and finds new approval", func(t *testing.T) {
		mockClient := &MockGitlabClient{}
		mockProcessor := &MockProcessor{}
		cfg := config.GitlabConfig{PullRequestID: 123, Restricted: true}
		commitTime := time.Now()
		reactionOld := &client.AwardEmoji{User: client.User{Username: "approver"}, UpdatedAt: commitTime.Add(-time.Hour)}
		reactionNew := &client.AwardEmoji{User: client.User{Username: "approver"}, UpdatedAt: commitTime.Add(time.Hour)}

		mockClient.ListAwardEmojisFunc = func(pID int, prID int) ([]*client.AwardEmoji, error) {
			return []*client.AwardEmoji{reactionOld, reactionNew}, nil
		}
		mockClient.GetLatestCommitTimestampFunc = func(pID int, mrID int) (time.Time, error) {
			return commitTime, nil
		}

		// The processor should only be called once, for the new reaction.
		callCount := 0
		mockProcessor.CheckApprovalFunc = func(r io.Reader, reac *client.AwardEmoji, c config.GitlabConfig) (bool, error) {
			callCount++
			assert.Equal(t, reactionNew, reac, "Processor should only be called with the new reaction")
			return true, nil
		}

		approved, err := CheckMandatoryApproval(mockClient, cfg, 1, "content", mockProcessor)
		assert.NoError(t, err)
		assert.True(t, approved)
		assert.Equal(t, 1, callCount, "CheckApproval should have only been called once")
	})

	t.Run("Error on ListAwardEmojis", func(t *testing.T) {
		mockClient := &MockGitlabClient{}
		mockClient.ListAwardEmojisFunc = func(pID int, prID int) ([]*client.AwardEmoji, error) {
			return nil, errors.New("api error")
		}
		_, err := CheckMandatoryApproval(mockClient, config.GitlabConfig{}, 1, "", &MockProcessor{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch reactions")
	})

	t.Run("Error on GetLatestCommitTimestamp", func(t *testing.T) {
		mockClient := &MockGitlabClient{}
		cfg := config.GitlabConfig{Restricted: true}
		mockClient.ListAwardEmojisFunc = func(pID int, prID int) ([]*client.AwardEmoji, error) { return nil, nil }
		mockClient.GetLatestCommitTimestampFunc = func(pID int, mrID int) (time.Time, error) {
			return time.Time{}, errors.New("commit error")
		}
		_, err := CheckMandatoryApproval(mockClient, cfg, 1, "", &MockProcessor{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch latest commit timestamp")
	})
}

// Keeping the simplified Run tests as they cover the logic of that function well.
func TestRun_Success(t *testing.T) {
	mockClient := &MockGitlabClient{}
	mockProcessor := &MockProcessor{}
	cfg := config.GitlabConfig{}
	project := &client.Project{ID: 1, DefaultBranch: "main"}
	reaction := &client.AwardEmoji{User: client.User{Username: "approver"}}

	mockClient.GetProjectFunc = func(repo string) (*client.Project, error) { return project, nil }
	mockClient.GetFileContentFunc = func(pID int, b string, p string) (string, error) { return "content", nil }
	mockClient.ListAwardEmojisFunc = func(pID int, prID int) ([]*client.AwardEmoji, error) { return []*client.AwardEmoji{reaction}, nil }
	mockProcessor.CheckApprovalFunc = func(r io.Reader, reac *client.AwardEmoji, c config.GitlabConfig) (bool, error) { return true, nil }

	exitCode := Run(mockClient, cfg, mockProcessor)
	assert.Equal(t, 0, exitCode)
}

func TestRun_Error(t *testing.T) {
	mockClient := &MockGitlabClient{}
	cfg := config.GitlabConfig{}

	mockClient.GetProjectFunc = func(repo string) (*client.Project, error) {
		return nil, errors.New("gitlab is down")
	}

	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	exitCode := Run(mockClient, cfg, &MockProcessor{})

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
