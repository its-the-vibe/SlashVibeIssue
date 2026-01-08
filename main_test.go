package main

import (
	"testing"

	"github.com/slack-go/slack"
)

func TestCreateIssueModalWithSparkles(t *testing.T) {
	// Test with sparkles emoji - should pre-populate title, description, and preselect copilot
	modal := createIssueModal(
		"✨ Set up Copilot instructions",
		"Configure instructions for this repository as documented in [Best practices for Copilot coding agent in your repository](https://gh.io/copilot-coding-agent-tips).\n\n<Onboard this repo>",
		true,
	)

	// Check modal structure
	if modal.Type != "modal" {
		t.Errorf("Expected modal type to be 'modal', got '%s'", modal.Type)
	}

	if modal.CallbackID != "create_github_issue_modal" {
		t.Errorf("Expected callback_id to be 'create_github_issue_modal', got '%s'", modal.CallbackID)
	}

	// Verify we have the expected number of blocks
	if len(modal.Blocks.BlockSet) != 5 {
		t.Errorf("Expected 5 blocks, got %d", len(modal.Blocks.BlockSet))
	}

	// Check title block (index 2)
	titleBlock := modal.Blocks.BlockSet[2]
	if inputBlock, ok := titleBlock.(*slack.InputBlock); ok {
		if inputBlock.BlockID != "title_block" {
			t.Errorf("Expected block_id to be 'title_block', got '%s'", inputBlock.BlockID)
		}
		// We can't easily check the InitialValue without more complex type assertions
	} else {
		t.Error("Expected block at index 2 to be an InputBlock")
	}
}

func TestCreateIssueModalWithoutSparkles(t *testing.T) {
	// Test without sparkles emoji - should have empty values
	modal := createIssueModal("", "", false)

	// Check modal structure
	if modal.Type != "modal" {
		t.Errorf("Expected modal type to be 'modal', got '%s'", modal.Type)
	}

	if modal.CallbackID != "create_github_issue_modal" {
		t.Errorf("Expected callback_id to be 'create_github_issue_modal', got '%s'", modal.CallbackID)
	}

	// Verify we have the expected number of blocks
	if len(modal.Blocks.BlockSet) != 5 {
		t.Errorf("Expected 5 blocks, got %d", len(modal.Blocks.BlockSet))
	}
}

func TestCreateIssueModalWithCustomTitle(t *testing.T) {
	// Test with custom title
	customTitle := "My custom issue"
	modal := createIssueModal(customTitle, "", false)

	// Check modal structure
	if modal.Type != "modal" {
		t.Errorf("Expected modal type to be 'modal', got '%s'", modal.Type)
	}

	// Verify we have the expected number of blocks
	if len(modal.Blocks.BlockSet) != 5 {
		t.Errorf("Expected 5 blocks, got %d", len(modal.Blocks.BlockSet))
	}
}

func TestExtractIssueURL(t *testing.T) {
tests := []struct {
name     string
output   string
expected string
}{
{
name: "Valid output with issue URL",
output: `Creating issue in its-the-vibe/SlashVibeIssue

https://github.com/its-the-vibe/SlashVibeIssue/issues/13`,
expected: "https://github.com/its-the-vibe/SlashVibeIssue/issues/13",
},
{
name: "Output with extra whitespace",
output: `Creating issue in its-the-vibe/SlashVibeIssue

  https://github.com/its-the-vibe/SlashVibeIssue/issues/42  `,
expected: "https://github.com/its-the-vibe/SlashVibeIssue/issues/42",
},
{
name: "Multi-line output with URL in middle",
output: `Creating issue in its-the-vibe/SlashVibeIssue
Some other text
https://github.com/its-the-vibe/TestRepo/issues/1
More text`,
expected: "https://github.com/its-the-vibe/TestRepo/issues/1",
},
{
name:     "Output without issue URL",
output:   "Creating issue in its-the-vibe/SlashVibeIssue\nSome error occurred",
expected: "",
},
{
name:     "Empty output",
output:   "",
expected: "",
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
result := extractIssueURL(tt.output)
if result != tt.expected {
t.Errorf("extractIssueURL() = %q, want %q", result, tt.expected)
}
})
}
}

func TestCreateIssueModalWithProjectCheckbox(t *testing.T) {
	// Test that the modal includes the "Add to project" checkbox selected by default
	modal := createIssueModal("", "", false)

	// Verify we have the expected number of blocks
	if len(modal.Blocks.BlockSet) != 5 {
		t.Errorf("Expected 5 blocks, got %d", len(modal.Blocks.BlockSet))
	}

	// Check assignment block (index 4)
	assignmentBlock := modal.Blocks.BlockSet[4]
	if actionBlock, ok := assignmentBlock.(*slack.ActionBlock); ok {
		if actionBlock.BlockID != "assignment_block" {
			t.Errorf("Expected block_id to be 'assignment_block', got '%s'", actionBlock.BlockID)
		}
		
		// Verify we have 2 elements (assign to copilot and add to project checkboxes)
		if len(actionBlock.Elements.ElementSet) != 2 {
			t.Errorf("Expected 2 checkbox elements in assignment block, got %d", len(actionBlock.Elements.ElementSet))
		}
	} else {
		t.Error("Expected block at index 4 to be an ActionBlock")
	}
}

func TestExtractIssueNumber(t *testing.T) {
	tests := []struct {
		name     string
		issueURL string
		expected int
	}{
		{
			name:     "Valid issue URL",
			issueURL: "https://github.com/its-the-vibe/SlashVibeIssue/issues/42",
			expected: 42,
		},
		{
			name:     "Another valid issue URL",
			issueURL: "https://github.com/org/repo/issues/123",
			expected: 123,
		},
		{
			name:     "Invalid URL",
			issueURL: "https://github.com/org/repo",
			expected: 0,
		},
		{
			name:     "Empty URL",
			issueURL: "",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractIssueNumber(tt.issueURL)
			if result != tt.expected {
				t.Errorf("extractIssueNumber() = %d, want %d", result, tt.expected)
			}
		})
	}
}
