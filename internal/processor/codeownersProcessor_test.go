package processor

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

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
			name:         "Success: Directory pattern owns the project directory",
			codeowners:   strings.NewReader("project/staging/app/ @approver"),
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
			name:         "Failure: * in a pattern does not cross a path separator",
			codeowners:   strings.NewReader("project/staging/* @approver"),
			reaction:     approvingUser,
			config:       baseCfg, // TerraformPath is 'project/staging/app'
			wantApproved: false,
		},
		{
			name:         "Failure: A path without a trailing slash names a file, not a directory",
			codeowners:   strings.NewReader("/project/staging/app @approver"),
			reaction:     approvingUser,
			config:       baseCfg, // TerraformPath is 'project/staging/app'
			wantApproved: false,
		},
		{
			name:         "Failure: Path does not match any rule",
			codeowners:   strings.NewReader("project/production/ @approver"),
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
			name:         "Behavior: Owner-less matching line does not override an earlier rule",
			codeowners:   strings.NewReader("* @approver\nproject/staging/"),
			reaction:     approvingUser,
			config:       baseCfg,
			wantApproved: true,
		},
		{
			name:         "Behavior: Non-ASCII leading space is part of the pattern, as in GitLab",
			codeowners:   strings.NewReader("\u00a0* @approver"),
			reaction:     approvingUser,
			config:       baseCfg,
			wantApproved: false,
		},
		{
			name: "CRITICAL: Last matching rule wins (user in last rule)",
			codeowners: strings.NewReader(`
				* @generic_admin
				project/staging/app/ @approver
			`),
			reaction:     approvingUser,
			config:       baseCfg,
			wantApproved: true,
		},
		{
			name: "CRITICAL: Last matching rule wins (user not in last rule)",
			codeowners: strings.NewReader(`
				* @approver
				project/staging/ @stage_lead_only
			`),
			reaction:     approvingUser,
			config:       baseCfg,
			wantApproved: false,
		},
		{
			name: "CRITICAL: Multiple owners on the last matching line",
			codeowners: strings.NewReader(`
				* @admin
				project/staging/ @lead @other_approver @approver
			`),
			reaction:     approvingUser,
			config:       baseCfg,
			wantApproved: true,
		},
		{
			name:         "CRITICAL: Leading slash anchors the pattern at the repository root",
			codeowners:   strings.NewReader("/project/staging/ @approver"),
			reaction:     approvingUser,
			config:       baseCfg,
			wantApproved: true,
		},
		{
			name: "CRITICAL: Leading slash pattern with last-match-wins",
			codeowners: strings.NewReader(`
				* @generic_admin
				/project/staging/app/ @approver
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

func TestNormalizePattern(t *testing.T) {
	testCases := []struct {
		pattern string
		want    string
	}{
		{pattern: "*", want: "/**/*"},
		{pattern: "/README.md", want: "/README.md"},
		{pattern: "README.md", want: "/**/README.md"},
		{pattern: "/docs/", want: "/docs/**/*"},
		{pattern: "docs/", want: "/**/docs/**/*"},
		{pattern: "/docs/*.md", want: "/docs/*.md"},
		{pattern: `\#notes/`, want: "/**/#notes/**/*"},
		{pattern: `/my\ dir/`, want: "/my dir/**/*"},
		{pattern: "/my\\\vdir/", want: "/my dir/**/*"},
	}

	for _, tc := range testCases {
		t.Run(tc.pattern, func(t *testing.T) {
			assert.Equal(t, tc.want, normalizePattern(tc.pattern))
		})
	}
}

func TestSplitEntry(t *testing.T) {
	testCases := []struct {
		name        string
		line        string
		wantPattern string
		wantOwners  []string
	}{
		{name: "pattern and owners", line: "/docs/ @a @b", wantPattern: "/docs/", wantOwners: []string{"@a", "@b"}},
		{name: "escaped space stays in the pattern", line: `/my\ dir/ @a`, wantPattern: `/my\ dir/`, wantOwners: []string{"@a"}},
		{name: "pattern without owners", line: "/docs/", wantPattern: "/docs/", wantOwners: nil},
		{
			name:        "non-ASCII space does not split, as Ruby \\s is ASCII only",
			line:        "/docs/ @evil @good",
			wantPattern: "/docs/ @evil",
			wantOwners:  []string{"@good"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pattern, owners := splitEntry(tc.line)
			assert.Equal(t, tc.wantPattern, pattern)
			assert.Equal(t, tc.wantOwners, owners)
		})
	}
}

