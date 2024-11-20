package main

import (
	"encoding/base64"
	"fmt"

	"github.com/xanzy/go-gitlab"
)

// GitlabClient implements GitlabClientInterface using go-gitlab client.
type GitlabClient struct {
	client GitlabClientInterface
	mrId   int
}

// Init initializes the GitLab client.
func (g *GitlabClient) Init(cfg *GitlabConfig) error {
	if g.client == nil {
		realClient := &realGitlabClient{}
		err := realClient.Init(cfg)
		if err != nil {
			return err
		}
		g.client = realClient
	}
	g.mrId = cfg.PullRequestID
	return nil
}

func (g *GitlabClient) GetProjectIDFromPath(path string) (int, error) {
	return g.client.GetProjectIDFromPath(path)
}

func (g *GitlabClient) FindDefaultBranch(projectID int) (string, error) {
	return g.client.FindDefaultBranch(projectID)
}

func (g *GitlabClient) GetFileContentFromBranch(projectID int, branch, filePath string) (string, error) {
	return g.client.GetFileContentFromBranch(projectID, branch, filePath)
}

func (g *GitlabClient) ListAwardEmoji() ([]*gitlab.AwardEmoji, error) {
	return g.client.ListAwardEmoji()
}

// realGitlabClient wraps an instance of gitlab.Client.
type realGitlabClient struct {
	client          *gitlab.Client
	projects        *gitlab.ProjectsService
	repositoryFiles *gitlab.RepositoryFilesService
	awardEmoji      *gitlab.AwardEmojiService
	projectPath     string
	mrId            int
}

func (r *realGitlabClient) Init(cfg *GitlabConfig) error {
	gitlabUrl := fmt.Sprintf("%s://%s", "https", cfg.Url)
	client, err := gitlab.NewClient(cfg.Token, gitlab.WithBaseURL(gitlabUrl))
	if err != nil {
		return err
	}

	r.client = client
	r.projects = r.client.Projects
	r.repositoryFiles = r.client.RepositoryFiles
	r.awardEmoji = r.client.AwardEmoji
	r.mrId = cfg.PullRequestID
	r.projectPath = fmt.Sprintf("%s/%s", cfg.BaseRepoOwner, cfg.BaseRepoName)
	return nil
}

func (r *realGitlabClient) GetProjectIDFromPath(path string) (int, error) {
	project, _, err := r.projects.GetProject(path, nil)
	if err != nil {
		return 0, err
	}
	return project.ID, nil
}

func (r *realGitlabClient) FindDefaultBranch(projectID int) (string, error) {
	project, _, err := r.projects.GetProject(projectID, nil)
	if err != nil {
		return "", err
	}
	return project.DefaultBranch, nil
}

func (r *realGitlabClient) GetFileContentFromBranch(projectID int, branch, filePath string) (string, error) {
	opt := &gitlab.GetFileOptions{Ref: gitlab.Ptr(branch)}
	file, _, err := r.repositoryFiles.GetFile(projectID, filePath, opt)
	if err != nil {
		return "", err
	}

	decodedContent, err := base64.StdEncoding.DecodeString(file.Content)
	if err != nil {
		return "", err
	}

	return string(decodedContent), nil
}

func (r *realGitlabClient) ListAwardEmoji() ([]*gitlab.AwardEmoji, error) {
	project, _, err := r.projects.GetProject(r.projectPath, nil)
	if err != nil {
		return nil, err
	}

	reactions, _, err := r.awardEmoji.ListMergeRequestAwardEmoji(project.ID, r.mrId, nil)
	if err != nil {
		return nil, err
	}

	return reactions, nil
}
