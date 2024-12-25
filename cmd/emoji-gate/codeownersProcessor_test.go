package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// -----------------------------------------------------------------------------
// Mock & Helper Types
// -----------------------------------------------------------------------------

// FaultyReader simulates an error when reading content.
type FaultyReader struct{}

func (f FaultyReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("simulated read error")
}

// -----------------------------------------------------------------------------
// Test: CanApprove
// -----------------------------------------------------------------------------

func TestCanApprove(t *testing.T) {
	baseCfg := GitlabConfig{
		ApproveEmoji:  "thumbsup",
		MrAuthor:      "author",
		Insecure:      false,
		TerraformPath: ".",
	}
	codeOwnersProcessor := CodeOwnersProcessor{}

	testCases := []struct {
		name        string
		owner       CodeOwner
		reaction    *AwardEmoji
		tfPath      string
		wantApprove bool
	}{
		{
			name: "Owner does not match",
			owner: CodeOwner{
				Owner: "user2",
				Path:  "*",
			},
			reaction: &AwardEmoji{
				Name: "thumbsup",
				User: struct {
					Username string `json:"username"`
				}{Username: "user1"},
			},
			wantApprove: false,
		},
		{
			name: "Emoji does not match",
			owner: CodeOwner{
				Owner: "user1",
				Path:  "*",
			},
			reaction: &AwardEmoji{
				Name: "thumbsdown",
				User: struct {
					Username string `json:"username"`
				}{Username: "user1"},
			},
			wantApprove: false,
		},
		{
			name: "MR author cannot approve their own MR",
			owner: CodeOwner{
				Owner: "author",
				Path:  "*",
			},
			reaction: &AwardEmoji{
				Name: "thumbsup",
				User: struct {
					Username string `json:"username"`
				}{Username: "author"},
			},
			wantApprove: false,
		},
		{
			name: "Path does not match",
			owner: CodeOwner{
				Owner: "user1",
				Path:  "non-matching-path",
			},
			reaction: &AwardEmoji{
				Name: "thumbsup",
				User: struct {
					Username string `json:"username"`
				}{Username: "user1"},
			},
			tfPath:      "different-path",
			wantApprove: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := overrideTerraformPath(baseCfg, tc.tfPath)
			got := codeOwnersProcessor.CanApprove(tc.owner, tc.reaction, cfg)
			assert.Equal(t, tc.wantApprove, got)
		})
	}
}

// overrideTerraformPath overrides the TerraformPath if tfPath is non-empty.
func overrideTerraformPath(cfg GitlabConfig, tfPath string) GitlabConfig {
	if tfPath == "" {
		return cfg
	}
	newCfg := cfg
	newCfg.TerraformPath = tfPath
	return newCfg
}

// -----------------------------------------------------------------------------
// Test: ParseCodeOwnersErrors
// -----------------------------------------------------------------------------

func TestParseCodeOwnersErrors(t *testing.T) {
	coProcessor := CodeOwnersProcessor{}

	testCases := []struct {
		name          string
		input         string
		reader        *FaultyReader // Used if we simulate a read error
		wantErrSubstr string        // If set, expect error to contain this
		wantEmpty     bool          // If true, expect owners to be empty
		wantOwners    []CodeOwner   // Expected on success
	}{
		{
			name:          "Scanner Error",
			reader:        &FaultyReader{},
			wantErrSubstr: "error reading CODEOWNERS content",
		},
		{
			name:      "Empty Content",
			input:     "",
			wantEmpty: true,
		},
		{
			name:      "Malformed Lines",
			input:     "# Comment Only\ndocs\n      ",
			wantEmpty: true,
		},
		{
			name: "Valid and Malformed Mixed",
			input: `
* @user1
docs
/scripts @user2
#comment
`,
			wantOwners: []CodeOwner{
				{Path: "*", Owner: "user1"},
				{Path: "/scripts", Owner: "user2"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			owners, err := parseOwners(coProcessor, tc.input, tc.reader)
			checkOwners(t, owners, err, tc.wantErrSubstr, tc.wantEmpty, tc.wantOwners)
		})
	}
}

// parseOwners reads CODEOWNERS data via either a FaultyReader or strings.Reader.
func parseOwners(coProcessor CodeOwnersProcessor, input string, reader *FaultyReader) ([]CodeOwner, error) {
	if reader != nil {
		return coProcessor.ParseCodeOwners(reader)
	}
	return coProcessor.ParseCodeOwners(strings.NewReader(input))
}

// checkOwners centralizes the repetitive assertions.
func checkOwners(
	t *testing.T,
	owners []CodeOwner,
	err error,
	wantErrSubstr string,
	wantEmpty bool,
	wantOwners []CodeOwner,
) {
	if wantErrSubstr != "" {
		assert.Error(t, err)
		assert.Contains(t, err.Error(), wantErrSubstr)
	} else {
		assert.NoError(t, err)
	}
	if wantEmpty {
		assert.Empty(t, owners)
	}
	if wantOwners != nil {
		assert.Equal(t, wantOwners, owners)
	}
}
