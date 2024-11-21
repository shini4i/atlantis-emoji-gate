package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// GitlabClientInterface defines the methods that must be implemented by a GitLab client.
type GitlabClientInterface interface {
	GetProject(projectPath string) (*Project, error)
	ListAwardEmojis(projectID, mrID int) ([]*AwardEmoji, error)
	GetFileContent(projectID int, branch, filePath string) (string, error)
}

type GitlabClient struct {
	Scheme  string
	BaseURL string
	Token   string
	client  *http.Client
}

type Project struct {
	ID            int    `json:"id"`
	DefaultBranch string `json:"default_branch"`
}

type AwardEmoji struct {
	Name string `json:"name"`
	User struct {
		Username string `json:"username"`
	} `json:"user"`
}

func NewGitlabClient(baseURL, token string) *GitlabClient {
	return &GitlabClient{
		Scheme:  "https",
		BaseURL: baseURL,
		Token:   token,
		client:  &http.Client{},
	}
}

func (g *GitlabClient) get(path string, target interface{}) error {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s://%s/api/v4/%s", g.Scheme, g.BaseURL, path), nil)
	if err != nil {
		return err
	}
	req.Header.Add("Private-Token", g.Token)
	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, target)
}

func (g *GitlabClient) GetProject(projectPath string) (*Project, error) {
	escapedPath := url.PathEscape(projectPath)
	var project Project
	err := g.get(fmt.Sprintf("/projects/%s", escapedPath), &project)
	return &project, err
}

func (g *GitlabClient) ListAwardEmojis(projectID, mrID int) ([]*AwardEmoji, error) {
	var emojis []*AwardEmoji
	err := g.get(fmt.Sprintf("/projects/%d/merge_requests/%d/award_emoji", projectID, mrID), &emojis)
	return emojis, err
}

func (g *GitlabClient) GetFileContent(projectID int, branch, filePath string) (string, error) {
	var content struct {
		Content string `json:"content"`
	}
	err := g.get(fmt.Sprintf("/projects/%d/repository/files/%s?ref=%s", projectID, filePath, branch), &content)
	if err != nil {
		return "", err
	}
	decodedContent, err := base64.StdEncoding.DecodeString(content.Content)
	if err != nil {
		return "", err
	}
	return string(decodedContent), nil
}
