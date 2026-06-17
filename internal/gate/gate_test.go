package gate

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/shini4i/atlantis-emoji-gate/internal/client"
	clientmocks "github.com/shini4i/atlantis-emoji-gate/internal/client/mocks"
	"github.com/shini4i/atlantis-emoji-gate/internal/config"
	procmocks "github.com/shini4i/atlantis-emoji-gate/internal/processor/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// captureLogs replaces the default slog logger with one that writes to a buffer,
// and returns the buffer and a cleanup function.
// NOTE: This mutates global state (slog.SetDefault) and is NOT safe for use
// with t.Parallel().
func captureLogs(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, nil)
	original := slog.Default()
	slog.SetDefault(slog.New(handler))
	return &buf, func() { slog.SetDefault(original) }
}

// -------------------- Unit Tests --------------------

func TestFetchCodeOwnersContent(t *testing.T) {
	ctx := context.Background()

	t.Run("Success with separate CodeOwnersRepo", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		mc := clientmocks.NewMockGitlabClientInterface(ctrl)
		cfg := config.GitlabConfig{
			CodeOwnersRepo: "shared/codeowners",
			CodeOwnersPath: "CODEOWNERS",
		}
		coProject := &client.Project{ID: 42, DefaultBranch: "develop"}

		mc.EXPECT().GetProject(ctx, "shared/codeowners").Return(coProject, nil)
		mc.EXPECT().GetFileContent(ctx, 42, "develop", "CODEOWNERS").Return("* @admin", nil)

		content, err := fetchCodeOwnersContent(ctx, mc, cfg, &client.Project{ID: 1})
		assert.NoError(t, err)
		assert.Equal(t, "* @admin", content)
	})

	t.Run("Success with same project", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		mc := clientmocks.NewMockGitlabClientInterface(ctrl)
		cfg := config.GitlabConfig{CodeOwnersPath: "CODEOWNERS"}
		project := &client.Project{ID: 1, DefaultBranch: "main"}

		mc.EXPECT().GetFileContent(ctx, 1, "main", "CODEOWNERS").Return("* @dev", nil)

		content, err := fetchCodeOwnersContent(ctx, mc, cfg, project)
		assert.NoError(t, err)
		assert.Equal(t, "* @dev", content)
	})

	t.Run("Failure on GetProject", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		mc := clientmocks.NewMockGitlabClientInterface(ctrl)
		cfg := config.GitlabConfig{CodeOwnersRepo: "codeowners/repo"}

		mc.EXPECT().GetProject(ctx, "codeowners/repo").Return(nil, errors.New("project not found"))

		_, err := fetchCodeOwnersContent(ctx, mc, cfg, &client.Project{ID: 1})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get codeowners project")
	})

	t.Run("Failure on GetFileContent", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		mc := clientmocks.NewMockGitlabClientInterface(ctrl)
		cfg := config.GitlabConfig{}
		project := &client.Project{ID: 1, DefaultBranch: "main"}

		mc.EXPECT().GetFileContent(ctx, 1, "main", "").Return("", errors.New("file not found"))

		_, err := fetchCodeOwnersContent(ctx, mc, cfg, project)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})
}

