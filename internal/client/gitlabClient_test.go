package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ------------------ Mock Types and Helpers ------------------

// MockRoundTripper is a configurable http.RoundTripper for testing.
type MockRoundTripper struct {
	RoundTripFunc func(req *http.Request) (*http.Response, error)
}

// RoundTrip satisfies the http.RoundTripper interface.
func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.RoundTripFunc(req)
}

// newMockTransport returns an http.RoundTripper with configurable status, body, and error.
func newMockTransport(status int, body string, mockErr error) http.RoundTripper {
	return &MockRoundTripper{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			if mockErr != nil {
				return nil, mockErr
			}
			return &http.Response{
				StatusCode: status,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		},
	}
}

// FaultyReadCloser simulates errors when reading or closing.
type FaultyReadCloser struct {
	FailRead  bool
	FailClose bool
}

// Read implements io.Reader, optionally returning an error.
func (f *FaultyReadCloser) Read(p []byte) (n int, err error) {
	if f.FailRead {
		return 0, fmt.Errorf("mocked read error")
	}
	copy(p, `{"key": "value"}`)
	return len(`{"key": "value"}`), io.EOF
}

// Close implements io.Closer, optionally returning an error.
func (f *FaultyReadCloser) Close() error {
	if f.FailClose {
		return fmt.Errorf("mocked close error")
	}
	return nil
}

// mockGitLabServer sets up a test server with predefined GitLab-like responses.
func mockGitLabServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v4/projects/mockProjectPath", func(w http.ResponseWriter, r *http.Request) {
		project := Project{
			ID:            1,
			DefaultBranch: "main",
		}
		response, _ := json.Marshal(project)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(response)
	})

	mux.HandleFunc("/api/v4/projects/1/merge_requests/1/award_emoji", func(w http.ResponseWriter, r *http.Request) {
		emojis := []*AwardEmoji{
			{Name: "thumbsup", User: User{Username: "user1"}},
		}
		response, _ := json.Marshal(emojis)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(response)
	})

	mux.HandleFunc("/api/v4/projects/1/repository/files/CODEOWNERS", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ref") != "main" {
			http.Error(w, "branch not found", http.StatusNotFound)
			return
		}
		content := base64.StdEncoding.EncodeToString([]byte("* @user1\n"))
		response, _ := json.Marshal(map[string]string{"content": content})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(response)
	})

	mux.HandleFunc("/api/v4/projects/1/merge_requests/1/versions", func(w http.ResponseWriter, r *http.Request) {
		versions := []DiffVersion{
			{ID: 108, CreatedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)},
			{ID: 110, CreatedAt: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)},
		}
		response, _ := json.Marshal(versions)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(response)
	})

	return httptest.NewServer(mux)
}

// newTestGitlabClient creates a test GitLab client pointing to the mock server.
func newTestGitlabClient(baseURL string) *GitlabClient {
	trimmed := strings.TrimPrefix(baseURL, "http://")
	client := NewGitlabClient(trimmed, "dummyToken")
	client.scheme = "http"
	return client
}

// ------------------ Actual Tests ------------------

func TestGitlabClient_SuccessCases(t *testing.T) {
	server := mockGitLabServer()
	defer server.Close()

	client := newTestGitlabClient(server.URL)

	t.Run("GetProject", func(t *testing.T) {
		project, err := client.GetProject(context.Background(), "mockProjectPath")
		assert.NoError(t, err)
		assert.Equal(t, 1, project.ID)
		assert.Equal(t, "main", project.DefaultBranch)
	})

	t.Run("ListAwardEmojis", func(t *testing.T) {
		emojis, err := client.ListAwardEmojis(context.Background(), 1, 1)
		assert.NoError(t, err)
		assert.Len(t, emojis, 1)
		assert.Equal(t, "thumbsup", emojis[0].Name)
		assert.Equal(t, "user1", emojis[0].User.Username)
	})

	t.Run("GetFileContent", func(t *testing.T) {
		content, err := client.GetFileContent(context.Background(), 1, "main", "CODEOWNERS")
		assert.NoError(t, err)
		assert.Equal(t, "* @user1\n", content)
	})
}

