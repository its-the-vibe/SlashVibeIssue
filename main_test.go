package main

import (
	"encoding/json"
	"fmt"
	"strings"
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

func TestTitleGenerationOutputUnmarshal(t *testing.T) {
	tests := []struct {
		name           string
		jsonOutput     string
		expectedTitle  string
		expectedPrompt string
		expectError    bool
	}{
		{
			name: "Valid title generation output",
			jsonOutput: `{
"version": 1,
"title": "Fix Slack message search for commit hash in threads",
"prompt": "The slack message search by git commit hash is not working. Maybe the slack message search is not searching threads.  Can you investigate and fix?"
}`,
			expectedTitle:  "Fix Slack message search for commit hash in threads",
			expectedPrompt: "The slack message search by git commit hash is not working. Maybe the slack message search is not searching threads.  Can you investigate and fix?",
			expectError:    false,
		},
		{
			name: "Output with different version",
			jsonOutput: `{
"version": 2,
"title": "Update documentation",
"prompt": "Please update the README with new instructions"
}`,
			expectedTitle:  "Update documentation",
			expectedPrompt: "Please update the README with new instructions",
			expectError:    false,
		},
		{
			name:        "Invalid JSON",
			jsonOutput:  `{"invalid json"`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output TitleGenerationOutput
			err := json.Unmarshal([]byte(tt.jsonOutput), &output)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if output.Title != tt.expectedTitle {
				t.Errorf("Title = %q, want %q", output.Title, tt.expectedTitle)
			}

			if output.Prompt != tt.expectedPrompt {
				t.Errorf("Prompt = %q, want %q", output.Prompt, tt.expectedPrompt)
			}
		})
	}
}

func TestAgentInputStructuredFormat(t *testing.T) {
	// Test that the agent input is properly structured as JSON
	tests := []struct {
		name         string
		message      string
		expectedJSON string
	}{
		{
			name:         "Simple message",
			message:      "Fix the bug in the login page",
			expectedJSON: `{"message":"Fix the bug in the login page"}`,
		},
		{
			name:         "Message with quotes",
			message:      `The user said "hello" to me`,
			expectedJSON: `{"message":"The user said \"hello\" to me"}`,
		},
		{
			name:         "Message with newlines",
			message:      "First line\nSecond line",
			expectedJSON: `{"message":"First line\nSecond line"}`,
		},
		{
			name:         "Empty message",
			message:      "",
			expectedJSON: `{"message":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agentInput := AgentInput{
				Message: tt.message,
			}

			inputJSON, err := json.Marshal(agentInput)
			if err != nil {
				t.Errorf("Failed to marshal agent input: %v", err)
				return
			}

			if string(inputJSON) != tt.expectedJSON {
				t.Errorf("Agent input JSON = %q, want %q", string(inputJSON), tt.expectedJSON)
			}

			// Verify we can unmarshal it back
			var decoded AgentInput
			if err := json.Unmarshal(inputJSON, &decoded); err != nil {
				t.Errorf("Failed to unmarshal agent input: %v", err)
				return
			}

			if decoded.Message != tt.message {
				t.Errorf("Decoded message = %q, want %q", decoded.Message, tt.message)
			}
		})
	}
}

func TestParseRepoWithOrg(t *testing.T) {
	// Test that we correctly parse repo values that may include org
	tests := []struct {
		name             string
		repoInput        string
		configOrg        string
		expectedFullName string
	}{
		{
			name:             "Repo with org included",
			repoInput:        "my-org/my-repo",
			configOrg:        "default-org",
			expectedFullName: "my-org/my-repo",
		},
		{
			name:             "Repo without org (backward compatibility)",
			repoInput:        "my-repo",
			configOrg:        "default-org",
			expectedFullName: "default-org/my-repo",
		},
		{
			name:             "Different org via repo input",
			repoInput:        "other-org/SlashVibeIssue",
			configOrg:        "its-the-vibe",
			expectedFullName: "other-org/SlashVibeIssue",
		},
		{
			name:             "Personal repo via repo input",
			repoInput:        "username/personal-repo",
			configOrg:        "company-org",
			expectedFullName: "username/personal-repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the logic used in createGitHubIssue and sendConfirmation
			var repoFullName string
			if strings.Contains(tt.repoInput, "/") {
				repoFullName = tt.repoInput
			} else {
				repoFullName = fmt.Sprintf("%s/%s", tt.configOrg, tt.repoInput)
			}

			if repoFullName != tt.expectedFullName {
				t.Errorf("repoFullName = %q, want %q", repoFullName, tt.expectedFullName)
			}
		})
	}
}

func TestGitHubWebhookEventUnmarshal(t *testing.T) {
	tests := []struct {
		name             string
		jsonPayload      string
		expectedAction   string
		expectedAssignee string
		expectedIssueURL string
		expectError      bool
	}{
		{
			name: "Issue assigned to Copilot",
			jsonPayload: `{
"action": "assigned",
"assignee": {
"login": "Copilot",
"type": "Bot"
},
"issue": {
"html_url": "https://github.com/org/repo/issues/42",
"number": 42,
"title": "Test Issue"
}
}`,
			expectedAction:   "assigned",
			expectedAssignee: "Copilot",
			expectedIssueURL: "https://github.com/org/repo/issues/42",
			expectError:      false,
		},
		{
			name: "Issue assigned to regular user",
			jsonPayload: `{
"action": "assigned",
"assignee": {
"login": "johndoe",
"type": "User"
},
"issue": {
"html_url": "https://github.com/org/repo/issues/43",
"number": 43,
"title": "Another Test Issue"
}
}`,
			expectedAction:   "assigned",
			expectedAssignee: "johndoe",
			expectedIssueURL: "https://github.com/org/repo/issues/43",
			expectError:      false,
		},
		{
			name: "Issue closed event",
			jsonPayload: `{
"action": "closed",
"issue": {
"html_url": "https://github.com/org/repo/issues/44",
"number": 44,
"title": "Closed Issue"
}
}`,
			expectedAction:   "closed",
			expectedAssignee: "",
			expectedIssueURL: "https://github.com/org/repo/issues/44",
			expectError:      false,
		},
		{
			name:        "Invalid JSON",
			jsonPayload: `{"invalid json"`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var event GitHubWebhookEvent
			err := json.Unmarshal([]byte(tt.jsonPayload), &event)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if event.Action != tt.expectedAction {
				t.Errorf("Action = %q, want %q", event.Action, tt.expectedAction)
			}

			// Check assignee - handle nil case
			var actualAssignee string
			if event.Assignee != nil {
				actualAssignee = event.Assignee.Login
			}
			if actualAssignee != tt.expectedAssignee {
				t.Errorf("Assignee.Login = %q, want %q", actualAssignee, tt.expectedAssignee)
			}

			if event.Issue.HTMLURL != tt.expectedIssueURL {
				t.Errorf("Issue.HTMLURL = %q, want %q", event.Issue.HTMLURL, tt.expectedIssueURL)
			}
		})
	}
}
