package main

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/shini4i/atlantis-emoji-gate/internal/client"
	"github.com/shini4i/atlantis-emoji-gate/internal/config"
	"github.com/shini4i/atlantis-emoji-gate/internal/processor"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// -------------------- Mock Implementations --------------------

// MockCodeOwnersProcessor is a mock implementation of CodeOwnersProcessorInterface
type MockCodeOwnersProcessor struct {
	mock.Mock
}

func (m *MockCodeOwnersProcessor) ParseCodeOwners(reader io.Reader) ([]processor.CodeOwner, error) {
	args := m.Called(reader)
	// Handle nil return for []CodeOwner to avoid panic
	var owners []processor.CodeOwner
	if tmp := args.Get(0); tmp != nil {
		owners = tmp.([]processor.CodeOwner)
	}
	return owners, args.Error(1)
}

func (m *MockCodeOwnersProcessor) CanApprove(owner processor.CodeOwner, reaction *client.AwardEmoji, cfg config.GitlabConfig) bool {
	args := m.Called(owner, reaction, cfg)
	return args.Bool(0)
}

// MockGitlabClient is a mock implementation of GitlabClientInterface
type MockGitlabClient struct {
	mock.Mock
}

func (m *MockGitlabClient) GetProject(repo string) (*client.Project, error) {
	args := m.Called(repo)
	// Handle nil return for *Project to avoid panic
	var project *client.Project
	if tmp := args.Get(0); tmp != nil {
		project = tmp.(*client.Project)
	}
	return project, args.Error(1)
}

func (m *MockGitlabClient) GetFileContent(projectID int, branch string, path string) (string, error) {
	args := m.Called(projectID, branch, path)
	return args.String(0), args.Error(1)
}

func (m *MockGitlabClient) ListAwardEmojis(projectID int, pullRequestID int) ([]*client.AwardEmoji, error) {
	args := m.Called(projectID, pullRequestID)
	// Handle nil return for []*AwardEmoji to avoid panic
	var emojis []*client.AwardEmoji
	if tmp := args.Get(0); tmp != nil {
		emojis = tmp.([]*client.AwardEmoji)
	}
	return emojis, args.Error(1)
}

func (m *MockGitlabClient) GetLatestCommitTimestamp(projectID int, mrID int) (time.Time, error) {
	args := m.Called(projectID, mrID)
	// Return the mocked time value
	return args.Get(0).(time.Time), args.Error(1)
}

func (m *MockGitlabClient) GetMrCommits(projectID int, mrID int) ([]*client.Commit, error) {
	args := m.Called(projectID, mrID)
	commits := args.Get(0).([]*client.Commit)
	return commits, args.Error(1)
}

// -------------------- Test Cases --------------------

// TestFetchCodeOwnersContent_WithCodeOwnersRepo_Success tests fetching CODEOWNERS content from a specified CodeOwnersRepo.
func TestFetchCodeOwnersContent_WithCodeOwnersRepo_Success(t *testing.T) {
	// Initialize mocks
	mockClient := new(MockGitlabClient)
	cfg := config.GitlabConfig{
		CodeOwnersRepo: "codeowners/repo",
		CodeOwnersPath: "/path/to/CODEOWNERS",
	}
	project := &client.Project{
		ID:            1,
		DefaultBranch: "main",
	}

	// Mocked repository data
	codeOwnersRepo := &client.Project{
		ID:            2,
		DefaultBranch: "develop",
	}

	// Expected file content
	expectedContent := "owner1 @user1"

	// Setup expectations
	mockClient.On("GetProject", cfg.CodeOwnersRepo).Return(codeOwnersRepo, nil)
	mockClient.On("GetFileContent", codeOwnersRepo.ID, codeOwnersRepo.DefaultBranch, cfg.CodeOwnersPath).Return(expectedContent, nil)

	// Call the function
	content, err := fetchCodeOwnersContent(mockClient, cfg, project)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, expectedContent, content)

	// Ensure that all expectations were met
	mockClient.AssertExpectations(t)
}

