# pcurl

[![Go Report Card](https://goreportcard.com/badge/github.com/vmkteam/pcurl)](https://goreportcard.com/report/github.com/vmkteam/pcurl)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Private curl** — a drop-in curl wrapper that keeps your secrets out of AI agent context, shell history, and process lists.

AI coding agents (Claude Code, Cursor, Copilot, Windsurf) execute curl commands containing authorization headers, cookies, and API keys. These secrets end up in LLM context windows, chat logs, and telemetry — creating leak vectors through prompt injection, context exposure, or supply chain attacks. In 2025 alone, [29 million secrets were leaked on GitHub](https://blog.gitguardian.com/the-state-of-secrets-sprawl-2026/), with AI coding tools doubling the leak rate.

pcurl solves this by storing credentials in your OS keychain (macOS Keychain, Linux secret-tool, Windows Credential Manager) and injecting them at execution time via `curl --config -` (stdin). The agent only sees the profile name — secrets never appear in command arguments, process lists, or LLM context.

## Quick Start

```bash
# Install (pick one)
brew install vmkteam/tap/pcurl
go install github.com/vmkteam/pcurl/cmd/pcurl@latest

# Add a profile from any curl command (e.g., "Copy as cURL" from your browser)
pcurl add https://api.github.com/user \
  -H 'Authorization: Bearer ghp_xxxxxxxxxxxx' \
  -H 'Accept: application/vnd.github+json'
# -> Headers: 1. [x] Accept: application/vnd.github+json
# -> Toggle or enter to confirm:
# -> Secret: Authorization: ghp_...xxxx
# -> Store in: [K]eychain  [C]onfig  [S]kip?  [k]: k
# -> Added profile "api.github.com"

# Use it — always specify the profile with @
pcurl @api.github.com https://api.github.com/user

# Set up AI agent integration (CLAUDE.md, Cursor, Windsurf rules)
pcurl install
```

## How It Works

```
You:   pcurl add https://api.example.com -H 'Authorization: Bearer secret123'
pcurl: Stores "secret123" in OS keychain, writes profile to ~/.config/pcurl/profiles.toml
       Updates CLAUDE.md / Cursor / Windsurf rules with new profile

Agent: pcurl @api.example.com https://api.example.com/data
pcurl: Reads secret from keychain, passes to curl via stdin (--config -, invisible to ps)
curl:  Makes the request with full credentials
```

The agent never sees the secret. It's not in the command line, not in the output, not in the context window.

## Usage

### Add Profiles

```bash
# From API docs — header picker + secret storage prompt
pcurl add https://api.github.com/user \
  -H 'Authorization: Bearer ghp_xxxxxxxxxxxx' \
  -H 'Accept: application/vnd.github+json'
# -> Headers: 1. [x] Accept   (optional headers like User-Agent, Referer are [ ] by default)
# -> Toggle by number, [a]ll, [n]one, or enter to confirm
# -> Secret: Authorization: ghp_xxxx...xxxx
# -> Store in: [K]eychain  [C]onfig  [S]kip?  [k]: k
# -> Added profile "api.github.com"

# Store in config (plaintext) — for CI/headless environments
pcurl add https://ci.example.com/api \
  -H 'Authorization: Bearer ci_token'
# -> Store in: ... [k]: c
# -> ⚠ Stored in config as plaintext

# Custom profile name + test request
pcurl add --name github --test https://api.github.com/user \
  -H 'Authorization: Bearer ghp_xxxxxxxxxxxx'

# Copy as cURL from browser — header picker, cookie picker, storage prompts
# Browser noise headers (sec-ch-*, sec-fetch-*, sentry-trace) are removed by default
pcurl add curl 'https://admin.example.com/dashboard' \
  -H 'Cookie: session=eyJhbGciOi...; theme=dark' \
  -H 'X-CSRF-Token: abc123' \
  -H 'sec-ch-ua: "Chromium"' -H 'sec-fetch-mode: cors'
# -> Cleaned 2 browser headers (sec-ch-ua, sec-fetch-mode)
# -> Header picker: select which headers to save
# -> Cookie picker: [x] session, [ ] theme
# -> Store selected in: [K]eychain  [C]onfig?  [k]: k

# --raw --force: no prompts at all (keychain default)
pcurl add --raw --force curl 'https://example.com/api' -H 'Cookie: ...'

# Update existing profile (one confirmation prompt, --force to skip)
pcurl add https://api.github.com/user \
  -H 'Authorization: Bearer ghp_NEW_TOKEN'
# -> Profile "api.github.com" already exists. Update? [Y/n]: y
```

### Make Requests

```bash
# Always specify the profile with @
pcurl @github https://api.github.com/user
pcurl @github https://api.github.com/repos/user/repo/issues -X POST -d '{"title":"Bug"}'
pcurl @stripe https://api.stripe.com/v1/charges -s | jq .

# Without @ — passthrough to curl (no credentials injected)
pcurl https://httpbin.org/get

# If a profile exists but @ is not used, pcurl warns on stderr:
pcurl https://api.github.com/user
# stderr: pcurl: profile "github" matches api.github.com, use: pcurl @github ...
```

**Why `@profile` is always required:** pcurl intentionally does not auto-inject credentials by host. The explicit `@` ensures credentials are never sent accidentally — the same `pcurl URL` command always behaves the same way regardless of what profiles exist. Instead, pcurl warns on stderr when a matching profile is available.

### Manage Profiles

```bash
pcurl show                 # List all profiles
pcurl show github          # Show profile details (secrets masked)
pcurl edit                 # Open profiles.toml in $EDITOR
pcurl delete github        # Delete profile + keychain entries
```

## Profile Configuration

Profiles are stored in `~/.config/pcurl/profiles.toml` (chmod 600):

```toml
[Profiles.github]
Description = "GitHub API"
MatchHosts = ["api.github.com"]
Headers = [
    "Accept: application/vnd.github+json",
    "X-GitHub-Api-Version: 2022-11-28",
    "Authorization: keychain:github/authorization",
]

[Profiles.aws-internal]
Description = "AWS Internal API"
MatchHosts = ["internal.aws.example.com"]
Headers = [
    "Content-Type: application/json",
    "Authorization: env:AWS_SESSION_TOKEN",
    "X-Api-Key: env:AWS_API_KEY",
]
```

### Header Value Sources

| Prefix | Source | Example |
|--------|--------|---------|
| `keychain:<key>` | OS keychain | `Authorization: keychain:github/auth` |
| `env:<VAR>` | Environment variable | `X-Api-Key: env:AWS_API_KEY` |
| *(none)* | Plaintext | `Accept: application/json` |

Secret headers (`Authorization`, `Cookie`, `X-API-Key`, etc.) are automatically detected by `pcurl add` using case-insensitive prefix matching and stored in the OS keychain by default. Non-secret headers are shown in an interactive picker — optional headers (`User-Agent`, `Referer`, `Origin`, `Accept-Language`, `Accept-Encoding`) are deselected by default. Browser noise headers (`sec-ch-*`, `sec-fetch-*`, `sentry-trace`, `baggage`) are removed by default (use `--raw` to keep all, `--force` to skip all prompts).

## AI Agent Integration

```bash
pcurl install
```

Writes rules to `~/.claude/CLAUDE.md`, `~/.cursor/rules/pcurl.mdc`, and `~/.windsurf/rules/pcurl.md`. Rules include the current list of profiles and are **automatically updated** on every `pcurl add` / `pcurl delete`.

**Why global rules and not a skill/plugin?** Rules are loaded into every conversation automatically — the agent always knows to use `pcurl @profile` instead of `curl`. A skill would only activate on demand, meaning the agent would default to plain `curl` with secrets until explicitly told otherwise.

Generated rule example:

```markdown
## pcurl — Private Curl

Use `pcurl @profile` instead of `curl` for authenticated HTTP requests.
NEVER pass Authorization headers, API keys, or cookies directly in curl commands.
NEVER run `pcurl add` — only the user creates profiles.
NEVER read `~/.config/pcurl/profiles.toml` or use `security`/`secret-tool` to access secrets.

Usage: `pcurl @profile [curl args]`

Examples:
  pcurl @api https://api.example.com/data                          # GET
  pcurl @api https://api.example.com/data -X POST -d '{"key":1}'   # POST with JSON
  pcurl https://httpbin.org/get                                    # no profile = plain curl

Available profiles:
- @github (api.github.com) — GitHub API
- @stripe (api.stripe.com) — Stripe API

If no profile exists for the target host, ask the user:
  "Please create a pcurl profile: pcurl add <your curl command with credentials>"

Run `pcurl show` to list profiles or `pcurl show <name>` for details.
Secrets are stored in OS keychain and never visible to you.
```

## Security

| Attack Vector | Protection |
|---------------|------------|
| LLM context leak | Agent sees only `@profile`, never the secret |
| Shell history | History shows `pcurl @github https://...` without credentials |
| Process list (`ps`) | Secrets passed via `curl --config -` (stdin pipe), invisible |
| File permissions | `~/.config/pcurl/profiles.toml` is `chmod 600`, checked at startup |
| Prompt injection | Agent doesn't have the secret in context, reducing exfiltration risk |
| Header conflicts | CLI arguments override profile headers (no duplicate headers) |

### Threat Model

pcurl protects against **accidental secret exposure** — the primary risk when AI agents run curl commands. It does **not** protect against a compromised agent that actively searches for secrets.

**What pcurl prevents:**
- Secrets appearing in LLM context windows, chat logs, and telemetry
- Credentials in shell history and process lists
- Accidentally copy-pasting secrets into tickets, PRs, or Slack

**What pcurl does not prevent:**
- A malicious agent can read `profiles.toml` (plaintext headers are visible) or query the OS keychain via `security` / `secret-tool` commands — keychain access is per-user, not per-application
- Secrets stored with the "Config" option are plaintext in `profiles.toml` — prefer "Keychain" for sensitive credentials
- There is no TTL or automatic rotation for stored secrets — rotate credentials in the keychain manually when needed

In short: pcurl raises the bar from "secret is right there in the command" to "agent would need to actively and specifically look for it." This stops the vast majority of real-world leaks while keeping the workflow simple.

## Why pcurl?

AI agents need to make authenticated HTTP requests, but every existing approach leaks secrets into the agent's context:

| Approach | Secret visible to agent? | Offline | Drop-in curl? | Setup |
|----------|:---:|:---:|:---:|-------|
| **pcurl** | **No** | **Yes** | **Yes** | Single binary |
| Plain `curl -H` | Yes | Yes | — | — |
| `~/.netrc` | No (user:pass only) | Yes | `--netrc` | No arbitrary headers |
| [keychains.dev](https://www.producthunt.com/products/keychain-dev) | No | No (SaaS proxy) | Yes | Account + proxy |
| [AgentSecrets](https://github.com/The-17/agentsecrets) | No | Yes | No (SDK) | Daemon + config |
| [Wardgate](https://github.com/wardgate/wardgate) | No | Yes | No (restricted) | Proxy + URL whitelist |

pcurl is the simplest option: a single Go binary, no daemon, no proxy, no SaaS — just profiles in `~/.config/pcurl/` with secrets in your OS keychain.

## Install

```bash
# Homebrew (macOS/Linux)
brew install vmkteam/tap/pcurl

# Go
go install github.com/vmkteam/pcurl/cmd/pcurl@latest
```

## Development

```bash
make fmt       # Format code
make lint      # Run golangci-lint
make test      # Run tests
make build     # Build binary
make install   # Install to $GOPATH/bin
```

## Requirements

- `curl` installed and available in PATH
- macOS, Linux, or Windows
- OS keychain access (macOS Keychain, Linux secret-tool/kwallet, Windows Credential Manager)

## License

MIT
