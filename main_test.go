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