// TestFetchCodeOwnersContent_WithoutCodeOwnersRepo_Success tests fetching CODEOWNERS content from the base project when CodeOwnersRepo is empty.
func TestFetchCodeOwnersContent_WithoutCodeOwnersRepo_Success(t *testing.T) {
	// Initialize mocks
	mockClient := new(MockGitlabClient)
	cfg := config.GitlabConfig{
		CodeOwnersRepo: "",
		CodeOwnersPath: "/path/to/CODEOWNERS",
	}
	project := &client.Project{
		ID:            1,
		DefaultBranch: "main",
	}

	// Expected file content
	expectedContent := "owner2 @user2"

	// Setup expectations
	mockClient.On("GetFileContent", project.ID, project.DefaultBranch, cfg.CodeOwnersPath).Return(expectedContent, nil)

	// Call the function
	content, err := fetchCodeOwnersContent(mockClient, cfg, project)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, expectedContent, content)

	// Ensure that all expectations were met
	mockClient.AssertExpectations(t)
}

// TestFetchCodeOwnersContent_GetProjectError tests error handling when GetProject fails.
func TestFetchCodeOwnersContent_GetProjectError(t *testing.T) {
	// Initialize mocks
	mockClient := new(MockGitlabClient)
	cfg := config.GitlabConfig{
		CodeOwnersRepo: "invalid/repo",
		CodeOwnersPath: "/path/to/CODEOWNERS",
	}
	project := &client.Project{
		ID:            1,
		DefaultBranch: "main",
	}

	// Setup expectations
	mockClient.On("GetProject", cfg.CodeOwnersRepo).Return(nil, fmt.Errorf("project not found"))

	// Call the function
	content, err := fetchCodeOwnersContent(mockClient, cfg, project)

	// Assertions
	assert.Error(t, err)
	assert.Equal(t, "", content)
	assert.Contains(t, err.Error(), "failed to get codeowners project")

	mockClient.AssertExpectations(t)
}

// TestFetchCodeOwnersContent_GetFileContentError tests error handling when GetFileContent fails.
func TestFetchCodeOwnersContent_GetFileContentError(t *testing.T) {
	// Initialize mocks
	mockClient := new(MockGitlabClient)
	cfg := config.GitlabConfig{
		CodeOwnersRepo: "codeowners/repo",
		CodeOwnersPath: "/path/to/CODEOWNERS",
	}
	project := &client.Project{
		ID:            1,
		DefaultBranch: "main",
	}

	// Mocked repository data
	codeOwnersRepo := &client.Project{
		ID:            2,
		DefaultBranch: "develop",
	}

	// Setup expectations
	mockClient.On("GetProject", cfg.CodeOwnersRepo).Return(codeOwnersRepo, nil)
	mockClient.On("GetFileContent", codeOwnersRepo.ID, codeOwnersRepo.DefaultBranch, cfg.CodeOwnersPath).Return("", fmt.Errorf("file not found"))

	// Call the function
	content, err := fetchCodeOwnersContent(mockClient, cfg, project)

	// Assertions
	assert.Error(t, err)
	assert.Equal(t, "", content)
	assert.Contains(t, err.Error(), "file not found")

	// Ensure that all expectations were met
	mockClient.AssertExpectations(t)
}

// TestCheckMandatoryApproval_WithApprovals tests CheckMandatoryApproval when approvals are found.
func TestCheckMandatoryApproval_WithApprovals(t *testing.T) {
	// Initialize mocks
	mockClient := new(MockGitlabClient)
	mockProcessor := new(MockCodeOwnersProcessor)

	// Sample data
	projectID := 1
	cfg := config.GitlabConfig{
		PullRequestID: 123,
	}
	codeOwnersContent := "sample content"

	owners := []processor.CodeOwner{
		{Owner: "owner1"},
		{Owner: "owner2"},
	}

	reactions := []*client.AwardEmoji{
		{User: client.User{Username: "user1"}},
		{User: client.User{Username: "user2"}},
	}

	// Setup expectations
	mockProcessor.On("ParseCodeOwners", mock.Anything).Return(owners, nil)
	mockClient.On("ListAwardEmojis", projectID, cfg.PullRequestID).Return(reactions, nil)
	// Setup expectations for all combinations
	mockProcessor.On("CanApprove", owners[0], reactions[0], cfg).Return(true)
	mockProcessor.On("CanApprove", owners[0], reactions[1], cfg).Return(false)
	mockProcessor.On("CanApprove", owners[1], reactions[0], cfg).Return(false)
	mockProcessor.On("CanApprove", owners[1], reactions[1], cfg).Return(true)

	// Call the function
	approved, err := CheckMandatoryApproval(mockClient, cfg, projectID, codeOwnersContent, mockProcessor)

	// Assertions
	assert.NoError(t, err)
	assert.True(t, approved)

	// Ensure that all expectations were met
	mockProcessor.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}

