package main

import (
	"github.com/slack-go/slack"
)

func createIssueModal(initialTitle, initialDescription string, preselectCopilot bool) slack.ModalViewRequest {
	titleInput := &slack.PlainTextInputBlockElement{
		Type:     slack.METPlainTextInput,
		ActionID: "issue_title",
		Placeholder: &slack.TextBlockObject{
			Type: slack.PlainTextType,
			Text: "Brief summary of the issue",
		},
	}

	// Pre-populate title if provided
	if initialTitle != "" {
		titleInput.InitialValue = initialTitle
	}

	descriptionInput := &slack.PlainTextInputBlockElement{
		Type:      slack.METPlainTextInput,
		ActionID:  "issue_description",
		Multiline: true,
		Placeholder: &slack.TextBlockObject{
			Type: slack.PlainTextType,
			Text: "Provide more details, reproduction steps, etc.",
		},
	}

	// Pre-populate description if provided
	if initialDescription != "" {
		descriptionInput.InitialValue = initialDescription
	}

	// Create checkbox option
	copilotOption := &slack.OptionBlockObject{
		Text: &slack.TextBlockObject{
			Type: slack.PlainTextType,
			Text: "Assign to Copilot",
		},
		Value: "true",
	}

	// Create checkbox element with optional pre-selection
	checkboxElement := slack.NewCheckboxGroupsBlockElement(
		"assign_copilot",
		copilotOption,
	)

	// Pre-select the checkbox if requested
	if preselectCopilot {
		checkboxElement.InitialOptions = []*slack.OptionBlockObject{copilotOption}
	}

	// Create project checkbox option
	projectOption := &slack.OptionBlockObject{
		Text: &slack.TextBlockObject{
			Type: slack.PlainTextType,
			Text: "Add to project",
		},
		Value: "true",
	}

	// Create project checkbox element (selected by default)
	projectCheckboxElement := slack.NewCheckboxGroupsBlockElement(
		"add_to_project",
		projectOption,
	)
	projectCheckboxElement.InitialOptions = []*slack.OptionBlockObject{projectOption}

	return slack.ModalViewRequest{
		Type:       slack.VTModal,
		CallbackID: "create_github_issue_modal",
		Title: &slack.TextBlockObject{
			Type: slack.PlainTextType,
			Text: "New GitHub Issue",
		},
		Submit: &slack.TextBlockObject{
			Type: slack.PlainTextType,
			Text: "Create Issue",
		},
		Close: &slack.TextBlockObject{
			Type: slack.PlainTextType,
			Text: "Cancel",
		},
		Blocks: slack.Blocks{
			BlockSet: []slack.Block{
				&slack.SectionBlock{
					Type: slack.MBTSection,
					Text: &slack.TextBlockObject{
						Type: slack.MarkdownType,
						Text: "Fill out the details below to open a new issue in your repository.",
					},
				},
				&slack.InputBlock{
					Type:    slack.MBTInput,
					BlockID: "repo_selection_block",
					Label: &slack.TextBlockObject{
						Type: slack.PlainTextType,
						Text: "Select Repository",
					},
					Element: &slack.SelectBlockElement{
						Type:     slack.OptTypeExternal,
						ActionID: "SlashVibeIssue",
						Placeholder: &slack.TextBlockObject{
							Type: slack.PlainTextType,
							Text: "Search for a repo...",
						},
					},
				},
				&slack.InputBlock{
					Type:    slack.MBTInput,
					BlockID: "title_block",
					Label: &slack.TextBlockObject{
						Type: slack.PlainTextType,
						Text: "Issue Title",
					},
					Element: titleInput,
				},
				&slack.InputBlock{
					Type:    slack.MBTInput,
					BlockID: "description_block",
					Label: &slack.TextBlockObject{
						Type: slack.PlainTextType,
						Text: "Description",
					},
					Element: descriptionInput,
				},
				&slack.ActionBlock{
					Type:    slack.MBTAction,
					BlockID: "assignment_block",
					Elements: &slack.BlockElements{
						ElementSet: []slack.BlockElement{
							checkboxElement,
							projectCheckboxElement,
						},
					},
				},
			},
		},
	}
}
