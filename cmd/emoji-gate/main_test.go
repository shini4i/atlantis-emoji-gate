package main

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// -------------------- Mock Implementations --------------------

// MockCodeOwnersProcessor is a mock implementation of CodeOwnersProcessorInterface
type MockCodeOwnersProcessor struct {
	mock.Mock
}

func (m *MockCodeOwnersProcessor) ParseCodeOwners(reader io.Reader) ([]CodeOwner, error) {
	args := m.Called(reader)
	// Handle nil return for []CodeOwner to avoid panic
	var owners []CodeOwner
	if tmp := args.Get(0); tmp != nil {
		owners = tmp.([]CodeOwner)
	}
	return owners, args.Error(1)
}

func (m *MockCodeOwnersProcessor) CanApprove(owner CodeOwner, reaction *AwardEmoji, cfg GitlabConfig) bool {
	args := m.Called(owner, reaction, cfg)
	return args.Bool(0)
}

// MockGitlabClient is a mock implementation of GitlabClientInterface
type MockGitlabClient struct {
	mock.Mock
}

func (m *MockGitlabClient) GetProject(repo string) (*Project, error) {
	args := m.Called(repo)
	// Handle nil return for *Project to avoid panic
	var project *Project
	if tmp := args.Get(0); tmp != nil {
		project = tmp.(*Project)
	}
	return project, args.Error(1)
}

func (m *MockGitlabClient) GetFileContent(projectID int, branch string, path string) (string, error) {
	args := m.Called(projectID, branch, path)
	return args.String(0), args.Error(1)
}

func (m *MockGitlabClient) ListAwardEmojis(projectID int, pullRequestID int) ([]*AwardEmoji, error) {
	args := m.Called(projectID, pullRequestID)
	// Handle nil return for []*AwardEmoji to avoid panic
	var emojis []*AwardEmoji
	if tmp := args.Get(0); tmp != nil {
		emojis = tmp.([]*AwardEmoji)
	}
	return emojis, args.Error(1)
}

// -------------------- Test Cases --------------------

