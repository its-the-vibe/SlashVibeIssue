# Copilot Instructions for SlashVibeIssue

## Project Overview
SlashVibeIssue is a Go-based microservice that integrates Slack slash commands with GitHub issue creation. It uses Redis for pub/sub messaging and communicates with two external services: Poppit (for executing GitHub CLI commands) and SlackLiner (for sending timed confirmation messages).

## Architecture
- **Language**: Go 1.24+
- **Communication**: Redis pub/sub for receiving Slack events
- **External Services**: 
  - Poppit: Executes GitHub CLI commands asynchronously
  - SlackLiner: Sends timed confirmation messages to Slack
- **Deployment**: Docker with multi-stage build using scratch runtime

## Code Conventions

### Go Style
- Follow standard Go conventions and idioms
- Use `gofmt` for formatting
- Keep functions focused and single-purpose
- Use meaningful variable names (e.g., `repo`, `title`, `description`)

### Configuration
- All configuration is managed via environment variables
- Use the `Config` struct to centralize configuration
- Provide sensible defaults using `getEnv()` helper function
- Document all environment variables in the README

### Error Handling
- Log errors with context using `log.Printf()`
- Return errors from functions that can fail
- Don't panic; use `log.Fatal()` only for startup failures (missing required config)

### JSON Handling
- Use struct tags for JSON marshaling/unmarshaling
- Handle JSON parsing errors gracefully with logging
- Use proper type assertions when working with `interface{}` from JSON

### Redis Patterns
- Use pub/sub for receiving events from Slack
- Use lists (RPUSH) for sending commands/messages to other services
- Always check for context cancellation in long-running goroutines
- Close resources (pubsub, client) with defer

### Slack Integration
- Use the `slack-go/slack` library for Slack API interactions
- Build modals using the Block Kit API
- Use proper action IDs and callback IDs for identification
- Pre-populate modal fields when appropriate (e.g., `:sparkles:` command)

### String Handling
- Escape single quotes in shell commands using `strings.ReplaceAll(str, "'", "'\''")` 
- Use `strings.TrimSpace()` for user input
- Use `fmt.Sprintf()` for string formatting

### Modal Structure
- The callback ID `create_github_issue_modal` identifies issue creation submissions
- Repository selection uses external select with action_id `SlashVibeIssue`
- Use consistent block IDs: `repo_selection_block`, `title_block`, `description_block`, `assignment_block`

## Special Features

### Sparkles Command
When users type `/issue :sparkles:`, the modal is pre-populated with:
- Title: "✨ Set up Copilot instructions"
- Description: Template with onboarding link
- Copilot assignment: Pre-selected

This pattern can be extended for other preset issue templates.

## Testing
- Tests are in `main_test.go`
- Use table-driven tests where appropriate
- Test configuration loading, JSON parsing, and string escaping

## Building and Running

### Local Development
```bash
go build -o slashvibeissue .
export SLACK_BOT_TOKEN=xoxb-your-token
export GITHUB_ORG=your-org
./slashvibeissue
```

### Docker
```bash
docker build -t slashvibeissue:latest .
docker-compose up
```

## Key Integration Points

### Poppit Commands
- Send commands to `poppit:commands` Redis list
- Include repo, branch, type, dir, and commands array
- Type should be `slash-vibe-issue` for issue creation

### SlackLiner Messages
- Send messages to SlackLiner Redis list (configurable)
- Include channel, text, and TTL
- Used for confirmation messages after issue creation

## Security Considerations
- Never log sensitive data (tokens, passwords)
- Validate and escape user input before using in shell commands
- Use environment variables for all secrets
- Run Docker container as non-root when possible