// TestCheckMandatoryApproval_WithoutApprovals tests CheckMandatoryApproval when no approvals are found.
func TestCheckMandatoryApproval_WithoutApprovals(t *testing.T) {
	// Initialize mocks
	mockClient := new(MockGitlabClient)
	mockProcessor := new(MockCodeOwnersProcessor)

	// Sample data
	projectID := 1
	cfg := config.GitlabConfig{
		PullRequestID: 123,
	}
	codeOwnersContent := "sample content"

	owners := []processor.CodeOwner{
		{Owner: "owner1"},
	}

	reactions := []*client.AwardEmoji{
		{User: client.User{Username: "user1"}},
	}

	// Setup expectations
	mockProcessor.On("ParseCodeOwners", mock.Anything).Return(owners, nil)
	mockClient.On("ListAwardEmojis", projectID, cfg.PullRequestID).Return(reactions, nil)
	mockProcessor.On("CanApprove", owners[0], reactions[0], cfg).Return(false)

	// Call the function
	approved, err := CheckMandatoryApproval(mockClient, cfg, projectID, codeOwnersContent, mockProcessor)

	// Assertions
	assert.NoError(t, err)
	assert.False(t, approved)

	// Ensure that all expectations were met
	mockProcessor.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}

// TestCheckMandatoryApproval_ParseError tests CheckMandatoryApproval when ParseCodeOwners returns an error.
func TestCheckMandatoryApproval_ParseError(t *testing.T) {
	// Initialize mocks
	mockClient := new(MockGitlabClient)
	mockProcessor := new(MockCodeOwnersProcessor)

	// Sample data
	projectID := 1
	cfg := config.GitlabConfig{
		PullRequestID: 123,
	}
	codeOwnersContent := "invalid content"

	// Setup expectations
	mockProcessor.On("ParseCodeOwners", mock.Anything).Return(nil, fmt.Errorf("parse error"))

	// Call the function
	approved, err := CheckMandatoryApproval(mockClient, cfg, projectID, codeOwnersContent, mockProcessor)

	// Assertions
	assert.Error(t, err)
	assert.False(t, approved)
	assert.Contains(t, err.Error(), "failed to parse CODEOWNERS")

	// Ensure that all expectations were met
	mockProcessor.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}

// TestCheckMandatoryApproval_ListAwardEmojisError tests CheckMandatoryApproval when ListAwardEmojis returns an error.
func TestCheckMandatoryApproval_ListAwardEmojisError(t *testing.T) {
	// Initialize mocks
	mockClient := new(MockGitlabClient)
	mockProcessor := new(MockCodeOwnersProcessor)

	// Sample data
	projectID := 1
	cfg := config.GitlabConfig{
		PullRequestID: 123,
	}
	codeOwnersContent := "sample content"

	owners := []processor.CodeOwner{
		{Owner: "owner1"},
	}

	// Setup expectations
	mockProcessor.On("ParseCodeOwners", mock.Anything).Return(owners, nil)
	mockClient.On("ListAwardEmojis", projectID, cfg.PullRequestID).Return(nil, fmt.Errorf("API error"))

	// Call the function
	approved, err := CheckMandatoryApproval(mockClient, cfg, projectID, codeOwnersContent, mockProcessor)

	// Assertions
	assert.Error(t, err)
	assert.False(t, approved)
	assert.Contains(t, err.Error(), "failed to fetch reactions")

	// Ensure that all expectations were met
	mockProcessor.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}

// TestFilterApprovals_WithValidApprovals tests filterApprovals when valid approvals are present.
func TestFilterApprovals_WithValidApprovals(t *testing.T) {
	// Initialize mocks
	mockProcessor := new(MockCodeOwnersProcessor)

	date := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	// Sample data
	owners := []processor.CodeOwner{
		{Owner: "owner1"},
		{Owner: "owner2"},
	}

	reactions := []*client.AwardEmoji{
		{User: client.User{Username: "user1"}},
		{User: client.User{Username: "user2"}},
	}

	cfg := config.GitlabConfig{}

	// Setup expectations for all combinations
	mockProcessor.On("CanApprove", owners[0], reactions[0], cfg).Return(true)
	mockProcessor.On("CanApprove", owners[0], reactions[1], cfg).Return(false)
	mockProcessor.On("CanApprove", owners[1], reactions[0], cfg).Return(false)
	mockProcessor.On("CanApprove", owners[1], reactions[1], cfg).Return(true)

	// Call the function
	approvedBy := filterApprovals(owners, reactions, cfg, date, mockProcessor)

	// Assertions
	expected := []string{"user1", "user2"}
	assert.Equal(t, expected, approvedBy)

	// Ensure that all expectations were met
	mockProcessor.AssertExpectations(t)
}

