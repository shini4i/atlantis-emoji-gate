package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
	client.ProjectID = 1

	emojis, err := client.ListAwardEmojis(1)
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
	client.ProjectID = 1

	content, err := client.GetFileContent("main", "CODEOWNERS")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expectedContent := "* @user1\n"
	assert.Equal(t, expectedContent, content)
}
