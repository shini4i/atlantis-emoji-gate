package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ------------------ Mock Types and Helpers ------------------

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

func (f *FaultyReadCloser) Read(p []byte) (n int, err error) {
	if f.FailRead {
		return 0, fmt.Errorf("mocked read error")
	}
	copy(p, `{"key": "value"}`)
	return len(`{"key": "value"}`), io.EOF
}

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
			{Name: "thumbsup", User: struct {
				Username string `json:"username"`
			}{Username: "user1"}},
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

	return httptest.NewServer(mux)
}

// Helper to create a test GitLab client pointing to the mock server.
func newTestGitlabClient(baseURL string) *GitlabClient {
	// 'baseURL[7:]' strips "http://" from the server URL.
	client := NewGitlabClient(baseURL[7:], "dummyToken")
	client.Scheme = "http"
	return client
}

// ------------------ Actual Tests ------------------

func TestGitlabClient_SuccessCases(t *testing.T) {
	server := mockGitLabServer()
	defer server.Close()

	client := newTestGitlabClient(server.URL)

	t.Run("GetProject", func(t *testing.T) {
		project, err := client.GetProject("mockProjectPath")
		assert.NoError(t, err)
		assert.Equal(t, 1, project.ID)
		assert.Equal(t, "main", project.DefaultBranch)
	})

	t.Run("ListAwardEmojis", func(t *testing.T) {
		emojis, err := client.ListAwardEmojis(1, 1)
		assert.NoError(t, err)
		assert.Len(t, emojis, 1)
		assert.Equal(t, "thumbsup", emojis[0].Name)
		assert.Equal(t, "user1", emojis[0].User.Username)
	})

	t.Run("GetFileContent", func(t *testing.T) {
		content, err := client.GetFileContent(1, "main", "CODEOWNERS")
		assert.NoError(t, err)
		assert.Equal(t, "* @user1\n", content)
	})
}

// Tests focusing on error behavior in the low-level `get()` method.
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
		// Invalid URL
		invalidClient := NewGitlabClient("%41:8080", "dummyToken")
		invalidClient.Scheme = "http"

		var target interface{}
		err := invalidClient.get("test-path", &target)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create request")
	})

	t.Run("failed to execute request - mocked error", func(t *testing.T) {
		clientWithMockErr := NewGitlabClient("valid-url", "dummyToken")
		clientWithMockErr.Scheme = "http"
		clientWithMockErr.client = &http.Client{
			Transport: &MockRoundTripper{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					return nil, fmt.Errorf("mocked client error")
				},
			},
		}

		var target interface{}
		err := clientWithMockErr.get("test-path", &target)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to execute request")
	})

	t.Run("HTTP request failure (invalid URL)", func(t *testing.T) {
		clientInvalidURL := NewGitlabClient("invalid-url", "dummyToken")

		var target interface{}
		err := clientInvalidURL.get("test-path", &target)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to execute request")
	})

	t.Run("non-successful status code", func(t *testing.T) {
		var target interface{}
		err := client.get("error-status", &target)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "received non-200 response: 500")
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		var target interface{}
		err := client.get("invalid-json", &target)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal response")
	})

	t.Run("failed to read response body (200 OK)", func(t *testing.T) {
		readFailClient := NewGitlabClient("valid-url", "dummyToken")
		readFailClient.Scheme = "http"
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

		var target interface{}
		err := readFailClient.get("test-path", &target)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mocked read error")
	})

	t.Run("failed to read body (non-200)", func(t *testing.T) {
		readFailClient := NewGitlabClient("valid-url", "dummyToken")
		readFailClient.Scheme = "http"
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

		var target interface{}
		err := readFailClient.get("test-path", &target)
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
			client.Scheme = "http"
			client.client = &http.Client{
				Transport: newMockTransport(tc.status, tc.body, tc.transportErr),
			}
			content, err := client.GetFileContent(123, "main", "some-file")

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErrContains)
			assert.Empty(t, content)
		})
	}
}

// Tests for invalid Base64 content in GetFileContent.
func TestGitlabClient_GetFileContent_Base64DecodeFailure(t *testing.T) {
	client := &GitlabClient{
		Scheme:  "https",
		BaseURL: "example.com",
		Token:   "test-token",
		client: &http.Client{
			Transport: newMockTransport(http.StatusOK, `{"content":"!!invalid_base64!!"}`, nil),
		},
	}
	_, err := client.GetFileContent(123, "main", "file.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode base64")
}