func TestGitlabClient_Timeout(t *testing.T) {
	client := NewGitlabClient("example.com", "token")
	assert.Equal(t, defaultTimeout, client.client.Timeout)
}

func TestGitlabClient_GetFileContent_URLEncoding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The path ".github/CODEOWNERS" should be URL-encoded as ".github%2FCODEOWNERS".
		// Go's http server decodes %2F in the Path, so we check RawPath instead.
		rawPath := r.URL.RawPath
		if rawPath == "" {
			rawPath = r.URL.Path
		}
		assert.Contains(t, rawPath, ".github%2FCODEOWNERS",
			"file path should be URL-encoded in the request")

		content := base64.StdEncoding.EncodeToString([]byte("* @user1\n"))
		response, _ := json.Marshal(map[string]string{"content": content})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(response)
	}))
	defer server.Close()

	client := newTestGitlabClient(server.URL)

	content, err := client.GetFileContent(context.Background(), 1, "main", ".github/CODEOWNERS")
	assert.NoError(t, err)
	assert.Equal(t, "* @user1\n", content)
}

// TestGitlabClient_Pagination verifies that paginated endpoints collect results from all pages.
func TestGitlabClient_Pagination(t *testing.T) {
	t.Run("ListAwardEmojis collects multiple pages", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			page := r.URL.Query().Get("page")
			pageNum, _ := strconv.Atoi(page)

			var emojis []*AwardEmoji
			switch pageNum {
			case 1:
				emojis = []*AwardEmoji{
					{Name: "thumbsup", User: User{Username: "user1"}},
					{Name: "thumbsup", User: User{Username: "user2"}},
				}
				w.Header().Set("X-Next-Page", "2")
			case 2:
				emojis = []*AwardEmoji{
					{Name: "thumbsup", User: User{Username: "user3"}},
				}
				// No X-Next-Page header means last page
			default:
				t.Errorf("unexpected page: %d", pageNum)
			}

			response, _ := json.Marshal(emojis)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(response)
		}))
		defer server.Close()

		client := newTestGitlabClient(server.URL)
		emojis, err := client.ListAwardEmojis(context.Background(), 1, 1)
		assert.NoError(t, err)
		assert.Len(t, emojis, 3)
		assert.Equal(t, "user1", emojis[0].User.Username)
		assert.Equal(t, "user2", emojis[1].User.Username)
		assert.Equal(t, "user3", emojis[2].User.Username)
	})

	t.Run("pagination error on second page propagates", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount == 1 {
				emojis := []*AwardEmoji{{Name: "thumbsup", User: User{Username: "user1"}}}
				response, _ := json.Marshal(emojis)
				w.Header().Set("X-Next-Page", "2")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(response)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("server error"))
			}
		}))
		defer server.Close()

		client := newTestGitlabClient(server.URL)
		emojis, err := client.ListAwardEmojis(context.Background(), 1, 1)
		assert.Error(t, err)
		assert.Nil(t, emojis)
	})

	t.Run("pagination with invalid JSON on page", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("not-json"))
		}))
		defer server.Close()

		client := newTestGitlabClient(server.URL)
		emojis, err := client.ListAwardEmojis(context.Background(), 1, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal page")
		assert.Nil(t, emojis)
	})

	t.Run("pagination sends per_page=100", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "100", r.URL.Query().Get("per_page"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
		}))
		defer server.Close()

		client := newTestGitlabClient(server.URL)
		_, _ = client.ListAwardEmojis(context.Background(), 1, 1)
	})

	t.Run("pagination respects X-Next-Page value", func(t *testing.T) {
		var requestedPages []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			page := r.URL.Query().Get("page")
			requestedPages = append(requestedPages, page)

			switch page {
			case "1":
				w.Header().Set("X-Next-Page", "2")
			case "2":
				w.Header().Set("X-Next-Page", "3")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
		}))
		defer server.Close()

		client := newTestGitlabClient(server.URL)
		_, err := client.ListAwardEmojis(context.Background(), 1, 1)
		assert.NoError(t, err)
		assert.Equal(t, []string{"1", "2", "3"}, requestedPages)
	})

	t.Run("pagination with existing query string uses & separator", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify the URL contains & instead of ? for pagination params
			assert.Contains(t, r.URL.RawQuery, "existing=true&per_page=100&page=1")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
		}))
		defer server.Close()

		client := newTestGitlabClient(server.URL)
		// Call getAll directly with a path that already has a query parameter
		result, err := getAll[*AwardEmoji](context.Background(), client, "projects/1/merge_requests/1/award_emoji?existing=true")
		assert.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("pagination safety limit bounds the number of requests", func(t *testing.T) {
		// A server that always points at the same next page must be cut off
		// by the iteration count, not by the page number it advertises.
		var calls int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Header().Set("X-Next-Page", "2")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
		}))
		defer server.Close()

		client := newTestGitlabClient(server.URL)
		emojis, err := client.ListAwardEmojis(context.Background(), 1, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "pagination exceeded safety limit")
		assert.Nil(t, emojis)
		assert.Equal(t, maxPages, calls)
	})

	t.Run("a fetch spanning exactly maxPages pages succeeds", func(t *testing.T) {
		var calls int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			if page < maxPages {
				w.Header().Set("X-Next-Page", strconv.Itoa(page+1))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `[{"name":"thumbsup","user":{"username":"user%d"}}]`, page)
		}))
		defer server.Close()

		client := newTestGitlabClient(server.URL)
		emojis, err := client.ListAwardEmojis(context.Background(), 1, 1)
		assert.NoError(t, err)
		assert.Len(t, emojis, maxPages)
		assert.Equal(t, maxPages, calls)
	})

	t.Run("a cancelled context aborts before any request is sent", func(t *testing.T) {
		// main.go relies on this to bound the whole run with one deadline.
		var calls int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		client := newTestGitlabClient(server.URL)
		emojis, err := client.ListAwardEmojis(ctx, 1, 1)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, emojis)
		assert.Equal(t, 0, calls)
	})

	t.Run("an expired deadline interrupts a stalled request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done() // stall until the client gives up
		}))
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		client := newTestGitlabClient(server.URL)
		emojis, err := client.ListAwardEmojis(ctx, 1, 1)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Nil(t, emojis)
	})

	for _, bad := range []string{"not-a-number", "0", "-1"} {
		t.Run("malformed X-Next-Page "+bad+" is an error, not another request", func(t *testing.T) {
			var calls int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("X-Next-Page", bad)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("[]"))
			}))
			defer server.Close()

			client := newTestGitlabClient(server.URL)
			emojis, err := client.ListAwardEmojis(context.Background(), 1, 1)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), fmt.Sprintf("invalid X-Next-Page header %q", bad))
			assert.Nil(t, emojis)
			assert.Equal(t, 1, calls)
		})
	}
}

