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

type MockRoundTripper struct {
	RoundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.RoundTripFunc(req) // Calls the mock function
}

type FaultyReadCloser struct {
	FailRead  bool
	FailClose bool
}

func (f *FaultyReadCloser) Read(p []byte) (n int, err error) {
	if f.FailRead {
		return 0, fmt.Errorf("mocked read error")
	}
	content := `{"key": "value"}`
	copy(p, content)
	return len(content), io.EOF
}

func (f *FaultyReadCloser) Close() error {
	if f.FailClose {
		return fmt.Errorf("mocked close error")
	}
	return nil
}

// mockGitLabServer sets up a mock GitLab API server with predefined responses.
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
		queryParams := r.URL.Query()
		if queryParams.Get("ref") != "main" {
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

func TestGitlabClient_GetProject(t *testing.T) {
	server := mockGitLabServer()
	defer server.Close()

	client := NewGitlabClient(server.URL[7:], "dummyToken")
	client.Scheme = "http" // Use HTTP scheme for the test server

	project, err := client.GetProject("mockProjectPath")
	assert.NoError(t, err)
	assert.Equal(t, 1, project.ID)
	assert.Equal(t, "main", project.DefaultBranch)
}

func TestGitlabClient_ListAwardEmojis(t *testing.T) {
	server := mockGitLabServer()
	defer server.Close()

	client := NewGitlabClient(server.URL[7:], "dummyToken")
	client.Scheme = "http" // Use HTTP scheme for the test server

	emojis, err := client.ListAwardEmojis(1, 1)
	assert.NoError(t, err)
	assert.Len(t, emojis, 1)
	assert.Equal(t, "thumbsup", emojis[0].Name)
	assert.Equal(t, "user1", emojis[0].User.Username)
}

func TestGitlabClient_GetFileContent(t *testing.T) {
	server := mockGitLabServer()
	defer server.Close()

	client := NewGitlabClient(server.URL[7:], "dummyToken")
	client.Scheme = "http" // Use HTTP scheme for the test server

	content, err := client.GetFileContent(1, "main", "CODEOWNERS")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expectedContent := "* @user1\n"
	assert.Equal(t, expectedContent, content)
}

func TestGitlabClient_Get_ErrorCases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("DEBUG: Received request on path: %s\n", r.URL.Path) // Log request path

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

	client := NewGitlabClient(server.URL[7:], "dummyToken")
	client.Scheme = "http"

	// Test case 1: Failed to create request (invalid URL)
	t.Run("failed to create request", func(t *testing.T) {
		client := NewGitlabClient("%41:8080", "dummyToken")
		client.Scheme = "http"

		var target interface{}
		err := client.get("test-path", &target)
		assert.Error(t, err, "Expected an error when the request creation fails")
		assert.Contains(t, err.Error(), "failed to create request", "Expected error to mention failed request creation")
	})

	// Test case 2: HTTP client fails to execute the request (simulated error) If this happens in the real world, it's likely due to network issues.
	t.Run("failed to execute request", func(t *testing.T) {
		// Create a mock HTTP client with a custom RoundTripper
		mockTransport := &MockRoundTripper{
			RoundTripFunc: func(req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("mocked client error")
			},
		}

		client := NewGitlabClient("valid-url", "dummyToken")
		client.Scheme = "http"
		client.client = &http.Client{Transport: mockTransport}

		var target interface{}
		err := client.get("test-path", &target)

		// Assertions
		assert.Error(t, err, "Expected an error when the HTTP client fails")
		assert.Contains(t, err.Error(), "failed to execute request", "Expected error to mention failed request execution")
	})

	// Test case 2: HTTP request fails due to a simulated error
	t.Run("HTTP request failure", func(t *testing.T) {
		client := NewGitlabClient("invalid-url", "dummyToken")
		var target interface{}
		err := client.get("/invalid-path", &target)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to execute request", "Expected an error due to invalid URL")
	})

	// Test case 3: Server returns a non-2xx status code
	t.Run("Non-successful status code", func(t *testing.T) {
		var target interface{}
		err := client.get("error-status", &target)
		assert.Error(t, err, "Expected an error for non-2xx status code")
		assert.Contains(t, err.Error(), "received non-200 response: 500 - Internal Server Error", "Expected error to include status code and message")
	})

	// Test case 4: Invalid JSON in response body
	t.Run("Invalid JSON response", func(t *testing.T) {
		var target interface{}
		err := client.get("invalid-json", &target)
		assert.Error(t, err, "Expected an error for invalid JSON response")
		assert.Contains(t, err.Error(), "failed to unmarshal response", "Expected error to mention unmarshalling failure")
	})

	// Test case 5: Failed to read response body
	t.Run("failed to read response body", func(t *testing.T) {
		// Create a mock HTTP client with a custom RoundTripper
		mockTransport := &MockRoundTripper{
			RoundTripFunc: func(req *http.Request) (*http.Response, error) {
				// Simulate a successful response with a faulty body reader
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       &FaultyReadCloser{FailRead: true},
				}, nil
			},
		}

		client := NewGitlabClient("valid-url", "dummyToken")
		client.Scheme = "http"
		client.client = &http.Client{Transport: mockTransport}

		var target interface{}
		err := client.get("test-path", &target)

		// Assertions
		assert.Error(t, err, "Expected an error when reading the response body fails")
		assert.Contains(t, err.Error(), "mocked read error", "Expected error to mention failed body read")
	})

	// Test case 6: Failed to read response body in non-200 status code
	t.Run("Failed to read response body in non-200 status code", func(t *testing.T) {
		// Create a mock HTTP client with a custom RoundTripper
		mockTransport := &MockRoundTripper{
			RoundTripFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusInternalServerError, // Simulate non-200 status code
					Body: &FaultyReadCloser{
						FailRead: true, // Simulate a read error
					},
					Header: make(http.Header),
				}, nil
			},
		}

		// Initialize GitLab client
		client := NewGitlabClient("valid-url", "dummyToken")
		client.Scheme = "http"
		client.client = &http.Client{Transport: mockTransport}

		// Execute the `get()` function
		var target interface{}
		err := client.get("test-path", &target)

		// Assertions
		assert.Error(t, err, "Expected an error when failing to read the response body")
		assert.Contains(t, err.Error(), "mocked read error", "Expected the error message to indicate the read failure")
	})
}