func TestCheckMandatoryApproval(t *testing.T) {
	ctx := context.Background()

	t.Run("Success on second reaction", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		mc := clientmocks.NewMockGitlabClientInterface(ctrl)
		mp := procmocks.NewMockProcessor(ctrl)
		cfg := config.GitlabConfig{PullRequestID: 123}
		reactionValid := &client.AwardEmoji{User: client.User{Username: "approver"}}
		reactionInvalid := &client.AwardEmoji{User: client.User{Username: "non-approver"}}

		mc.EXPECT().ListAwardEmojis(ctx, 1, 123).Return([]*client.AwardEmoji{reactionInvalid, reactionValid}, nil)

		gomock.InOrder(
			mp.EXPECT().CheckApproval(gomock.Any(), reactionInvalid, cfg).Return(false, nil),
			mp.EXPECT().CheckApproval(gomock.Any(), reactionValid, cfg).Return(true, nil),
		)

		approved, err := CheckMandatoryApproval(ctx, mc, cfg, 1, "content", mp)
		assert.NoError(t, err)
		assert.True(t, approved)
	})

	t.Run("Restricted mode skips old and finds new approval", func(t *testing.T) {
		logBuf, cleanup := captureLogs(t)
		defer cleanup()

		ctrl := gomock.NewController(t)

		mc := clientmocks.NewMockGitlabClientInterface(ctrl)
		mp := procmocks.NewMockProcessor(ctrl)
		cfg := config.GitlabConfig{PullRequestID: 123, Restricted: true}
		commitTime := time.Now()
		reactionNew := &client.AwardEmoji{User: client.User{Username: "approver"}, UpdatedAt: commitTime.Add(time.Hour)}

		mc.EXPECT().ListAwardEmojis(ctx, 1, 123).Return([]*client.AwardEmoji{
			{User: client.User{Username: "approver"}, UpdatedAt: commitTime.Add(-time.Hour)},
			reactionNew,
		}, nil)
		mc.EXPECT().GetLatestCommitTimestamp(ctx, 1, 123).Return(commitTime, nil)

		// Only the new reaction should reach the processor.
		mp.EXPECT().CheckApproval(gomock.Any(), reactionNew, cfg).Return(true, nil)

		approved, err := CheckMandatoryApproval(ctx, mc, cfg, 1, "content", mp)
		assert.NoError(t, err)
		assert.True(t, approved)
		assert.Contains(t, logBuf.String(), "Skipping outdated approval")
	})

	t.Run("Empty reactions returns early", func(t *testing.T) {
		logBuf, cleanup := captureLogs(t)
		defer cleanup()

		ctrl := gomock.NewController(t)

		mc := clientmocks.NewMockGitlabClientInterface(ctrl)
		mc.EXPECT().ListAwardEmojis(ctx, 1, 0).Return([]*client.AwardEmoji{}, nil)

		approved, err := CheckMandatoryApproval(ctx, mc, config.GitlabConfig{}, 1, "", procmocks.NewMockProcessor(ctrl))
		assert.NoError(t, err)
		assert.False(t, approved)
		assert.Contains(t, logBuf.String(), "Mandatory approval not found")
	})

	t.Run("Error on ListAwardEmojis", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		mc := clientmocks.NewMockGitlabClientInterface(ctrl)
		mc.EXPECT().ListAwardEmojis(ctx, 1, 0).Return(nil, errors.New("api error"))

		_, err := CheckMandatoryApproval(ctx, mc, config.GitlabConfig{}, 1, "", procmocks.NewMockProcessor(ctrl))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch reactions")
	})

	t.Run("Error on GetLatestCommitTimestamp", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		mc := clientmocks.NewMockGitlabClientInterface(ctrl)
		cfg := config.GitlabConfig{Restricted: true}
		reaction := &client.AwardEmoji{User: client.User{Username: "approver"}}

		mc.EXPECT().ListAwardEmojis(ctx, 1, 0).Return([]*client.AwardEmoji{reaction}, nil)
		mc.EXPECT().GetLatestCommitTimestamp(ctx, 1, 0).Return(time.Time{}, errors.New("commit error"))

		_, err := CheckMandatoryApproval(ctx, mc, cfg, 1, "", procmocks.NewMockProcessor(ctrl))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch latest commit timestamp")
	})

	t.Run("Error on CheckApproval propagates", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		mc := clientmocks.NewMockGitlabClientInterface(ctrl)
		mp := procmocks.NewMockProcessor(ctrl)
		cfg := config.GitlabConfig{PullRequestID: 10}
		reaction := &client.AwardEmoji{User: client.User{Username: "approver"}}

		mc.EXPECT().ListAwardEmojis(ctx, 1, 10).Return([]*client.AwardEmoji{reaction}, nil)
		mp.EXPECT().CheckApproval(gomock.Any(), reaction, cfg).Return(false, errors.New("parse error"))

		_, err := CheckMandatoryApproval(ctx, mc, cfg, 1, "content", mp)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error during approval check")
		assert.Contains(t, err.Error(), "parse error")
	})
}

