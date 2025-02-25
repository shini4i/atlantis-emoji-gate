package client

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

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

	// Add new endpoint for merge request commits
	mux.HandleFunc("/api/v4/projects/1/merge_requests/1/commits", func(w http.ResponseWriter, r *http.Request) {
		commits := []*Commit{
			{
				ID:        "abc123",
				CreatedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			},
			{
				ID:        "def456",
				CreatedAt: time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC),
			},
		}
		_ = json.NewEncoder(w).Encode(commits)
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

// Tests for GetMrCommits
func TestGitlabClient_GetMrCommits(t *testing.T) {
	server := mockGitLabServer()
	defer server.Close()

	client := newTestGitlabClient(server.URL)

	t.Run("successful commits retrieval", func(t *testing.T) {
		commits, err := client.GetMrCommits(1, 1)
		assert.NoError(t, err)
		assert.Len(t, commits, 2)
		assert.Equal(t, "abc123", commits[0].ID)
		assert.Equal(t,
			time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			commits[0].CreatedAt)
	})

	t.Run("error on invalid project ID", func(t *testing.T) {
		commits, err := client.GetMrCommits(-1, 1)
		assert.Error(t, err)
		assert.Nil(t, commits)
	})

	t.Run("error on non-existent merge request", func(t *testing.T) {
		client := NewGitlabClient("valid-url", "dummyToken")
		client.client = &http.Client{
			Transport: newMockTransport(http.StatusNotFound, "not found", nil),
		}
		commits, err := client.GetMrCommits(1, 999)
		assert.Error(t, err)
		assert.Nil(t, commits)
	})
}

// Tests for GetLatestCommitTimestamp
func TestGitlabClient_GetLatestCommitTimestamp(t *testing.T) {
	server := mockGitLabServer()
	defer server.Close()

	client := newTestGitlabClient(server.URL)

	t.Run("successful timestamp retrieval", func(t *testing.T) {
		timestamp, err := client.GetLatestCommitTimestamp(1, 1)
		assert.NoError(t, err)
		expectedTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		assert.Equal(t, expectedTime, timestamp)
	})

	t.Run("error when no commits exist", func(t *testing.T) {
		emptyClient := NewGitlabClient("valid-url", "dummyToken")
		emptyClient.client = &http.Client{
			Transport: newMockTransport(http.StatusOK, "[]", nil),
		}
		_, err := emptyClient.GetLatestCommitTimestamp(1, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no commits found")
	})

	t.Run("error propagation from GetMrCommits", func(t *testing.T) {
		errorClient := NewGitlabClient("valid-url", "dummyToken")
		errorClient.client = &http.Client{
			Transport: newMockTransport(http.StatusInternalServerError,
				"server error", nil),
		}
		_, err := errorClient.GetLatestCommitTimestamp(1, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("malformed commit timestamp", func(t *testing.T) {
		badTimeClient := NewGitlabClient("valid-url", "dummyToken")
		badTimeClient.client = &http.Client{
			Transport: newMockTransport(http.StatusOK,
				`[{"id":"123","created_at":"invalid-time"}]`, nil),
		}
		_, err := badTimeClient.GetLatestCommitTimestamp(1, 1)
		assert.Error(t, err)
	})
}

func TestGitlabClient_Get_BodyCloseErrorLogging(t *testing.T) {
	// Create a mock HTTP client with a faulty response body
	mockTransport := &MockRoundTripper{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: &FaultyReadCloser{
					FailRead:  false, // No issue while reading
					FailClose: true,  // Fails on Close
				},
				Header: make(http.Header),
			}, nil
		},
	}

	// Capture os.Stdout output by redirecting it temporarily
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w

	var outputCaptured string
	done := make(chan bool)
	go func() {
		var buf bytes.Buffer
		_, err := io.Copy(&buf, r)
		if err != nil {
			t.Error("failed to read from pipe:", err)
		}
		outputCaptured = buf.String()
		done <- true
	}()

	// Initialize the GitLab client using the mock transport
	client := NewGitlabClient("valid-url", "dummyToken")
	client.Scheme = "http"
	client.client = &http.Client{Transport: mockTransport}

	var target map[string]interface{}
	err := client.get("test-path", &target)

	_ = w.Close()
	os.Stdout = stdout
	<-done

	// Assertions to validate behavior
	assert.NoError(t, err, "The main operation should succeed despite the close error")

	// Validate the captured log output
	assert.Contains(t, outputCaptured, "failed to close response body:", "Log should contain the message about closing failure")
	assert.Contains(t, outputCaptured, "mocked close error", "Log should include the specific close error")
}