func TestGitlabClient_GetFileContent_ErrorCases(t *testing.T) {
	t.Run("File not found (404 error)", func(t *testing.T) {
		// Mock HTTP client to simulate 404 response
		mockTransport := &MockRoundTripper{
			RoundTripFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusNotFound, // Simulate 404 Not Found
					Body:       io.NopCloser(strings.NewReader("file not found")),
					Header:     make(http.Header),
				}, nil
			},
		}

		client := NewGitlabClient("valid-url", "dummyToken")
		client.Scheme = "http"
		client.client = &http.Client{Transport: mockTransport}

		// Call GetFileContent with all required arguments
		content, err := client.GetFileContent(123, "main", "nonexistent-file-path")

		// Assertions
		assert.Error(t, err, "Expected an error for 404 response")
		assert.Contains(t, err.Error(), "404", "Expected error message to include 404 status")
		assert.Empty(t, content, "Expected content to be empty on failure")
	})

	t.Run("Authentication error (401/403)", func(t *testing.T) {
		mockTransport := &MockRoundTripper{
			RoundTripFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusForbidden, // Simulate 403 Forbidden
					Body:       io.NopCloser(strings.NewReader("access denied")),
					Header:     make(http.Header),
				}, nil
			},
		}

		client := NewGitlabClient("valid-url", "dummyToken")
		client.Scheme = "http"
		client.client = &http.Client{Transport: mockTransport}

		// Call GetFileContent with all required arguments
		content, err := client.GetFileContent(123, "main", "restricted-file-path")

		// Assertions
		assert.Error(t, err, "Expected an error for 403 response")
		assert.Contains(t, err.Error(), "403", "Expected error message to include 403 status")
		assert.Empty(t, content, "Expected content to be empty on failure")
	})

	t.Run("Network error", func(t *testing.T) {
		mockTransport := &MockRoundTripper{
			RoundTripFunc: func(req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("mocked network error") // Simulate network failure
			},
		}

		client := NewGitlabClient("valid-url", "dummyToken")
		client.Scheme = "http"
		client.client = &http.Client{Transport: mockTransport}

		// Call GetFileContent with all required arguments
		content, err := client.GetFileContent(123, "main", "any-file-path")

		// Assertions
		assert.Error(t, err, "Expected an error for network failure")
		assert.Contains(t, err.Error(), "mocked network error", "Expected error message to include network failure")
		assert.Empty(t, content, "Expected content to be empty on failure")
	})

	t.Run("Malformed response content", func(t *testing.T) {
		mockTransport := &MockRoundTripper{
			RoundTripFunc: func(req *http.Request) (*http.Response, error) {
				// Simulate a valid status but invalid response content
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("not valid json")), // Invalid body
					Header:     make(http.Header),
				}, nil
			},
		}

		client := NewGitlabClient("valid-url", "dummyToken")
		client.Scheme = "http"
		client.client = &http.Client{Transport: mockTransport}

		// Call GetFileContent with all required arguments
		content, err := client.GetFileContent(123, "main", "malformed-content-path")

		// Assertions
		assert.Error(t, err, "Expected an error for malformed content")
		assert.Contains(t, err.Error(), "failed to unmarshal", "Expected error message to mention unmarshaling failure")
		assert.Empty(t, content, "Expected content to be empty on failure")
	})

	t.Run("Unexpected status code", func(t *testing.T) {
		mockTransport := &MockRoundTripper{
			RoundTripFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusTeapot, // Simulate unexpected 418 status
					Body:       io.NopCloser(strings.NewReader("I'm a teapot")),
					Header:     make(http.Header),
				}, nil
			},
		}

		client := NewGitlabClient("valid-url", "dummyToken")
		client.Scheme = "http"
		client.client = &http.Client{Transport: mockTransport}

		// Call GetFileContent with all required arguments
		content, err := client.GetFileContent(123, "main", "weird-status-code-path")

		// Assertions
		assert.Error(t, err, "Expected an error for unexpected status code")
		assert.Contains(t, err.Error(), "418", "Expected error message to include unexpected status code 418")
		assert.Empty(t, content, "Expected content to be empty on failure")
	})
}