func TestOwnsDirectory(t *testing.T) {
	const dir = "project/staging/app"

	testCases := []struct {
		name    string
		pattern string
		dir     string
		want    bool
	}{
		{name: "* owns every directory", pattern: "*", dir: dir, want: true},
		{name: "* owns the repository root", pattern: "*", dir: ".", want: true},
		{name: "/* owns the repository root", pattern: "/*", dir: ".", want: true},
		{name: "/* does not own a subdirectory", pattern: "/*", dir: dir, want: false},
		{name: "absolute directory path", pattern: "/project/staging/app/", dir: dir, want: true},
		{name: "absolute ancestor directory path", pattern: "/project/", dir: dir, want: true},
		{name: "files directly in the directory", pattern: "/project/staging/app/*", dir: dir, want: true},
		{name: "trailing ** acts as a single *", pattern: "/project/staging/app/**", dir: dir, want: true},
		{name: "trailing ** does not cross a separator", pattern: "/project/staging/**", dir: dir, want: false},
		{name: "* does not cross a separator", pattern: "/project/staging/*", dir: dir, want: false},
		{name: "path without trailing slash names a file", pattern: "/project/staging/app", dir: dir, want: false},
		{name: "relative full path", pattern: "project/staging/app/", dir: dir, want: true},
		{name: "relative path matches at any depth", pattern: "staging/app/", dir: dir, want: true},
		{name: "relative single directory matches at any depth", pattern: "staging/", dir: dir, want: true},
		{name: "absolute path is anchored at the root", pattern: "/staging/", dir: dir, want: false},
		{name: "file name pattern does not own a directory", pattern: "*.tf", dir: dir, want: false},
		{name: "* inside a segment", pattern: "/project/*/app/", dir: dir, want: true},
		{name: "? matches one character", pattern: "/project/st?ging/app/", dir: dir, want: true},
		{name: "character class", pattern: "/project/[rs]taging/app/", dir: dir, want: true},
		{name: "negated class with !", pattern: "/project/[!p]taging/app/", dir: dir, want: true},
		{name: "negated class with ^", pattern: "/project/[^s]taging/app/", dir: dir, want: false},
		{name: "globstar matches many directories", pattern: "/project/**/app/", dir: dir, want: true},
		{name: "globstar matches zero directories", pattern: "/project/**/staging/app/", dir: dir, want: true},
		{name: "globstar followed by *", pattern: "/project/**/*", dir: dir, want: true},
		{name: "matching is case-sensitive", pattern: "/Project/", dir: dir, want: false},
		{name: "* matches a leading dot", pattern: "/*/app/", dir: ".config/app", want: true},
		{name: "globstar crosses dot directories", pattern: "/**/app/", dir: ".config/app", want: true},
		{name: "escaped hash", pattern: `\#notes/`, dir: "#notes", want: true},
		{name: "escaped space", pattern: `/my\ dir/`, dir: "my dir", want: true},
		{name: "escaped [! is a literal", pattern: `/a\[!b/`, dir: "a[!b", want: true},
		{name: "dir with ./ prefix", pattern: "/project/staging/app/", dir: "./project/staging/app", want: true},
		{name: "dir with trailing slash", pattern: "/project/staging/app/", dir: "project/staging/app/", want: true},
		{name: "dir with leading slash", pattern: "/project/staging/app/", dir: "/project/staging/app", want: true},
		{name: "empty dir is the repository root", pattern: "/*", dir: "", want: true},
		{name: "./ prefixed subdirectory is not the root", pattern: "/*", dir: "./project", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ownsDirectory(normalizePattern(tc.pattern), tc.dir)
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	t.Run("many globstars do not cause exponential matching", func(t *testing.T) {
		pattern := "/" + strings.Repeat("**/", 2000) + "y/"
		done := make(chan bool)
		go func() {
			got, _ := ownsDirectory(normalizePattern(pattern), "x/x/x/x/x/x/x/x")
			done <- got
		}()
		select {
		case got := <-done:
			assert.False(t, got)
		case <-time.After(5 * time.Second):
			t.Fatal("ownsDirectory did not finish within 5s")
		}
	})

	t.Run("malformed pattern is an error even when no segment is compared", func(t *testing.T) {
		_, err := ownsDirectory(normalizePattern("/a/[/"), ".")
		assert.Error(t, err)
	})
}