// TestFilterApprovals_NoValidApprovals tests filterApprovals when no valid approvals are present.
func TestFilterApprovals_NoValidApprovals(t *testing.T) {
	// Initialize mocks
	mockProcessor := new(MockCodeOwnersProcessor)

	date := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	// Sample data
	owners := []processor.CodeOwner{
		{Owner: "owner1"},
	}

	reactions := []*client.AwardEmoji{
		{User: client.User{Username: "user1"}},
	}

	cfg := config.GitlabConfig{}

	// Setup expectations
	mockProcessor.On("CanApprove", owners[0], reactions[0], cfg).Return(false)

	// Call the function
	approvedBy := filterApprovals(owners, reactions, cfg, date, mockProcessor)

	// Assertions
	var expected []string
	assert.Equal(t, expected, approvedBy)

	// Ensure that all expectations were met
	mockProcessor.AssertExpectations(t)
}

// TestProcessMR_SuccessWithApprovals tests ProcessMR when approvals are found.
func TestProcessMR_SuccessWithApprovals(t *testing.T) {
	// Initialize mocks
	mockClient := new(MockGitlabClient)
	mockProcessor := new(MockCodeOwnersProcessor)

	// Sample data
	cfg := config.GitlabConfig{
		BaseRepoOwner:  "owner",
		BaseRepoName:   "repo",
		CodeOwnersPath: "/path/to/CODEOWNERS",
		PullRequestID:  123,
	}
	project := &client.Project{
		ID:            1,
		DefaultBranch: "main",
	}

	codeOwnersContent := "sample content"

	// Setup expectations for GetProject
	mockClient.On("GetProject", fmt.Sprintf("%s/%s", cfg.BaseRepoOwner, cfg.BaseRepoName)).Return(project, nil)

	// Setup expectations for fetchCodeOwnersContent
	mockClient.On("GetFileContent", project.ID, project.DefaultBranch, cfg.CodeOwnersPath).Return(codeOwnersContent, nil)

	// Setup expectations for CheckMandatoryApproval
	owners := []processor.CodeOwner{
		{Owner: "owner1"},
	}
	reactions := []*client.AwardEmoji{
		{User: client.User{Username: "user1"}},
	}

	mockProcessor.On("ParseCodeOwners", mock.Anything).Return(owners, nil)
	mockClient.On("ListAwardEmojis", project.ID, cfg.PullRequestID).Return(reactions, nil)
	mockProcessor.On("CanApprove", owners[0], reactions[0], cfg).Return(true)

	// Call the function
	approved, err := ProcessMR(mockClient, cfg, mockProcessor)

	// Assertions
	assert.NoError(t, err)
	assert.True(t, approved)

	// Ensure that all expectations were met
	mockProcessor.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}

// TestProcessMR_SuccessWithoutApprovals tests ProcessMR when no approvals are found.
func TestProcessMR_SuccessWithoutApprovals(t *testing.T) {
	// Initialize mocks
	mockClient := new(MockGitlabClient)
	mockProcessor := new(MockCodeOwnersProcessor)

	// Sample data
	cfg := config.GitlabConfig{
		BaseRepoOwner:  "owner",
		BaseRepoName:   "repo",
		CodeOwnersPath: "/path/to/CODEOWNERS",
		PullRequestID:  123,
	}
	project := &client.Project{
		ID:            1,
		DefaultBranch: "main",
	}

	codeOwnersContent := "sample content"

	// Setup expectations for GetProject
	mockClient.On("GetProject", fmt.Sprintf("%s/%s", cfg.BaseRepoOwner, cfg.BaseRepoName)).Return(project, nil)

	// Setup expectations for fetchCodeOwnersContent
	mockClient.On("GetFileContent", project.ID, project.DefaultBranch, cfg.CodeOwnersPath).Return(codeOwnersContent, nil)

	// Setup expectations for CheckMandatoryApproval
	owners := []processor.CodeOwner{
		{Owner: "owner1"},
	}
	reactions := []*client.AwardEmoji{
		{User: client.User{Username: "user1"}},
	}

	mockProcessor.On("ParseCodeOwners", mock.Anything).Return(owners, nil)
	mockClient.On("ListAwardEmojis", project.ID, cfg.PullRequestID).Return(reactions, nil)
	mockProcessor.On("CanApprove", owners[0], reactions[0], cfg).Return(false)

	// Call the function
	approved, err := ProcessMR(mockClient, cfg, mockProcessor)

	// Assertions
	assert.NoError(t, err)
	assert.False(t, approved)

	// Ensure that all expectations were met
	mockProcessor.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}