func TestProcessMR(t *testing.T) {
	ctx := context.Background()

	t.Run("Error on GetProject", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		mc := clientmocks.NewMockGitlabClientInterface(ctrl)
		cfg := config.GitlabConfig{BaseRepoOwner: "org", BaseRepoName: "repo"}

		mc.EXPECT().GetProject(ctx, "org/repo").Return(nil, errors.New("not found"))

		_, err := ProcessMR(ctx, mc, cfg, procmocks.NewMockProcessor(ctrl))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get project")
	})

	t.Run("Error on fetchCodeOwnersContent", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		mc := clientmocks.NewMockGitlabClientInterface(ctrl)
		cfg := config.GitlabConfig{BaseRepoOwner: "org", BaseRepoName: "repo", CodeOwnersPath: "CODEOWNERS"}
		project := &client.Project{ID: 1, DefaultBranch: "main"}

		mc.EXPECT().GetProject(ctx, "org/repo").Return(project, nil)
		mc.EXPECT().GetFileContent(ctx, 1, "main", "CODEOWNERS").Return("", errors.New("forbidden"))

		_, err := ProcessMR(ctx, mc, cfg, procmocks.NewMockProcessor(ctrl))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch CODEOWNERS file")
	})
}

func TestRun_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	ctx := context.Background()
	mc := clientmocks.NewMockGitlabClientInterface(ctrl)
	mp := procmocks.NewMockProcessor(ctrl)
	cfg := config.GitlabConfig{}
	project := &client.Project{ID: 1, DefaultBranch: "main"}
	reaction := &client.AwardEmoji{User: client.User{Username: "approver"}}

	mc.EXPECT().GetProject(ctx, "/").Return(project, nil)
	mc.EXPECT().GetFileContent(ctx, 1, "main", "").Return("content", nil)
	mc.EXPECT().ListAwardEmojis(ctx, 1, 0).Return([]*client.AwardEmoji{reaction}, nil)
	mp.EXPECT().CheckApproval(gomock.Any(), reaction, cfg).Return(true, nil)

	exitCode := Run(ctx, mc, cfg, mp)
	assert.Equal(t, 0, exitCode)
}

func TestRun_Error(t *testing.T) {
	logBuf, cleanup := captureLogs(t)
	defer cleanup()

	ctrl := gomock.NewController(t)

	ctx := context.Background()
	mc := clientmocks.NewMockGitlabClientInterface(ctrl)
	cfg := config.GitlabConfig{}

	mc.EXPECT().GetProject(ctx, "/").Return(nil, errors.New("gitlab is down"))

	exitCode := Run(ctx, mc, cfg, procmocks.NewMockProcessor(ctrl))

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, logBuf.String(), "Error processing MR")
	assert.Contains(t, logBuf.String(), "gitlab is down")
}

func TestRun_NotApproved(t *testing.T) {
	logBuf, cleanup := captureLogs(t)
	defer cleanup()

	ctrl := gomock.NewController(t)

	ctx := context.Background()
	mc := clientmocks.NewMockGitlabClientInterface(ctrl)
	mp := procmocks.NewMockProcessor(ctrl)
	cfg := config.GitlabConfig{}
	project := &client.Project{ID: 1, DefaultBranch: "main"}
	reaction := &client.AwardEmoji{User: client.User{Username: "someone"}}

	mc.EXPECT().GetProject(ctx, "/").Return(project, nil)
	mc.EXPECT().GetFileContent(ctx, 1, "main", "").Return("content", nil)
	mc.EXPECT().ListAwardEmojis(ctx, 1, 0).Return([]*client.AwardEmoji{reaction}, nil)
	mp.EXPECT().CheckApproval(gomock.Any(), reaction, cfg).Return(false, nil)

	exitCode := Run(ctx, mc, cfg, mp)
	assert.Equal(t, 1, exitCode)
	assert.Contains(t, logBuf.String(), "Mandatory approval not found")
}

func TestRun_InsecureMode(t *testing.T) {
	logBuf, cleanup := captureLogs(t)
	defer cleanup()

	ctrl := gomock.NewController(t)

	ctx := context.Background()
	mc := clientmocks.NewMockGitlabClientInterface(ctrl)
	mp := procmocks.NewMockProcessor(ctrl)
	cfg := config.GitlabConfig{Insecure: true}
	project := &client.Project{ID: 1, DefaultBranch: "main"}
	reaction := &client.AwardEmoji{User: client.User{Username: "approver"}}

	mc.EXPECT().GetProject(ctx, "/").Return(project, nil)
	mc.EXPECT().GetFileContent(ctx, 1, "main", "").Return("content", nil)
	mc.EXPECT().ListAwardEmojis(ctx, 1, 0).Return([]*client.AwardEmoji{reaction}, nil)
	mp.EXPECT().CheckApproval(gomock.Any(), reaction, cfg).Return(true, nil)

	exitCode := Run(ctx, mc, cfg, mp)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, logBuf.String(), "Insecure mode enabled")
}
