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

On first use, the server prompts you to authenticate via device code flow: open the
sign-in link it returns and enter the code shown. Microsoft does not pre-fill the code
on that page, so you do type it.

After that first sign-in the token is cached. Subsequent expiries are handled by a
silent refresh, so you should not be prompted again unless the refresh token itself
expires or is revoked.

If a tool call ever reports that authentication is required:

1. Call `account` with `operation="list"` to see which account is disconnected.
2. Call `account` with `operation="login"` and that account's label.
3. Retry your original request.

The `account` verbs stay reachable while a sign-in is outstanding, so you can always
inspect and repair account state — even mid-prompt.

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
| `account.remove` | Remove an account and clear its tokens |
| `account.list` | List connected accounts |
| `account.login` | Re-authenticate a disconnected account |
| `account.logout` | Disconnect an account without removing it |
| `account.refresh` | Force a token refresh; returns the new expiry |
| `system.status` | Server health and configuration |

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `TEAMS_MCP_TEAMS_ENABLED` | `false` | Enable Teams read operations |
| `TEAMS_MCP_TEAMS_MANAGE_ENABLED` | `false` | Enable send operations (implies TEAMS_ENABLED) |
| `TEAMS_MCP_AUTH_METHOD` | inferred | Auth method: `device_code`, `browser` or `auth_code`. Unset, it is inferred from the client ID (see below) |
| `TEAMS_MCP_CLIENT_ID` | `outlook-desktop` | OAuth client ID: a well-known name or a UUID |
| `TEAMS_MCP_TENANT_ID` | `common` | Entra ID tenant |
| `TEAMS_MCP_READ_ONLY` | `false` | Block all write operations |
| `TEAMS_MCP_LOG_LEVEL` | `warn` | Log level: debug, info, warn, error |

## Authentication methods

`TEAMS_MCP_AUTH_METHOD` is inferred when unset:

| Client ID | Inferred method | Source |
|-----------|-----------------|--------|
| Any name in the well-known table (including the default `outlook-desktop`) | `device_code` | `inferred` |
| Any other UUID | `browser` | `default` |

An explicit `TEAMS_MCP_AUTH_METHOD` always wins.

**Why `device_code` is the default.** The shipped client ID is the Microsoft Office
first-party application `d3590ed6-52b3-4102-aeff-aad2292ab01c`, which has broad
implicit Microsoft 365 access including Teams. Neither alternative works against it:

- `browser` fails with `AADSTS50011` — `InteractiveBrowserCredential` binds a random
  localhost port and none is registered on that application.
- `auth_code` is blocked by a Microsoft anti-phishing interstitial on the
  `nativeclient` redirect page, so the flow does not complete.

Both were tested against a live account on 2026-09-02. See
`docs/cr/CR-0067-authentication-resilience-and-in-band-recovery.md`.

Operators who need a hands-free sign-in should register their own application with an
`http://localhost` redirect URI and set `TEAMS_MCP_CLIENT_ID` to its UUID, which routes
them to `browser` automatically.

**Token behaviour.** Both azidentity credentials are constructed with
`DisableAutomaticAuthentication`, so the Graph SDK can never open a browser window or
emit a device code in the middle of an unrelated tool call. Every prompt comes from the
auth middleware, and only after a silent cache refresh has been tried and failed.

## OAuth Scopes

Requested against the first-party client ID, which has pre-consented Teams access.
Explicit Teams scopes (`Chat.Read`, `Team.ReadBasic.All`) require admin consent and
break the device code flow, so they are not used.

| Scope | When |
|-------|------|
| `Calendars.ReadWrite` | Always |
| `Mail.Read` | `TEAMS_MCP_TEAMS_ENABLED=true` |
| `Mail.ReadWrite` | `TEAMS_MCP_TEAMS_MANAGE_ENABLED=true` (replaces `Mail.Read`) |

## License

MIT
