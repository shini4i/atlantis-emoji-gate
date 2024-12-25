package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// FaultyReader simulates an error when reading content.
type FaultyReader struct{}

func (f FaultyReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("simulated read error")
}

func TestCanApprove(t *testing.T) {
	cfg := GitlabConfig{
		ApproveEmoji:  "thumbsup",
		MrAuthor:      "author",
		Insecure:      false,
		TerraformPath: "matching-path/subdir",
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
			// We'll override TerraformPath to something else
			tfPath:      "different-path",
			wantApprove: false,
		},
		{
			name: "Wildcard owner path matches everything",
			owner: CodeOwner{
				Owner: "user1",
				Path:  "*",
			},
			reaction: &AwardEmoji{
				Name: "thumbsup",
				User: struct {
					Username string `json:"username"`
				}{Username: "user1"},
			},
			wantApprove: true,
		},
		{
			name: "Terraform path matches owner path prefix",
			owner: CodeOwner{
				Owner: "user3",
				Path:  "matching-path",
			},
			reaction: &AwardEmoji{
				Name: "thumbsup",
				User: struct {
					Username string `json:"username"`
				}{Username: "user3"},
			},
			wantApprove: true,
		},
		{
			name: "Terraform path does not match owner path prefix",
			owner: CodeOwner{
				Owner: "user4",
				Path:  "non-matching-path",
			},
			reaction: &AwardEmoji{
				Name: "thumbsup",
				User: struct {
					Username string `json:"username"`
				}{Username: "user4"},
			},
			wantApprove: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Copy the base config so we can override TerraformPath if needed
			localCfg := cfg
			if tc.tfPath != "" {
				localCfg.TerraformPath = tc.tfPath
			}

			got := codeOwnersProcessor.CanApprove(tc.owner, tc.reaction, localCfg)
			assert.Equal(t, tc.wantApprove, got)
		})
	}
}

func TestParseCodeOwnersErrors(t *testing.T) {
	coProcessor := CodeOwnersProcessor{}

	testCases := []struct {
		name          string
		input         string
		reader        *FaultyReader // Only used if simulating a read error
		wantErrSubstr string        // if non-empty, we expect an error containing this
		wantEmpty     bool          // if true, we expect owners to be empty
		wantOwners    []CodeOwner   // for valid lines
	}{
		{
			name:          "Scanner Error",
			reader:        &FaultyReader{},
			wantErrSubstr: "error reading CODEOWNERS content",
		},
		{
			name:       "Empty Content",
			input:      "",
			wantEmpty:  true,
			wantOwners: nil, // owners should be empty
		},
		{
			name:       "Malformed Lines",
			input:      "# Comment Only\ndocs\n      ",
			wantEmpty:  true,
			wantOwners: nil,
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
			var owners []CodeOwner
			var err error

			if tc.reader != nil {
				// Use faulty reader
				owners, err = coProcessor.ParseCodeOwners(tc.reader)
			} else {
				owners, err = coProcessor.ParseCodeOwners(strings.NewReader(tc.input))
			}

			if tc.wantErrSubstr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrSubstr)
			} else {
				assert.NoError(t, err)
			}

			if tc.wantEmpty {
				assert.Empty(t, owners)
			}
			if tc.wantOwners != nil {
				assert.Equal(t, tc.wantOwners, owners)
			}
		})
	}
}
