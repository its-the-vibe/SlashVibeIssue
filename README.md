# SlashVibeIssue

Slack Slash Command for creating a new GitHub issue

## Overview

SlashVibeIssue is a Go service that listens for Slack slash commands via Redis and creates GitHub issues through an interactive modal dialog. When users run `/issue` in Slack, they're presented with a form to fill in issue details, select a repository, and optionally assign the issue to GitHub Copilot.

## Features

- 🎯 Interactive Slack modal for creating GitHub issues
- 🔄 Redis pub/sub for receiving Slack commands and view submissions
- ✨ Emoji reaction support to assign issues to Copilot after creation
- 🐙 Poppit integration for executing GitHub CLI commands
- ✅ Automatic confirmation messages via SlackLiner
- 🐳 Docker containerization with scratch runtime
- ⚙️ Configuration via environment variables

## Architecture

The service subscribes to four Redis channels:
1. **Slash commands channel** (default: `slack-commands`) - Receives `/issue` commands
2. **View submission channel** (default: `slack-relay-view-submission`) - Receives modal submissions
3. **Poppit output channel** (default: `poppit:command-output`) - Receives command execution output from Poppit
4. **Reaction added channel** (default: `slack-relay-reaction-added`) - Receives emoji reaction events

When a modal is submitted, the service:
1. Extracts repository, title, description, and assignment preference
2. Sends a GitHub issue creation command to Poppit with metadata for tracking
3. Waits for Poppit to execute the command and publish the output
4. Parses the issue URL from the command output
5. Publishes a confirmation message to the SlackLiner list with the issue URL for delivery to Slack

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_ADDR` | `host.docker.internal:6379` | Redis server address |
| `REDIS_PASSWORD` | _(empty)_ | Redis password |
| `REDIS_CHANNEL` | `slack-commands` | Channel for slash commands |
| `REDIS_VIEW_SUBMISSION_CHANNEL` | `slack-relay-view-submission` | Channel for view submissions |
| `REDIS_REACTION_CHANNEL` | `slack-relay-reaction-added` | Channel for emoji reaction events |
| `REDIS_SLACKLINER_LIST` | `slackliner:notifications` | Redis list for SlackLiner messages |
| `REDIS_POPPIT_LIST` | `poppit:commands` | Redis list for Poppit command execution |
| `REDIS_POPPIT_OUTPUT_CHANNEL` | `poppit:command-output` | Redis channel for Poppit command output |
| `SLACK_BOT_TOKEN` | _(required)_ | Slack bot token |
| `GITHUB_ORG` | _(required)_ | GitHub organization name |
| `WORKING_DIR` | `/tmp` | Working directory for gh commands |
| `CONFIRMATION_CHANNEL` | `gh-issues` | Slack channel for confirmations |
| `CONFIRMATION_TTL` | `48h` | TTL for confirmation messages |
| `PROJECT_ID` | `1` | GitHub project ID for automatic issue assignment |
| `PROJECT_ORG` | `its-the-vibe` | GitHub organization for project assignment |

## Building

### Local Build
```bash
go build -o slashvibeissue .
```

### Docker Build
```bash
docker build -t slashvibeissue:latest .
```

### Docker Compose
```bash
docker-compose up -d
```

## Running

### Prerequisites
- Go 1.24+ (for local development)
- Redis server
- Poppit service (for executing GitHub CLI commands)
- SlackLiner service (for sending confirmation messages)
- Slack workspace with bot token
- GitHub CLI (`gh`) with `project` scope for assigning issues to projects:
  ```bash
  gh auth refresh -s project
  ```

### Local Development
```bash
export SLACK_BOT_TOKEN=xoxb-your-token
export GITHUB_ORG=your-org
./slashvibeissue
```

### Docker
```bash
docker-compose up
```

## Usage

### Creating an Issue

1. In Slack, type `/issue`
2. Fill in the modal:
   - Select a repository (external select - requires integration)
   - Enter issue title
   - Enter issue description
   - Optionally check "Assign to Copilot"
   - "Add to project" checkbox is checked by default
3. Click "Create Issue"
4. Confirmation message appears in #gh-issues channel

### Assigning Issue to Copilot via Emoji Reaction

After an issue is created, you can assign it to Copilot by reacting to the confirmation message with the ✨ (`:sparkles:`) emoji:

1. React with ✨ to any issue confirmation message
2. The service will automatically assign the issue to @copilot (if not already assigned)
3. Only works if:
   - The issue was not already assigned to Copilot during creation
   - The reaction is from a human user (not a bot)
   - The confirmation message has valid metadata

Note: The confirmation messages include metadata about the issue (URL, repository, assignment status) to support this feature.

## Integration Points

- **Poppit**: For executing GitHub CLI commands asynchronously
- **SlackLiner**: For sending timed confirmation messages to Slack

## Modal Structure

The service uses the callback ID `create_github_issue_modal` to identify submissions. The modal includes:
- Repository selection (external select with action_id `SlashVibeIssue`)
- Issue title (plain text input)
- Issue description (multiline text input)
- Copilot assignment checkbox
- Add to project checkbox (checked by default)

## License

See repository license.