// Tests focusing on error behavior in the low-level doGet/get methods.
func TestGitlabClient_Get_ErrorCases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4/error-status":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("Internal Server Error"))
		case "/api/v4/invalid-json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("invalid-json-format"))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := newTestGitlabClient(server.URL)

	t.Run("failed to create request", func(t *testing.T) {
		invalidClient := NewGitlabClient("%41:8080", "dummyToken")
		invalidClient.scheme = "http"

		var target any
		err := invalidClient.get(context.Background(), "test-path", &target)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create request")
	})

	t.Run("failed to execute request - mocked error", func(t *testing.T) {
		clientWithMockErr := NewGitlabClient("valid-url", "dummyToken")
		clientWithMockErr.scheme = "http"
		clientWithMockErr.client = &http.Client{
			Transport: &MockRoundTripper{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					return nil, fmt.Errorf("mocked client error")
				},
			},
		}

		var target any
		err := clientWithMockErr.get(context.Background(), "test-path", &target)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to execute request")
	})

	t.Run("HTTP request failure (invalid URL)", func(t *testing.T) {
		clientInvalidURL := NewGitlabClient("invalid-url", "dummyToken")

		var target any
		err := clientInvalidURL.get(context.Background(), "test-path", &target)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to execute request")
	})

	t.Run("non-successful status code", func(t *testing.T) {
		var target any
		err := client.get(context.Background(), "error-status", &target)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "received non-200 response: 500")
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		var target any
		err := client.get(context.Background(), "invalid-json", &target)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal response")
	})

	t.Run("failed to read response body (200 OK)", func(t *testing.T) {
		readFailClient := NewGitlabClient("valid-url", "dummyToken")
		readFailClient.scheme = "http"
		readFailClient.client = &http.Client{
			Transport: &MockRoundTripper{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       &FaultyReadCloser{FailRead: true},
					}, nil
				},
			},
		}

		var target any
		err := readFailClient.get(context.Background(), "test-path", &target)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mocked read error")
	})

	t.Run("failed to read body (non-200)", func(t *testing.T) {
		readFailClient := NewGitlabClient("valid-url", "dummyToken")
		readFailClient.scheme = "http"
		readFailClient.client = &http.Client{
			Transport: &MockRoundTripper{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusInternalServerError,
						Body:       &FaultyReadCloser{FailRead: true},
						Header:     make(http.Header),
					}, nil
				},
			},
		}

		var target any
		err := readFailClient.get(context.Background(), "test-path", &target)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mocked read error")
	})
}

