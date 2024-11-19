package main

import (
	"fmt"
	"testing"

	"github.com/xanzy/go-gitlab"
)

func TestRun_Success(t *testing.T) {
	mockClient := &MockGitlabClient{
		ProjectID:     123,
		DefaultBranch: "main",
		FileContent:   "* @user1",
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
					Username: "user1",
				},
			},
		},
	}

	cfg := &GitlabConfig{
		BaseRepoOwner:  "test-owner",
		BaseRepoName:   "test-repo",
		CodeOwnersPath: "CODEOWNERS",
		ApproveEmoji:   "thumbsup",
		MrAuthor:       "user2",
		Insecure:       false,
	}

	exitCode := Run(mockClient, cfg)
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestRun_Failure(t *testing.T) {
	mockClient := &MockGitlabClient{
		ProjectID:     123,
		DefaultBranch: "main",
		FileContent:   "* @user1",
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
					Username: "user2", // MR author cannot approve
				},
			},
		},
	}

	cfg := &GitlabConfig{
		BaseRepoOwner:  "test-owner",
		BaseRepoName:   "test-repo",
		CodeOwnersPath: "CODEOWNERS",
		ApproveEmoji:   "thumbsup",
		MrAuthor:       "user2",
		Insecure:       false,
	}

	exitCode := Run(mockClient, cfg)
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestRun_Error(t *testing.T) {
	mockClient := &MockGitlabClient{
		InitError: fmt.Errorf("failed to initialize GitLab client"),
	}

	cfg := &GitlabConfig{
		BaseRepoOwner:  "test-owner",
		BaseRepoName:   "test-repo",
		CodeOwnersPath: "CODEOWNERS",
		ApproveEmoji:   "thumbsup",
		MrAuthor:       "user2",
		Insecure:       false,
	}

	exitCode := Run(mockClient, cfg)
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}
