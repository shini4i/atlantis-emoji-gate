package main

import (
	"encoding/base64"
	"fmt"

	"github.com/xanzy/go-gitlab"
)

// GitlabClientInterface defines the methods required for interacting with GitLab.
type GitlabClientInterface interface {
	Init(cfg *GitlabConfig) error
	GetProjectIDFromPath(path string) (int, error)
	FindDefaultBranch(projectID int) (string, error)
	GetFileContentFromBranch(projectID int, branch, filePath string) (string, error)
	ListAwardEmoji() ([]*gitlab.AwardEmoji, error)
}

type GitlabClient struct {
	client      *gitlab.Client
	mrId        int
	projectPath string
}

// Init initializes the GitLab client.
func (g *GitlabClient) Init(cfg *GitlabConfig) error {
	gitlabUrl := fmt.Sprintf("%s://%s", "https", cfg.Url)
	client, err := gitlab.NewClient(cfg.Token, gitlab.WithBaseURL(gitlabUrl))
	if err != nil {
		return err
	}

	g.client = client
	g.mrId = cfg.PullRequestID
	g.projectPath = fmt.Sprintf("%s/%s", cfg.BaseRepoOwner, cfg.BaseRepoName)

	return nil
}

// GetProjectIDFromPath fetches the project ID for the given repository path.
func (g *GitlabClient) GetProjectIDFromPath(path string) (int, error) {
	project, _, err := g.client.Projects.GetProject(path, nil)
	if err != nil {
		return 0, err
	}

	return project.ID, nil
}

// FindDefaultBranch retrieves the default branch for a given project ID.
func (g *GitlabClient) FindDefaultBranch(projectID int) (string, error) {
	project, _, err := g.client.Projects.GetProject(projectID, nil)
	if err != nil {
		return "", err
	}

	return project.DefaultBranch, nil
}

// GetFileContentFromBranch retrieves the content of a specific file from a branch.
func (g *GitlabClient) GetFileContentFromBranch(projectID int, branch, filePath string) (string, error) {
	opt := &gitlab.GetFileOptions{
		Ref: gitlab.Ptr(branch),
	}
	file, _, err := g.client.RepositoryFiles.GetFile(projectID, filePath, opt)
	if err != nil {
		return "", err
	}

	decodedContent, err := base64.StdEncoding.DecodeString(file.Content)
	if err != nil {
		return "", err
	}

	return string(decodedContent), nil
}

// ListAwardEmoji fetches all reactions (emojis) on the current Merge Request.
func (g *GitlabClient) ListAwardEmoji() ([]*gitlab.AwardEmoji, error) {
	project, _, err := g.client.Projects.GetProject(g.projectPath, nil)
	if err != nil {
		return nil, err
	}

	reactions, _, err := g.client.AwardEmoji.ListMergeRequestAwardEmoji(project.ID, g.mrId, nil)
	if err != nil {
		return nil, err
	}

	return reactions, nil
}