// Tests for GetFileContent-specific errors.
func TestGitlabClient_GetFileContent_ErrorCases(t *testing.T) {
	testCases := []struct {
		name            string
		status          int
		body            string
		transportErr    error
		wantErrContains string
	}{
		{"File not found (404)", http.StatusNotFound, "file not found", nil, "404"},
		{"Authentication error (403)", http.StatusForbidden, "access denied", nil, "403"},
		{"Network error", 0, "", fmt.Errorf("mocked network error"), "mocked network error"},
		{"Malformed response content", http.StatusOK, "not valid json", nil, "failed to unmarshal"},
		{"Unexpected status code (418)", http.StatusTeapot, "I'm a teapot", nil, "418"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewGitlabClient("valid-url", "dummyToken")
			client.scheme = "http"
			client.client = &http.Client{
				Transport: newMockTransport(tc.status, tc.body, tc.transportErr),
			}
			content, err := client.GetFileContent(context.Background(), 123, "main", "some-file")

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErrContains)
			assert.Empty(t, content)
		})
	}
}

// Tests for invalid Base64 content in GetFileContent.
func TestGitlabClient_GetFileContent_Base64DecodeFailure(t *testing.T) {
	client := &GitlabClient{
		scheme:  "https",
		baseURL: "example.com",
		token:   "test-token",
		client: &http.Client{
			Transport: newMockTransport(http.StatusOK, `{"content":"!!invalid_base64!!"}`, nil),
		},
	}
	_, err := client.GetFileContent(context.Background(), 123, "main", "file.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode base64")
}