// TestProcessMR_GetProjectError tests ProcessMR when GetProject returns an error.
func TestProcessMR_GetProjectError(t *testing.T) {
	// Initialize mocks
	mockClient := new(MockGitlabClient)
	mockProcessor := new(MockCodeOwnersProcessor)

	// Sample data
	cfg := config.GitlabConfig{
		BaseRepoOwner:  "owner",
		BaseRepoName:   "repo",
		CodeOwnersPath: "/path/to/CODEOWNERS",
		PullRequestID:  123,
	}

	// Setup expectations for GetProject
	mockClient.On("GetProject", fmt.Sprintf("%s/%s", cfg.BaseRepoOwner, cfg.BaseRepoName)).Return(nil, fmt.Errorf("project not found"))

	// Call the function
	approved, err := ProcessMR(mockClient, cfg, mockProcessor)

	// Assertions
	assert.Error(t, err)
	assert.False(t, approved)
	assert.Contains(t, err.Error(), "failed to get project")

	// Ensure that all expectations were met
	mockClient.AssertExpectations(t)
	mockProcessor.AssertExpectations(t)
}

// TestProcessMR_FetchCodeOwnersError tests ProcessMR when fetchCodeOwnersContent returns an error.
func TestProcessMR_FetchCodeOwnersError(t *testing.T) {
	// Initialize mocks
	mockClient := new(MockGitlabClient)
	mockProcessor := new(MockCodeOwnersProcessor)

	// Sample data
	cfg := config.GitlabConfig{
		BaseRepoOwner:  "owner",
		BaseRepoName:   "repo",
		CodeOwnersPath: "/path/to/CODEOWNERS",
		PullRequestID:  123,
	}
	project := &client.Project{
		ID:            1,
		DefaultBranch: "main",
	}

	// Setup expectations for GetProject
	mockClient.On("GetProject", fmt.Sprintf("%s/%s", cfg.BaseRepoOwner, cfg.BaseRepoName)).Return(project, nil)

	// Setup expectations for fetchCodeOwnersContent
	mockClient.On("GetFileContent", project.ID, project.DefaultBranch, cfg.CodeOwnersPath).Return("", fmt.Errorf("file not found"))

	// Call the function
	approved, err := ProcessMR(mockClient, cfg, mockProcessor)

	// Assertions
	assert.Error(t, err)
	assert.False(t, approved)
	assert.Contains(t, err.Error(), "failed to fetch CODEOWNERS file")
	// If you wrapped the error, also check for the original error
	// assert.Contains(t, err.Error(), "file not found")

	// Ensure that all expectations were met
	mockClient.AssertExpectations(t)
	mockProcessor.AssertExpectations(t)
}

// TestRun_SuccessApproved tests Run function when approval is successful.
func TestRun_SuccessApproved(t *testing.T) {
	// Initialize mocks
	mockClient := new(MockGitlabClient)
	mockProcessor := new(MockCodeOwnersProcessor)

	// Sample data
	cfg := config.GitlabConfig{
		Insecure:       false,
		BaseRepoOwner:  "owner",
		BaseRepoName:   "repo",
		CodeOwnersPath: "/path/to/CODEOWNERS",
		PullRequestID:  123,
	}
	project := &client.Project{
		ID:            1,
		DefaultBranch: "main",
	}
	codeOwnersContent := "sample content"

	// Setup expectations for GetProject
	mockClient.On("GetProject", fmt.Sprintf("%s/%s", cfg.BaseRepoOwner, cfg.BaseRepoName)).Return(project, nil)

	// Setup expectations for fetchCodeOwnersContent
	mockClient.On("GetFileContent", project.ID, project.DefaultBranch, cfg.CodeOwnersPath).Return(codeOwnersContent, nil)

	// Setup expectations for CheckMandatoryApproval
	owners := []processor.CodeOwner{
		{Owner: "owner1"},
	}
	reactions := []*client.AwardEmoji{
		{User: client.User{Username: "user1"}},
	}

	mockProcessor.On("ParseCodeOwners", mock.Anything).Return(owners, nil)
	mockClient.On("ListAwardEmojis", project.ID, cfg.PullRequestID).Return(reactions, nil)
	mockProcessor.On("CanApprove", owners[0], reactions[0], cfg).Return(true)

	// Call the Run function
	exitCode := Run(mockClient, cfg, mockProcessor)

	// Assertions
	assert.Equal(t, 0, exitCode)

	// Ensure that all expectations were met
	mockProcessor.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}