// TestFetchCodeOwnersContent_WithCodeOwnersRepo_Success tests fetching CODEOWNERS content from a specified CodeOwnersRepo.
func TestFetchCodeOwnersContent_WithCodeOwnersRepo_Success(t *testing.T) {
	// Initialize mocks
	mockClient := new(MockGitlabClient)
	cfg := GitlabConfig{
		CodeOwnersRepo: "codeowners/repo",
		CodeOwnersPath: "/path/to/CODEOWNERS",
	}
	project := &Project{
		ID:            1,
		DefaultBranch: "main",
	}

	// Mocked repository data
	codeOwnersRepo := &Project{
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
	cfg := GitlabConfig{
		CodeOwnersRepo: "",
		CodeOwnersPath: "/path/to/CODEOWNERS",
	}
	project := &Project{
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
	cfg := GitlabConfig{
		CodeOwnersRepo: "invalid/repo",
		CodeOwnersPath: "/path/to/CODEOWNERS",
	}
	project := &Project{
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
	cfg := GitlabConfig{
		CodeOwnersRepo: "codeowners/repo",
		CodeOwnersPath: "/path/to/CODEOWNERS",
	}
	project := &Project{
		ID:            1,
		DefaultBranch: "main",
	}

	// Mocked repository data
	codeOwnersRepo := &Project{
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
	cfg := GitlabConfig{
		PullRequestID: 123,
	}
	codeOwnersContent := "sample content"

	owners := []CodeOwner{
		{Owner: "owner1"},
		{Owner: "owner2"},
	}

	reactions := []*AwardEmoji{
		{User: User{Username: "user1"}},
		{User: User{Username: "user2"}},
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
	cfg := GitlabConfig{
		PullRequestID: 123,
	}
	codeOwnersContent := "sample content"

	owners := []CodeOwner{
		{Owner: "owner1"},
	}

	reactions := []*AwardEmoji{
		{User: User{Username: "user1"}},
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
	cfg := GitlabConfig{
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
	cfg := GitlabConfig{
		PullRequestID: 123,
	}
	codeOwnersContent := "sample content"

	owners := []CodeOwner{
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

	// Sample data
	owners := []CodeOwner{
		{Owner: "owner1"},
		{Owner: "owner2"},
	}

	reactions := []*AwardEmoji{
		{User: User{Username: "user1"}},
		{User: User{Username: "user2"}},
	}

	cfg := GitlabConfig{}

	// Setup expectations for all combinations
	mockProcessor.On("CanApprove", owners[0], reactions[0], cfg).Return(true)
	mockProcessor.On("CanApprove", owners[0], reactions[1], cfg).Return(false)
	mockProcessor.On("CanApprove", owners[1], reactions[0], cfg).Return(false)
	mockProcessor.On("CanApprove", owners[1], reactions[1], cfg).Return(true)

	// Call the function
	approvedBy := filterApprovals(owners, reactions, cfg, mockProcessor)

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

	// Sample data
	owners := []CodeOwner{
		{Owner: "owner1"},
	}

	reactions := []*AwardEmoji{
		{User: User{Username: "user1"}},
	}

	cfg := GitlabConfig{}

	// Setup expectations
	mockProcessor.On("CanApprove", owners[0], reactions[0], cfg).Return(false)

	// Call the function
	approvedBy := filterApprovals(owners, reactions, cfg, mockProcessor)

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
	cfg := GitlabConfig{
		BaseRepoOwner:  "owner",
		BaseRepoName:   "repo",
		CodeOwnersPath: "/path/to/CODEOWNERS",
		PullRequestID:  123,
	}
	project := &Project{
		ID:            1,
		DefaultBranch: "main",
	}

	codeOwnersContent := "sample content"

	// Setup expectations for GetProject
	mockClient.On("GetProject", fmt.Sprintf("%s/%s", cfg.BaseRepoOwner, cfg.BaseRepoName)).Return(project, nil)

	// Setup expectations for fetchCodeOwnersContent
	mockClient.On("GetFileContent", project.ID, project.DefaultBranch, cfg.CodeOwnersPath).Return(codeOwnersContent, nil)

	// Setup expectations for CheckMandatoryApproval
	owners := []CodeOwner{
		{Owner: "owner1"},
	}
	reactions := []*AwardEmoji{
		{User: User{Username: "user1"}},
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
	cfg := GitlabConfig{
		BaseRepoOwner:  "owner",
		BaseRepoName:   "repo",
		CodeOwnersPath: "/path/to/CODEOWNERS",
		PullRequestID:  123,
	}
	project := &Project{
		ID:            1,
		DefaultBranch: "main",
	}

	codeOwnersContent := "sample content"

	// Setup expectations for GetProject
	mockClient.On("GetProject", fmt.Sprintf("%s/%s", cfg.BaseRepoOwner, cfg.BaseRepoName)).Return(project, nil)

	// Setup expectations for fetchCodeOwnersContent
	mockClient.On("GetFileContent", project.ID, project.DefaultBranch, cfg.CodeOwnersPath).Return(codeOwnersContent, nil)

	// Setup expectations for CheckMandatoryApproval
	owners := []CodeOwner{
		{Owner: "owner1"},
	}
	reactions := []*AwardEmoji{
		{User: User{Username: "user1"}},
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
	cfg := GitlabConfig{
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
	cfg := GitlabConfig{
		BaseRepoOwner:  "owner",
		BaseRepoName:   "repo",
		CodeOwnersPath: "/path/to/CODEOWNERS",
		PullRequestID:  123,
	}
	project := &Project{
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
	cfg := GitlabConfig{
		Insecure:       false,
		BaseRepoOwner:  "owner",
		BaseRepoName:   "repo",
		CodeOwnersPath: "/path/to/CODEOWNERS",
		PullRequestID:  123,
	}
	project := &Project{
		ID:            1,
		DefaultBranch: "main",
	}
	codeOwnersContent := "sample content"

	// Setup expectations for GetProject
	mockClient.On("GetProject", fmt.Sprintf("%s/%s", cfg.BaseRepoOwner, cfg.BaseRepoName)).Return(project, nil)

	// Setup expectations for fetchCodeOwnersContent
	mockClient.On("GetFileContent", project.ID, project.DefaultBranch, cfg.CodeOwnersPath).Return(codeOwnersContent, nil)

	// Setup expectations for CheckMandatoryApproval
	owners := []CodeOwner{
		{Owner: "owner1"},
	}
	reactions := []*AwardEmoji{
		{User: User{Username: "user1"}},
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
	cfg := GitlabConfig{
		Insecure:       false,
		BaseRepoOwner:  "owner",
		BaseRepoName:   "repo",
		CodeOwnersPath: "/path/to/CODEOWNERS",
		PullRequestID:  123,
	}
	project := &Project{
		ID:            1,
		DefaultBranch: "main",
	}
	codeOwnersContent := "sample content"

	// Setup expectations for GetProject
	mockClient.On("GetProject", fmt.Sprintf("%s/%s", cfg.BaseRepoOwner, cfg.BaseRepoName)).Return(project, nil)

	// Setup expectations for fetchCodeOwnersContent
	mockClient.On("GetFileContent", project.ID, project.DefaultBranch, cfg.CodeOwnersPath).Return(codeOwnersContent, nil)

	// Setup expectations for CheckMandatoryApproval
	owners := []CodeOwner{
		{Owner: "owner1"},
	}
	reactions := []*AwardEmoji{
		{User: User{Username: "user1"}},
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
	cfg := GitlabConfig{
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