// Tests for GetLatestPushTimestamp.
func TestGitlabClient_GetLatestPushTimestamp(t *testing.T) {
	server := mockGitLabServer()
	defer server.Close()

	client := newTestGitlabClient(server.URL)

	t.Run("returns the created_at of the highest version id", func(t *testing.T) {
		// mockGitLabServer lists the newest version second, so a naive
		// "first element" implementation would return the wrong timestamp.
		timestamp, err := client.GetLatestPushTimestamp(context.Background(), 1, 1)
		assert.NoError(t, err)
		assert.Equal(t, time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC), timestamp)
	})

	t.Run("reads the diff versions endpoint, never the commits endpoint", func(t *testing.T) {
		// Commit created_at mirrors the git committer date and can be backdated
		// by the pusher; diff version created_at is a GitLab-side record.
		var requested []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requested = append(requested, r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":7,"created_at":"2024-05-01T10:00:00Z"}]`))
		}))
		defer server.Close()

		client := newTestGitlabClient(server.URL)
		_, err := client.GetLatestPushTimestamp(context.Background(), 42, 9)
		assert.NoError(t, err)
		assert.Equal(t, []string{"/api/v4/projects/42/merge_requests/9/versions"}, requested)
	})

	t.Run("selects by highest id, not by position", func(t *testing.T) {
		// GitLab lists versions newest-first; a positional pick of the last
		// element would return the OLDEST push and re-open the bypass.
		newest := time.Date(2024, 3, 3, 0, 0, 0, 0, time.UTC)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[
				{"id":109,"created_at":"2024-03-01T00:00:00Z"},
				{"id":111,"created_at":"2024-03-03T00:00:00Z"},
				{"id":110,"created_at":"2024-03-02T00:00:00Z"}
			]`))
		}))
		defer server.Close()

		client := newTestGitlabClient(server.URL)
		timestamp, err := client.GetLatestPushTimestamp(context.Background(), 1, 1)
		assert.NoError(t, err)
		assert.Equal(t, newest, timestamp)
	})

	t.Run("scans every page so the newest version is found regardless of ordering", func(t *testing.T) {
		newest := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Query().Get("page") {
			case "1":
				w.Header().Set("X-Next-Page", "2")
				_, _ = w.Write([]byte(`[{"id":1,"created_at":"2024-01-01T00:00:00Z"}]`))
			case "2":
				_, _ = w.Write([]byte(`[{"id":2,"created_at":"2024-06-01T00:00:00Z"}]`))
			default:
				t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
			}
		}))
		defer server.Close()

		client := newTestGitlabClient(server.URL)
		timestamp, err := client.GetLatestPushTimestamp(context.Background(), 1, 1)
		assert.NoError(t, err)
		assert.Equal(t, newest, timestamp)
	})

	t.Run("error when no versions exist", func(t *testing.T) {
		emptyClient := NewGitlabClient("valid-url", "dummyToken")
		emptyClient.client = &http.Client{
			Transport: newMockTransport(http.StatusOK, "[]", nil),
		}
		_, err := emptyClient.GetLatestPushTimestamp(context.Background(), 1, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no diff versions found")
	})

	t.Run("error propagation from API", func(t *testing.T) {
		errorClient := NewGitlabClient("valid-url", "dummyToken")
		errorClient.client = &http.Client{
			Transport: newMockTransport(http.StatusInternalServerError,
				"server error", nil),
		}
		_, err := errorClient.GetLatestPushTimestamp(context.Background(), 1, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("error when the newest version has no created_at", func(t *testing.T) {
		// A zero timestamp would make every approval look current, so it must
		// be rejected rather than returned.
		// The older version carries a valid timestamp; the guard must inspect
		// the selected newest version, not fall back to an older one.
		noTimeClient := NewGitlabClient("valid-url", "dummyToken")
		noTimeClient.client = &http.Client{
			Transport: newMockTransport(http.StatusOK,
				`[{"id":4,"created_at":"2024-01-01T00:00:00Z"},{"id":9}]`, nil),
		}
		_, err := noTimeClient.GetLatestPushTimestamp(context.Background(), 1, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "diff version 9")
		assert.Contains(t, err.Error(), "has no created_at")
	})

	t.Run("malformed version timestamp", func(t *testing.T) {
		badTimeClient := NewGitlabClient("valid-url", "dummyToken")
		badTimeClient.client = &http.Client{
			Transport: newMockTransport(http.StatusOK,
				`[{"id":1,"created_at":"invalid-time"}]`, nil),
		}
		_, err := badTimeClient.GetLatestPushTimestamp(context.Background(), 1, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal")
	})
}

// TestGitlabClient_Get_BodyCloseErrorLogging verifies that body close errors are logged via slog.
func TestGitlabClient_Get_BodyCloseErrorLogging(t *testing.T) {
	mockTransport := &MockRoundTripper{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: &FaultyReadCloser{
					FailRead:  false,
					FailClose: true,
				},
				Header: make(http.Header),
			}, nil
		},
	}

	// Capture slog output via a custom handler.
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, nil)
	original := slog.Default()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(original)

	client := NewGitlabClient("valid-url", "dummyToken")
	client.scheme = "http"
	client.client = &http.Client{Transport: mockTransport}

	var target map[string]any
	err := client.get(context.Background(), "test-path", &target)

	assert.NoError(t, err, "The main operation should succeed despite the close error")
	assert.Contains(t, logBuf.String(), "Failed to close response body", "Log should contain the close failure message")
	assert.Contains(t, logBuf.String(), "mocked close error", "Log should include the specific close error")
}
