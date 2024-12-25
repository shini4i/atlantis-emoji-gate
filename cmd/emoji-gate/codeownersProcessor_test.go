package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

func TestParseCodeOwners(t *testing.T) {
	codeOwnersProcessor := CodeOwnersProcessor{}

	t.Run("Empty CODEOWNERS file", func(t *testing.T) {
		content := ""
		owners, err := codeOwnersProcessor.ParseCodeOwners(strings.NewReader(content))

		assert.NoError(t, err)
		assert.Empty(t, owners, "No owners should be parsed from an empty CODEOWNERS file")
	})

	t.Run("Valid CODEOWNERS rules", func(t *testing.T) {
		content := `
# Valid rule
* @user1 @user2
/docs @doc_owner
/scripts @script_owner1 @script_owner2
		`

		expectedOwners := []CodeOwner{
			{Path: "*", Owner: "user1"},
			{Path: "*", Owner: "user2"},
			{Path: "/docs", Owner: "doc_owner"},
			{Path: "/scripts", Owner: "script_owner1"},
			{Path: "/scripts", Owner: "script_owner2"},
		}

		owners, err := codeOwnersProcessor.ParseCodeOwners(strings.NewReader(content))

		assert.NoError(t, err)
		assert.Equal(t, expectedOwners, owners, "Parsed owners data does not match expected output")
	})
}