// TestRun_SuccessNotApproved tests Run function when no approval is found.
func TestRun_SuccessNotApproved(t *testing.T) {
	// Initialize mocks
	mockClient := new(MockGitlabClient)
	mockProcessor := new(MockCodeOwnersProcessor)

	// Sample data
	cfg := config.GitlabConfig{
		Insecure:       false,
		BaseRepoOwner:  "owner",
		BaseRepoName:   "repo",
		CodeOwnersPath: "/path/to/CODEOWNERS",
		PullRequestID:  123,
	}
	project := &client.Project{
		ID:            1,
		DefaultBranch: "main",
	}
	codeOwnersContent := "sample content"

	// Setup expectations for GetProject
	mockClient.On("GetProject", fmt.Sprintf("%s/%s", cfg.BaseRepoOwner, cfg.BaseRepoName)).Return(project, nil)

	// Setup expectations for fetchCodeOwnersContent
	mockClient.On("GetFileContent", project.ID, project.DefaultBranch, cfg.CodeOwnersPath).Return(codeOwnersContent, nil)

	// Setup expectations for CheckMandatoryApproval
	owners := []processor.CodeOwner{
		{Owner: "owner1"},
	}
	reactions := []*client.AwardEmoji{
		{User: client.User{Username: "user1"}},
	}

	mockProcessor.On("ParseCodeOwners", mock.Anything).Return(owners, nil)
	mockClient.On("ListAwardEmojis", project.ID, cfg.PullRequestID).Return(reactions, nil)
	mockProcessor.On("CanApprove", owners[0], reactions[0], cfg).Return(false)

	// Call the Run function
	exitCode := Run(mockClient, cfg, mockProcessor)

	// Assertions
	assert.Equal(t, 1, exitCode)

	// Ensure that all expectations were met
	mockProcessor.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}

// TestRun_ErrorProcessingMR tests Run function when ProcessMR returns an error.
func TestRun_ErrorProcessingMR(t *testing.T) {
	// Initialize mocks
	mockClient := new(MockGitlabClient)
	mockProcessor := new(MockCodeOwnersProcessor)

	// Sample data
	cfg := config.GitlabConfig{
		Insecure:       false,
		BaseRepoOwner:  "owner",
		BaseRepoName:   "repo",
		CodeOwnersPath: "/path/to/CODEOWNERS",
		PullRequestID:  123,
	}

	// Setup expectations for GetProject
	mockClient.On("GetProject", fmt.Sprintf("%s/%s", cfg.BaseRepoOwner, cfg.BaseRepoName)).Return(nil, fmt.Errorf("project not found"))

	// Call the Run function
	exitCode := Run(mockClient, cfg, mockProcessor)

	// Assertions
	assert.Equal(t, 1, exitCode)

	// Ensure that all expectations were met
	mockClient.AssertExpectations(t)
	mockProcessor.AssertExpectations(t)
}

