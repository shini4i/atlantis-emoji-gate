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

// NewGitlabClient creates a new GitlabClient with the given base URL and token.
func NewGitlabClient(baseURL, token string) *GitlabClient {
	return &GitlabClient{
		Scheme:  "https",
		BaseURL: baseURL,
		Token:   token,
		client:  &http.Client{},
	}
}

// get sends a GET request to the specified path and decodes the response into the target.
func (g *GitlabClient) get(path string, target interface{}) error {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s://%s/api/v4/%s", g.Scheme, g.BaseURL, path), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Private-Token", g.Token)

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			fmt.Println("failed to close response body:", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("received non-200 response: %d with failed body read: %w", resp.StatusCode, err)
		}
		return fmt.Errorf("received non-200 response: %d - %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

// GetProject retrieves the project details for the given project path.
func (g *GitlabClient) GetProject(projectPath string) (*Project, error) {
	escapedPath := url.PathEscape(projectPath)
	var project Project
	err := g.get(fmt.Sprintf("/projects/%s", escapedPath), &project)
	return &project, err
}

// ListAwardEmojis lists all award emojis for the specified project and merge request.
func (g *GitlabClient) ListAwardEmojis(projectID, mrID int) ([]*AwardEmoji, error) {
	var emojis []*AwardEmoji
	err := g.get(fmt.Sprintf("/projects/%d/merge_requests/%d/award_emoji", projectID, mrID), &emojis)
	return emojis, err
}

// GetFileContent retrieves the content of the specified file in the given project and branch.
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
		return "", fmt.Errorf("failed to decode base64 file content: %w", err)
	}
	return string(decodedContent), nil
}
