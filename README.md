# teams-local-mcp

A local MCP server that connects Claude to Microsoft Teams via the Microsoft Graph API. Read chats, browse teams and channels, and send messages — all from your terminal.

## Features

- **Chat**: list chats, read messages, send messages
- **Teams & Channels**: list teams, browse channels, read channel messages, post to channels
- **Multi-account**: add and switch between Microsoft accounts
- **Local-only**: runs as a single binary on your machine, no cloud relay

## Quick Start

### 1. Install

```bash
go install github.com/torenander/teams-local-mcp/cmd/teams-local-mcp@latest
```

Or build from source:

```bash
git clone https://github.com/torenander/teams-local-mcp.git
cd teams-local-mcp
go install ./cmd/teams-local-mcp/
```

### 2. Configure Claude Code

Add to `~/.claude.json` under `mcpServers`:

```json
"teams-local-mcp": {
  "type": "stdio",
  "command": "teams-local-mcp",
  "args": ["--stdio"],
  "env": {
    "TEAMS_MCP_TEAMS_ENABLED": "true",
    "TEAMS_MCP_TEAMS_MANAGE_ENABLED": "true"
  }
}
```

### 3. Authenticate

On first use, the server will prompt you to authenticate via device code flow. Open the URL shown and enter the code to grant Teams permissions.

## Available Operations

### Chat

| Verb | Description |
|------|-------------|
| `list_chats` | List your 1:1 and group chats |
| `get_chat` | Get details for a specific chat |
| `list_messages` | List messages in a chat |
| `get_message` | Get a specific message from a chat |
| `send_message` | Send a message to a chat (requires TEAMS_MANAGE_ENABLED) |

### Teams

| Verb | Description |
|------|-------------|
| `list_teams` | List teams you are a member of |
| `get_team` | Get details for a specific team |
| `list_channels` | List channels in a team |
| `list_messages` | List messages in a channel |
| `send_message` | Post a message to a channel (requires TEAMS_MANAGE_ENABLED) |

### Account & System

| Verb | Description |
|------|-------------|
| `account.add` | Add a new Microsoft account |
| `account.list` | List connected accounts |
| `account.login` | Re-authenticate a disconnected account |
| `system.status` | Server health and configuration |

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `TEAMS_MCP_TEAMS_ENABLED` | `false` | Enable Teams read operations |
| `TEAMS_MCP_TEAMS_MANAGE_ENABLED` | `false` | Enable send operations (implies TEAMS_ENABLED) |
| `TEAMS_MCP_AUTH_METHOD` | `device_code` | Auth method: `device_code` or `browser` |
| `TEAMS_MCP_CLIENT_ID` | Office desktop | OAuth client ID |
| `TEAMS_MCP_TENANT_ID` | `common` | Entra ID tenant |
| `TEAMS_MCP_READ_ONLY` | `false` | Block all write operations |
| `TEAMS_MCP_LOG_LEVEL` | `warn` | Log level: debug, info, warn, error |

## OAuth Scopes

| Scope | When |
|-------|------|
| `User.Read` | Always |
| `Team.ReadBasic.All` | Always |
| `Chat.Read` | TEAMS_ENABLED=true |
| `ChannelMessage.Read.All` | TEAMS_ENABLED=true |
| `Chat.ReadWrite` | TEAMS_MANAGE_ENABLED=true (replaces Chat.Read) |

## License

MIT