func TestRun_InsecureMode(t *testing.T) {
	tests := []struct {
		name              string
		insecure          bool
		expectContains    []string
		expectNotContains []string
		expectedExit      int
	}{
		{
			name:     "Insecure mode enabled",
			insecure: true,
			expectContains: []string{
				"Insecure mode enabled: MR author can approve their own MR if they are in CODEOWNERS",
				"Mandatory approval provided",
			},
			expectNotContains: []string{
				"MR author 'author1' cannot approve their own MR",
			},
			expectedExit: 0,
		},
		{
			name:     "Insecure mode disabled",
			insecure: false,
			expectContains: []string{
				"MR author 'author1' cannot approve their own MR",
				"Mandatory approval not found",
			},
			expectNotContains: []string{
				"Insecure mode enabled: MR author can approve their own MR if they are in CODEOWNERS",
				"Mandatory approval provided",
			},
			expectedExit: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockClient := new(MockGitlabClient)
			processor := &processor.CodeOwnersProcessor{}

			cfg := config.GitlabConfig{
				Insecure:       test.insecure,
				MrAuthor:       "author1",
				ApproveEmoji:   ":+1:",
				TerraformPath:  "/path/to/terraform/module",
				BaseRepoOwner:  "owner",
				BaseRepoName:   "repo",
				PullRequestID:  123,
				CodeOwnersPath: "/path/to/CODEOWNERS",
			}

			// Setup expectations for GetProject
			mockClient.On("GetProject", fmt.Sprintf("%s/%s", cfg.BaseRepoOwner, cfg.BaseRepoName)).Return(&client.Project{ID: 1, DefaultBranch: "main"}, nil)

			// Setup expectations for GetFileContent
			codeOwnersContent := "/path/to/terraform @author1"
			mockClient.On("GetFileContent", 1, "main", cfg.CodeOwnersPath).Return(codeOwnersContent, nil)

			// Setup expectations for ListAwardEmojis
			reactions := []*client.AwardEmoji{
				{Name: ":+1:", User: client.User{Username: "author1"}},
			}
			mockClient.On("ListAwardEmojis", 1, cfg.PullRequestID).Return(reactions, nil)

			// Capture the output from fmt.Println and fmt.Printf
			// Save original stdout
			origStdout := os.Stdout

			// Create a pipe to capture output
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Call Run
			exitCode := Run(mockClient, cfg, processor)

			// Close the writer and restore stdout
			w.Close()
			os.Stdout = origStdout

			// Read the captured output
			outputBytes, _ := io.ReadAll(r)
			output := string(outputBytes)

			// Validate exit code
			assert.Equal(t, test.expectedExit, exitCode)

			// Validate printed messages
			for _, msg := range test.expectContains {
				assert.Contains(t, output, msg)
			}
			for _, msg := range test.expectNotContains {
				assert.NotContains(t, output, msg)
			}

			// Ensure that all expectations were met
			mockClient.AssertExpectations(t)
		})
	}
}

// TestCheckMandatoryApproval_WithValidAndExpiredApprovals tests CheckMandatoryApproval with both valid and expired approvals.
// TestCheckMandatoryApproval_WithValidAndExpiredApprovals tests CheckMandatoryApproval with both valid and expired approvals.
func TestCheckMandatoryApproval_WithValidAndExpiredApprovals(t *testing.T) {
	// Initialize mocks
	mockClient := new(MockGitlabClient)
	mockProcessor := new(MockCodeOwnersProcessor)

	// Sample data
	projectID := 1
	cfg := config.GitlabConfig{
		PullRequestID: 123,
		Restricted:    true,
	}
	codeOwnersContent := "sample content"

	owners := []processor.CodeOwner{
		{Owner: "owner1"},
	}

	// Define timestamps
	latestCommitTimestamp := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	validReactionTimestamp := latestCommitTimestamp.Add(1 * time.Hour)
	expiredReactionTimestamp := latestCommitTimestamp.Add(-1 * time.Hour)

	reactions := []*client.AwardEmoji{
		{User: client.User{Username: "owner1"}, UpdatedAt: validReactionTimestamp},
		{User: client.User{Username: "owner2"}, UpdatedAt: expiredReactionTimestamp},
	}

	// Setup expectations
	mockProcessor.On("ParseCodeOwners", mock.Anything).Return(owners, nil)
	mockClient.On("ListAwardEmojis", projectID, cfg.PullRequestID).Return(reactions, nil)
	mockClient.On("GetLatestCommitTimestamp", projectID, cfg.PullRequestID).Return(latestCommitTimestamp, nil)
	mockProcessor.On("CanApprove", owners[0], reactions[0], cfg).Return(true)
	mockProcessor.On("CanApprove", owners[0], reactions[1], cfg).Return(false)

	// Call the function
	approved, err := CheckMandatoryApproval(mockClient, cfg, projectID, codeOwnersContent, mockProcessor)

	// Assertions
	assert.NoError(t, err)
	assert.True(t, approved)

	// Ensure that all expectations were met
	mockProcessor.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}
