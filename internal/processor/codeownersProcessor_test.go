package processor

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/shini4i/atlantis-emoji-gate/internal/client"
	"github.com/shini4i/atlantis-emoji-gate/internal/config"
	"github.com/stretchr/testify/assert"
)

// FaultyReader simulates an I/O error, which we'll use to test error handling.
type FaultyReader struct{}

func (f FaultyReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("simulated read error")
}

func TestCheckApproval(t *testing.T) {
	// Base configuration and reaction user used across many tests.
	// We will override fields as needed in specific test cases.
	baseCfg := config.GitlabConfig{
		ApproveEmoji:  "thumbsup",
		MrAuthor:      "mr_author",
		Insecure:      false,
		TerraformPath: "project/staging/app",
	}

	approvingUser := &client.AwardEmoji{
		Name: "thumbsup",
		User: client.User{Username: "approver"},
	}

	// Create the processor once.
	proc := NewProcessor()

	testCases := []struct {
		name         string
		codeowners   io.Reader
		reaction     *client.AwardEmoji
		config       config.GitlabConfig
		wantApproved bool
		wantErr      string // Substring of the expected error, if any.
	}{
		{
			name:         "Success: Wildcard path matches",
			codeowners:   strings.NewReader("* @approver"),
			reaction:     approvingUser,
			config:       baseCfg,
			wantApproved: true,
		},
		{
			name: "Success: Direct path glob matches",
			// Using 'project/staging/*' is a more precise glob pattern than 'project/staging/'.
			// This will correctly match 'project/staging/app'.
			codeowners:   strings.NewReader("project/staging/* @approver"),
			reaction:     approvingUser,
			config:       baseCfg,
			wantApproved: true,
		},
		{
			name:         "Failure: User is not in the CODEOWNERS list",
			codeowners:   strings.NewReader("* @some_other_user"),
			reaction:     approvingUser,
			config:       baseCfg,
			wantApproved: false,
		},
		{
			name:       "Failure: Wrong emoji is used for reaction",
			codeowners: strings.NewReader("* @approver"),
			reaction: &client.AwardEmoji{
				Name: "wrong_emoji", // Different from config
				User: client.User{Username: "approver"},
			},
			config:       baseCfg,
			wantApproved: false,
		},
		{
			name:       "Failure: MR author tries to approve their own MR",
			codeowners: strings.NewReader("* @mr_author"),
			reaction: &client.AwardEmoji{
				Name: "thumbsup",
				User: client.User{Username: "mr_author"}, // Same as MrAuthor in config
			},
			config:       baseCfg,
			wantApproved: false,
		},
		{
			name:       "Success: MR author can approve if insecure mode is on",
			codeowners: strings.NewReader("* @mr_author"),
			reaction: &client.AwardEmoji{
				Name: "thumbsup",
				User: client.User{Username: "mr_author"},
			},
			config: func() config.GitlabConfig {
				c := baseCfg
				c.Insecure = true // Override insecure mode
				return c
			}(),
			wantApproved: true,
		},
		{
			name:         "Failure: Path does not match any rule",
			codeowners:   strings.NewReader("project/production/* @approver"),
			reaction:     approvingUser,
			config:       baseCfg, // TerraformPath is 'project/staging/app'
			wantApproved: false,
		},
		{
			name:         "Failure: Reader returns an error",
			codeowners:   &FaultyReader{},
			reaction:     approvingUser,
			config:       baseCfg,
			wantApproved: false,
			wantErr:      "simulated read error",
		},
		{
			name:         "Failure: Malformed pattern in CODEOWNERS is an error",
			codeowners:   strings.NewReader("[* @approver"), // Malformed glob pattern
			reaction:     approvingUser,
			config:       baseCfg,
			wantApproved: false,
			wantErr:      "invalid pattern",
		},
		{
			name:         "Behavior: Empty file results in no approval",
			codeowners:   strings.NewReader(""),
			reaction:     approvingUser,
			config:       baseCfg,
			wantApproved: false,
		},
		{
			name:         "Behavior: File with only comments and malformed lines results in no approval",
			codeowners:   strings.NewReader("# A comment\n\n   \nmalformed_line_with_one_token"),
			reaction:     approvingUser,
			config:       baseCfg,
			wantApproved: false,
		},
		{
			name: "CRITICAL: Last matching rule wins (user in last rule)",
			codeowners: strings.NewReader(`
				* @generic_admin
				project/staging/* @approver
			`),
			reaction:     approvingUser,
			config:       baseCfg,
			wantApproved: true,
		},
		{
			name: "CRITICAL: Last matching rule wins (user not in last rule)",
			codeowners: strings.NewReader(`
				* @approver
				project/staging/* @stage_lead_only
			`),
			reaction:     approvingUser,
			config:       baseCfg,
			wantApproved: false,
		},
		{
			name: "CRITICAL: Multiple owners on the last matching line",
			codeowners: strings.NewReader(`
				* @admin
				project/staging/* @lead @other_approver @approver
			`),
			reaction:     approvingUser,
			config:       baseCfg,
			wantApproved: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			isApproved, err := proc.CheckApproval(tc.codeowners, tc.reaction, tc.config)

			// Assert
			if tc.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.wantApproved, isApproved)
		})
	}
}
