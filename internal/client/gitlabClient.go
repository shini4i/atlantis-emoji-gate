package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// defaultTimeout is the default HTTP client timeout for API requests.
	defaultTimeout = 30 * time.Second
	// maxPerPage is the maximum number of items per page for paginated GitLab API requests.
	maxPerPage = 100
)

//go:generate mockgen -destination=mocks/mock_client.go -package=mocks . GitlabClientInterface

// GitlabClientInterface defines the methods that must be implemented by a GitLab client.
// GetMrCommits is intentionally excluded as it is only used internally by GetLatestCommitTimestamp.
type GitlabClientInterface interface {
	GetProject(ctx context.Context, projectPath string) (*Project, error)
	ListAwardEmojis(ctx context.Context, projectID, mrID int) ([]*AwardEmoji, error)
	GetFileContent(ctx context.Context, projectID int, branch, filePath string) (string, error)
	GetLatestCommitTimestamp(ctx context.Context, projectID, mrID int) (time.Time, error)
}

// GitlabClient implements GitlabClientInterface using the GitLab REST API.
type GitlabClient struct {
	scheme  string
	baseURL string
	token   string
	client  *http.Client
}

// Project represents a GitLab project.
type Project struct {
	ID            int    `json:"id"`
	DefaultBranch string `json:"default_branch"`
}

// AwardEmoji represents an emoji reaction on a GitLab merge request.
type AwardEmoji struct {
	Name      string    `json:"name"`
	User      User      `json:"user"`
	UpdatedAt time.Time `json:"updated_at"`
}

// User represents a GitLab user.
type User struct {
	Username string `json:"username"`
}

// Commit represents a GitLab commit.
type Commit struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

// NewGitlabClient creates a new GitlabClient with the given base URL and token.
func NewGitlabClient(baseURL, token string) *GitlabClient {
	return &GitlabClient{
		scheme:  "https",
		baseURL: baseURL,
		token:   token,
		client:  &http.Client{Timeout: defaultTimeout},
	}
}

// doGet performs an HTTP GET request with context and returns the response body and headers.
func (g *GitlabClient) doGet(ctx context.Context, path string) ([]byte, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s://%s/api/v4/%s", g.scheme, g.baseURL, path), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Private-Token", g.token)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			slog.Warn("Failed to close response body", "error", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("received non-200 response: %d with failed body read: %w", resp.StatusCode, err)
		}
		return nil, nil, fmt.Errorf("received non-200 response: %d - %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, resp.Header, nil
}

// get sends a GET request to the specified path and decodes the response into the target.
func (g *GitlabClient) get(ctx context.Context, path string, target any) error {
	body, _, err := g.doGet(ctx, path)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

// getAll sends paginated GET requests and collects all array results across pages.
// It appends per_page and page query parameters to the base path automatically.
// The next page number is read from GitLab's X-Next-Page response header.
// A safety limit prevents unbounded loops in case of API misbehavior.
func getAll[T any](ctx context.Context, g *GitlabClient, basePath string) ([]T, error) {
	const maxPages = 100 // Safety limit: 100 pages * 100 items = 10,000 items max

	var all []T
	separator := "?"
	if strings.Contains(basePath, "?") {
		separator = "&"
	}

	for page := "1"; page != ""; {
		path := fmt.Sprintf("%s%sper_page=%d&page=%s", basePath, separator, maxPerPage, page)
		body, headers, err := g.doGet(ctx, path)
		if err != nil {
			return nil, err
		}

		var pageItems []T
		if err := json.Unmarshal(body, &pageItems); err != nil {
			return nil, fmt.Errorf("failed to unmarshal page %s: %w", page, err)
		}

		all = append(all, pageItems...)

		page = headers.Get("X-Next-Page")
		if page != "" {
			pageNum, _ := strconv.Atoi(page)
			if pageNum > maxPages {
				return nil, fmt.Errorf("pagination exceeded safety limit of %d pages", maxPages)
			}
		}
	}

	return all, nil
}

// GetProject retrieves the project details for the given project path.
func (g *GitlabClient) GetProject(ctx context.Context, projectPath string) (*Project, error) {
	escapedPath := url.PathEscape(projectPath)
	var project Project
	err := g.get(ctx, fmt.Sprintf("projects/%s", escapedPath), &project)
	return &project, err
}

// ListAwardEmojis lists all award emojis for the specified project and merge request.
// Results are paginated to ensure all emojis are retrieved.
func (g *GitlabClient) ListAwardEmojis(ctx context.Context, projectID, mrID int) ([]*AwardEmoji, error) {
	path := fmt.Sprintf("projects/%d/merge_requests/%d/award_emoji", projectID, mrID)
	return getAll[*AwardEmoji](ctx, g, path)
}

// GetFileContent retrieves the content of the specified file in the given project and branch.
func (g *GitlabClient) GetFileContent(ctx context.Context, projectID int, branch, filePath string) (string, error) {
	encodedPath := url.PathEscape(filePath)
	var content struct {
		Content string `json:"content"`
	}
	err := g.get(ctx, fmt.Sprintf("projects/%d/repository/files/%s?ref=%s", projectID, encodedPath, url.QueryEscape(branch)), &content)
	if err != nil {
		return "", err
	}
	decodedContent, err := base64.StdEncoding.DecodeString(content.Content)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 file content: %w", err)
	}
	return string(decodedContent), nil
}

// GetMrCommits retrieves all commits for the specified project and merge request.
// Results are paginated to ensure all commits are retrieved.
func (g *GitlabClient) GetMrCommits(ctx context.Context, projectID, mrID int) ([]*Commit, error) {
	path := fmt.Sprintf("projects/%d/merge_requests/%d/commits", projectID, mrID)
	return getAll[*Commit](ctx, g, path)
}

// GetLatestCommitTimestamp retrieves the timestamp of the latest commit for the specified project and merge request.
func (g *GitlabClient) GetLatestCommitTimestamp(ctx context.Context, projectID, mrID int) (time.Time, error) {
	commits, err := g.GetMrCommits(ctx, projectID, mrID)
	if err != nil {
		return time.Time{}, err
	}
	if len(commits) == 0 {
		return time.Time{}, fmt.Errorf("no commits found for MR %d", mrID)
	}
	return commits[0].CreatedAt, nil
}
