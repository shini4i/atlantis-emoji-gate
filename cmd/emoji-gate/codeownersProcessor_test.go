package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// FaultyReader simulates an error when reading content.
type FaultyReader struct{}

// Read always returns an error to simulate a faulty reader.
func (f FaultyReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("simulated read error")
}

// TestCanApprove tests the CanApprove function.
func TestCanApprove(t *testing.T) {
	cfg := GitlabConfig{
		ApproveEmoji:  "thumbsup",
		MrAuthor:      "author",
		Insecure:      false,
		TerraformPath: ".",
	}

	codeOwnersProcessor := CodeOwnersProcessor{}

	t.Run("Owner does not match", func(t *testing.T) {
		owner := CodeOwner{Owner: "user2", Path: "*"}
		reaction := &AwardEmoji{Name: "thumbsup", User: struct {
			Username string `json:"username"`
		}{Username: "user1"}}

		canApprove := codeOwnersProcessor.CanApprove(owner, reaction, cfg)
		assert.False(t, canApprove)
	})

	t.Run("Emoji does not match", func(t *testing.T) {
		owner := CodeOwner{Owner: "user1", Path: "*"}
		reaction := &AwardEmoji{Name: "thumbsdown", User: struct {
			Username string `json:"username"`
		}{Username: "user1"}}

		canApprove := codeOwnersProcessor.CanApprove(owner, reaction, cfg)
		assert.False(t, canApprove)
	})

	t.Run("MR author cannot approve their own MR", func(t *testing.T) {
		owner := CodeOwner{Owner: "author", Path: "*"}
		reaction := &AwardEmoji{Name: "thumbsup", User: struct {
			Username string `json:"username"`
		}{Username: "author"}}

		canApprove := codeOwnersProcessor.CanApprove(owner, reaction, cfg)
		assert.False(t, canApprove)
	})

	t.Run("Path does not match", func(t *testing.T) {
		owner := CodeOwner{Owner: "user1", Path: "non-matching-path"}
		reaction := &AwardEmoji{Name: "thumbsup", User: struct {
			Username string `json:"username"`
		}{Username: "user1"}}

		cfg.TerraformPath = "different-path"
		canApprove := codeOwnersProcessor.CanApprove(owner, reaction, cfg)
		assert.False(t, canApprove)
	})
}

func TestParseCodeOwnersErrors(t *testing.T) {
	coProcessor := CodeOwnersProcessor{}

	t.Run("Scanner Error", func(t *testing.T) {
		faultyReader := FaultyReader{}
		_, err := coProcessor.ParseCodeOwners(&faultyReader)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error reading CODEOWNERS content")
	})

	t.Run("Empty Content", func(t *testing.T) {
		content := ""
		owners, err := coProcessor.ParseCodeOwners(strings.NewReader(content))
		assert.NoError(t, err) // Expect no error for empty content
		assert.Empty(t, owners, "Owners should be empty for empty input")
	})

	t.Run("Malformed Lines", func(t *testing.T) {
		content := `
# Comment Only
docs
      `
		owners, err := coProcessor.ParseCodeOwners(strings.NewReader(content))
		assert.NoError(t, err)
		assert.Empty(t, owners, "Malformed lines should not produce owners")
	})

	t.Run("Valid and Malformed Mixed", func(t *testing.T) {
		content := `
* @user1
docs
/scripts @user2
#comment`

		expectedOwners := []CodeOwner{
			{Path: "*", Owner: "user1"},
			{Path: "/scripts", Owner: "user2"},
		}

		owners, err := coProcessor.ParseCodeOwners(strings.NewReader(content))
		assert.NoError(t, err)
		assert.Equal(t, expectedOwners, owners, "Only valid entries should be parsed")
	})
}
