package main

import "github.com/xanzy/go-gitlab"

// GitlabClientInterface defines the methods required for interacting with GitLab.
type GitlabClientInterface interface {
	Init(cfg *GitlabConfig) error
	GetProjectIDFromPath(path string) (int, error)
	FindDefaultBranch(projectID int) (string, error)
	GetFileContentFromBranch(projectID int, branch, filePath string) (string, error)
	ListAwardEmoji() ([]*gitlab.AwardEmoji, error)
}
