package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCodeOwners(t *testing.T) {
	// Create a temporary CODEOWNERS file for testing
	content := `
# This is a comment
* @owner1 @owner2
/docs @docowner
`
	tmpFile, err := os.CreateTemp("", "CODEOWNERS")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			t.Fatalf("Failed to remove temp file: %v", err)
		}
	}(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Call the function to test
	owners, err := ParseCodeOwners(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to parse CODEOWNERS file: %v", err)
	}

	// Assert the expected values
	expectedOwners := []string{"owner1", "owner2"}
	assert.Equal(t, expectedOwners, owners)
}
