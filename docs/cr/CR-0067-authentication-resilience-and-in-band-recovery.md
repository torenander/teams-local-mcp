---
name: authentication-resilience-and-in-band-recovery
description: Port the proven authentication fixes from outlook-local-mcp CR-0067 into teams-local-mcp — silent token refresh before any interactive flow, silent-only credentials that make that refresh safe, recovery verbs that stay reachable and responsive during a sign-in, bounded background auth contexts, and recovery guidance that names verbs which actually exist. The inferred default stays device_code.
id: "CR-0067"
status: "proposed"
date: 2026-09-02
requestor: tor
stakeholders: LLM consumers of the MCP server, headless and unattended deployments
source-branch: feat/auth-resilience
base-commit: 9799e55
ported-from: outlook-local-mcp CR-0067, branch feat/cr-0067-auth-resilience, head 4ca720a
---

# Authentication resilience and in-band recovery

## Change Summary

`teams-local-mcp` is an architecture clone of `outlook-local-mcp`, and `internal/auth`
was cloned before the latter's CR-0067 landed. Every defect that CR fixed is present
here, verbatim. This change ports the fixes.

Concretely: the middleware now tries a silent token refresh before starting any
user-visible flow; both azidentity credentials are constructed silent-only so that
refresh is possible at all and the Graph SDK can no longer open a browser window
mid-tool-call; the `account` verbs stay reachable *and responsive* while a sign-in is
outstanding; both background authentication contexts are bounded; the pending-auth
bookkeeping is race-free; the recovery guidance names verbs that exist; and the device
code prompt is presented as a direct sign-in link plus the code, with an
acknowledgement that waits for the flow and retries the original call.

**The inferred default remains `device_code`.** See [Not ported](#not-ported) — the
upstream project tried `auth_code`, tested it live, and reverted it.

## Numbering

This document reuses the upstream identifier `CR-0067`, deliberately.

This repository has no `docs/` tree and no prior CR documents — this is the first.
Its source *comments*, however, already reference CR-0022, CR-0024, CR-0037, CR-0038,
CR-0049, CR-0051, CR-0052, CR-0056, CR-0058, CR-0060, CR-0061, CR-0064 and CR-0065,
all inherited from `outlook-local-mcp`. That namespace is therefore already shared.
Keeping the same number means an inline `CR-0067 A4` comment reads correctly in either
repository, and a reader can go straight to the upstream validation report for the
live evidence behind each decision. Inventing a parallel `CR-0001` would have made the
inherited references incoherent.

## Motivation

Each defect was verified against `main` @ `9799e55` before being fixed.

| # | Defect on `main` @ `9799e55` | Location |
|---|---|---|
| 1 | The middleware never asks the token cache before asking the user. `handleAuthError` dispatches straight to an interactive flow. | `internal/auth/middleware.go` |
| 2 | Credentials are built without `DisableAutomaticAuthentication`, so the Graph SDK's bearer-token policy can open a browser or emit a device code mid-request, outside middleware control. | `internal/auth/auth.go` |
| 3 | The startup probe skips `device_code` entirely and guesses readiness from an `os.Stat` of the auth record. | `cmd/teams-local-mcp/main.go` |
| 4 | While a flow is pending, every `authMW`-wrapped verb — including `account.login`, `account.list`, `account.refresh` — is answered "Authentication is still in progress". | `internal/auth/middleware.go` |
| 5 | Both background auth goroutines run on an unbounded `context.Background()`, so an abandoned sign-in freezes the session for the process lifetime. | `internal/auth/middleware.go` (2 sites) |
| 6 | `pendingDone` / `pendingErr` are written by the background goroutine and read unsynchronised at middleware entry: a data race. | `internal/auth/middleware.go` |
| 7 | `FormatAuthError` tells the LLM to "call `account_list`, then `account_add`". `account.add` registers a *new* account, so following the advice creates duplicates — and both names are pre-CR-0060 flat tool names this server no longer exposes. | `internal/auth/errors.go` |
| 8 | `classifyAuthError` tests the bare string `authentication required` before the generic branch, so specific detail is swallowed. | `internal/auth/errors.go` |

Defect 4 is worse than it looks. Fixing it by exemption alone reproduces a hang, for
the reason recorded under [A4](#a4--recovery-verbs-stay-reachable-and-responsive).

## Ported

| ID | Change | Status |
|---|---|---|
| A1 | Silent token refresh before any interactive flow, on both the fresh-credential fast path and the auth-error path. | Ported |
| A2 | *(withdrawn upstream)* Inferring `auth_code` as the default. | **Not ported** — see below |
| A3 | Always register `system.complete_auth`. | **Not applicable** — see below |
| A4 | Recovery verbs stay reachable and responsive; bounded background contexts; race-free pending state. | Ported |
| A5 | Correct, method-aware recovery guidance; preserve explanatory detail. | Ported |
| A6 | Hand the resolved account back to the middleware through a mutable context slot. | **Not ported** — see below |
| A7 | Device code presented as a direct sign-in link plus the code; acknowledgement waits and retries. | Ported |

### A1 — silent refresh, and the flag that makes it safe

`internal/auth/silent.go` adds `TrySilentToken`, called at two sites in
`internal/auth/middleware.go`: the fresh-credential fast path and the top of
`handleAuthError`. On success the original tool call is retried and the user sees
nothing.

This is only safe because of the second half. azidentity's `publicClient.GetToken`
tries `AcquireTokenSilent` and then **falls through to `reqToken`** — an interactive
browser window for `InteractiveBrowserCredential`, a fresh device code for
`DeviceCodeCredential`. Verified first-hand in this environment against
`azidentity@v1.13.1`, `public_client.go:135-156`. Probing such a credential
speculatively would emit an unusable device code on every tool call.

`DisableAutomaticAuthentication: true` suppresses exactly that: `GetToken` returns
`azidentity.AuthenticationRequiredError` instead of calling `reqToken`. It is declared
on **both** `InteractiveBrowserCredentialOptions:44-47` (wired at `:93`) and
`DeviceCodeCredentialOptions:45-48` (wired at `:113`) — confirmed by an untruncated
grep of the module cache, because the upstream project's first attempt at this check
was piped through `head` and produced a confident wrong answer.

`Authenticate()` calls `reqToken` directly and never reads the option
(`public_client.go:110-132`), so the deliberate interactive flows are unaffected.

Eligibility is an explicit allowlist (`silentOnlyAcquirer`), not an auth-method
string: `*AuthCodeCredential` (which declares `SilentOnly()` natively) plus the two
azidentity types this package constructs with the flag. Anything else is skipped, so a
credential added later without the flag fails safe rather than surprising the user.

With `GetToken` silent-only, the startup probe in `cmd/teams-local-mcp/main.go` no
longer needs its `device_code` special case; `preAuthenticated` now reflects a real
token check rather than the presence of a file, and the `authRecordPath` parameter is
dropped.

### A4 — recovery verbs stay reachable and responsive

Two files: `internal/auth/recovery_ops.go` classifies the `account` aggregate tool as
recovery surface, and `internal/auth/pending.go` makes the background attempt's
outcome publishable without a data race and bounds both flows at 300s.

**The exemption alone is not enough, and this is the subtle part.** azidentity's
`publicClient.client()` returns a shared `*sync.Mutex` (`caeMu` or `noCAEMu`), and
*both* `Authenticate` and `GetToken` take it (`public_client.go:120-124` and
`:145-147`). An interactive `Authenticate` holds it for the entire sign-in. It is a
plain `sync.Mutex`, so **it is not context-aware**: passing a short context to a Graph
call does not bound the wait. Exempting the `account` verbs therefore lets them reach
their handlers, where they then block on the credential — a hang, which is strictly
worse than the pending message it replaced.

This was reproduced here, not taken on trust. `TestRecoveryVerbsStayResponsiveDuringAuth`
drives the real `AuthMiddleware` against a credential that models the shared
non-context-aware mutex. With the guard removed it fails in exactly the upstream
shape:

```
--- FAIL: TestRecoveryVerbsStayResponsiveDuringAuth (5.00s)
    account verb HUNG while a sign-in was pending (innerRan=true, getTokenCalls=3)
```

`innerRan=true` is the important part: the exemption matched and the handler ran. The
hang is downstream.

**Fix.** Bounding is unavailable — abandoning a goroutine blocked on a
non-context-aware mutex leaves it to write `entry.Email` later, racing every reader.
So the call is not made while the lock is held. `internal/auth/inflight.go` counts
running interactive flows; the middleware brackets both background goroutines with
`BeginInteractiveAuth`/`EndInteractiveAuth`, and the two blocking call sites in this
repository consult it:

| Site | Behaviour during a sign-in | Rationale |
|---|---|---|
| `auth.EnsureEmail` (reached by `account.list`, `account.add`, `account.login`) | skips | The address only enriches a display string and resolves on a later call. |
| `tools.HandleRefreshAccount` (`account.refresh`) | declines with an explanation | A refresh is pointless while the flow about to mint a fresh token is still running. |

`account.login` needs no guard: `setupCredential` builds a *new* credential with its
own mutex.

The signal is process-wide rather than per-credential. During a sign-in the user is
already being prompted, so briefly degrading best-effort enrichment for every account
is a good trade against ever hanging a recovery verb.

### A5 — recovery guidance

`FormatAuthErrorFor` and `recoverySteps` replace the single hardcoded block. Every
variant starts from `account operation="list"` and re-authenticates with
`account operation="login"`; `operation="add"` appears only in the unknown-method
variant, and only as the "not registered at all" case.

`authRequiredDetail` replaces the substring test in `classifyAuthError`, so
`"authentication required: device code prompt was not received from Entra ID"` keeps
its detail instead of degrading to the contentless bare message. A new branch
recognises `azidentity.AuthenticationRequiredError` and suppresses its SDK-level
"Call Authenticate to authenticate a user interactively" advice, which the LLM cannot
act on.

The upstream `auth_code` variant additionally names `system operation="complete_auth"`.
This server does not register that verb, so the wording here stops at the in-band
paste prompt. A test asserts no guidance mentions `complete_auth`.

### A7 — device code sign-in link

Ported because the precondition in the brief is met: this repository already has URL
elicitation plumbing (`urlElicitFunc` / `defaultURLElicit` in `middleware.go`,
`RequestURLElicitation` in `add_account.go`).

`DeviceCodeMsgKey` now carries a structured `chan auth.DeviceCodePrompt` rather than
`chan string`, so receivers can build a deep link and quote the code separately.
`presentDeviceCode` moves out of `middleware.go` into `devicecode_present.go` and
switches from form-mode elicitation to URL mode.

**What the link does and does not do.** Upstream verified live on 2026-09-02 that
`https://login.microsoft.com/device?otc=<code>` preserves the `otc` parameter through
the redirect to the `deviceauth` page — but the page still renders with the Code field
**empty**. Microsoft does not pre-fill it. The link removes the navigation step only.
The elicitation message therefore quotes the code, and
`TestDeviceCodeElicitMessage_QuotesTheCode` asserts both that the code is present and
that no string claims pre-filling. That defect is the reason the assertion exists: an
earlier upstream revision omitted the code on the false premise that it was
pre-filled, so clients that *do* support URL elicitation would have shown a link and
nothing to type.

The valuable half of A7 is not the link. It is that acknowledging the elicitation now
waits for the background flow and **retries the original tool call**, so the sign-in
completes the user's actual request. The plain-text fallback is unchanged and
unaugmented: the Entra ID message verbatim, which already names both the page and the
code. Because clients such as Claude Code answer elicitation with "Method not found",
that fallback is the primary surface, not a degraded one.

## Not ported

### A2 — inferring `auth_code` as the default: do not retry this

`InferAuthMethod` is unchanged. Upstream implemented this, drove it against a real
Microsoft 365 account on 2026-09-02, and reverted it. The `nativeclient` redirect page
now renders an anti-phishing interstitial —

> "This page is not normally shown and could be a sign of a phishing attempt. The URL
> contains your password. Close this page immediately and do not copy or share the URL
> with anyone."

— followed by "You have reached the wrong page." The flow does not complete. This is
not a transient bug: copying an authorization code out of the address bar is
behaviourally identical to the phishing technique the interstitial exists to prevent.

`auth_code` remains fully implemented and selectable with
`TEAMS_MCP_AUTH_METHOD=auth_code`. Only the inference is unchanged.

The rationale is now recorded in the `InferAuthMethod` doc comment so the decision is
not silently re-litigated.

### A3 — `system.complete_auth`: not applicable

This server has no `complete_auth` verb. `buildSystemVerbs` registers only `help` and
`status`, and there is no `internal/tools/complete_auth.go`. There is therefore nothing
to make unconditional.

**This surfaces a pre-existing defect, left unfixed and recorded here.** Two strings in
this repository already tell the user to call a verb that does not exist:

- `internal/auth/middleware.go:~380` — the `auth_code` elicitation fallback: *"…call
  the complete_auth tool with the redirect_url parameter to finish authentication."*
- `internal/tools/add_account.go:~506` — the same advice from `add_account`.

Both are reachable only under `TEAMS_MCP_AUTH_METHOD=auth_code`, which is not the
default, and fixing them properly means either porting the verb or rewording two
messages. Both are out of scope for this change. Recorded as a follow-up.

### A6 — per-account re-authentication slot

Not ported. The bug is present: `authMW` wraps *outside* `accountResolverMW` (see
`internal/server/server.go` and the `wrap` helpers in `chat_verbs.go` /
`teams_verbs.go`), so the `AccountAuthFromContext` lookup in `handleAuthError` can
never see a value the resolver injected into a context derived inside the call.
Re-authentication therefore always uses the default closure credential, whichever
account the tool call targeted.

Left out because it was outside this change's brief and is independent of the five
items requested. It matters only for multi-account setups. Recorded as a follow-up.

## Open question: which client ID, and can `browser` work here?

This needs a live test, which this change does not attempt.

**Finding: the shipped default is the Microsoft first-party app, not a custom
registration.** `internal/config/config.go:212` reads:

```go
ClientID: ResolveClientID(GetEnv("TEAMS_MCP_CLIENT_ID", "outlook-desktop")),
```

`outlook-desktop` resolves to `d3590ed6-52b3-4102-aeff-aad2292ab01c` — the same
Microsoft Office application `outlook-local-mcp` ships. Commit `9799e55` moved
deliberately *to* it, from `m365-desktop`, with the reasoning that the first-party app
has broad implicit Microsoft 365 access including Teams, while explicit Teams scopes
require admin consent and break device code flow. Commit `66534a2` records that
`teams-desktop` (`1fec8e78`) does not support device code flow at all.

**Consequence: upstream's `AADSTS50011` finding transfers directly.** It was gathered
against this exact application ID. `browser` cannot be the default here for the same
reason it cannot be there: `InteractiveBrowserCredential` binds a random localhost
port and none of them is registered on `d3590ed6`.

**What remains open.** `WellKnownClientIDs` also carries
`"teams-local-mcp": "dd5fc5c5-eb9a-4f6f-97bd-1a9fecb277d3"`. That is not a Microsoft
first-party ID and looks like a custom app registration, which normally *does* register
a localhost redirect URI. If it does, `browser` would work for that client ID — and
`browser` is the one flow that completes with no human transcription step at all.

Two things block a conclusion:

1. It is currently unreachable by inference. `InferAuthMethod` matches *any* UUID in
   `WellKnownClientIDs` and returns `device_code`, so `teams-local-mcp` is treated as
   a first-party app even though it is not. Selecting `browser` for it requires
   `TEAMS_MCP_AUTH_METHOD=browser` explicitly.
2. Nobody has run a sign-in against it.

**Proposed test — and the wrong way to run it.**

> **Do not test this by fetching the authorize endpoint.** Entra ID defers
> `redirect_uri` validation until *after* authentication. A `curl` against
> `/authorize` with `redirect_uri=http://localhost:12345` returns a rendered Microsoft
> login page and no error whether or not the URI is registered. Upstream made exactly
> this mistake and recorded it as a methodology caution. The rendered page is a false
> negative.

The only reliable test is an end-to-end sign-in:

```bash
TEAMS_MCP_CLIENT_ID=teams-local-mcp \
TEAMS_MCP_AUTH_METHOD=browser \
TEAMS_MCP_TEAMS_ENABLED=true \
  teams-local-mcp --stdio
```

Then issue any `chat` verb and complete the browser sign-in that opens.

| Outcome | Meaning | Follow-up |
|---|---|---|
| Sign-in completes, tool call returns data | `dd5fc5c5` registers a localhost redirect URI and has the Teams scopes | Worth a follow-up CR: make `InferAuthMethod` return `browser` for `teams-local-mcp` specifically, giving a hands-free default that `outlook-local-mcp` cannot have |
| `AADSTS50011` | No localhost redirect URI registered | `device_code` stands; record it here alongside the `d3590ed6` evidence |
| `AADSTS65001` / consent error | Redirect URI is fine but scopes are not consented | Separate problem; note which scopes |
| `AADSTS700016` (application not found) | The ID is not a real registration in the signing-in tenant | Remove it from `WellKnownClientIDs` |

Worth checking in the same session: whether `dd5fc5c5` accepts the
`Calendars.ReadWrite` / `Mail.Read` scope set this server currently requests, since
those scopes exist only because `d3590ed6` has implicit Teams access (see
`internal/auth/auth.go`). A custom registration would more naturally use `Chat.Read`
and `Team.ReadBasic.All`.

## Affected components

| Component | Change |
|---|---|
| `internal/auth/silent.go` *(new)* | `SilentTokenCredential`, `silentOnlyAcquirer`, `TrySilentToken` |
| `internal/auth/pending.go` *(new)* | `pendingAuthAttempt`, race-free publication, `backgroundAuthTimeout` |
| `internal/auth/recovery_ops.go` *(new)* | `isRecoveryOperation` |
| `internal/auth/inflight.go` *(new)* | Interactive-auth in-flight counter |
| `internal/auth/devicecode_prompt.go` *(new)* | `DeviceCodePrompt`, `SignInURL` |
| `internal/auth/devicecode_present.go` *(new)* | URL-mode presentation, ack → wait → retry, verbatim fallback |
| `internal/auth/middleware.go` | Entry point extracted to `(*authMiddlewareState).wrap`; recovery exemption; silent refresh at both sites; race-free pending; in-flight bracketing; bounded contexts; `presentDeviceCode` moved out |
| `internal/auth/auth.go` | `DisableAutomaticAuthentication: true` on both azidentity credentials; `DeviceCodeMsgKey` element type |
| `internal/auth/authcode.go` | `SilentOnly()` marker |
| `internal/auth/errors.go` | `FormatAuthErrorFor`, `recoverySteps`, `authRequiredDetail`, `AuthenticationRequiredError` branch, reordered classification |
| `internal/auth/email_resolver.go` | `EnsureEmail` skips during a sign-in |
| `internal/tools/refresh_account.go` | Declines during a sign-in; QF1012 cleanup |
| `internal/tools/add_account.go` | Device code channel element type |
| `internal/config/config.go` | `InferAuthMethod` doc comment records the rejected alternatives (behaviour unchanged) |
| `cmd/teams-local-mcp/main.go` | Startup probe runs for every method; `authRecordPath` parameter dropped |
| `README.md` | Authentication section brought in line |

`internal/auth` had **no tests** before this change. Nine test files are added.

## Testing

| File | Covers |
|---|---|
| `internal/auth/cr0067_helpers_test.go` | Shared fixtures |
| `internal/auth/silent_test.go` | Eligibility allowlist; escalating credential never probed; `SetupCredential` yields `AuthenticationRequiredError` on both azidentity methods |
| `internal/auth/recovery_reachable_test.go` | The A4 regression suite — **verified to fail against the unfixed code** |
| `internal/auth/recovery_ops_test.go` | Domain classification |
| `internal/auth/middleware_cr0067_test.go` | Pending gate vs. recovery exemption; silent refresh on both paths; `pendingOutcome`; both background contexts bounded |
| `internal/auth/devicecode_prompt_test.go` | `SignInURL` cases; the message quotes the code and never claims pre-filling |
| `internal/auth/errors_cr0067_test.go` | Method-specific guidance; detail preservation; `authRequiredDetail` |
| `internal/tools/refresh_account_test.go` | `account.refresh` declines promptly and never calls `GetToken` during a sign-in |
| `internal/config/config_cr0067_test.go` | `InferAuthMethod` defaults pinned; shipped client ID pinned |
| `cmd/teams-local-mcp/main_cr0067_test.go` | Startup probe runs for `device_code` without prompting |

Only `TestRecoveryVerbsStayResponsiveDuringAuth` was mutation-checked against the
unfixed code. The rest were written alongside their implementations.

`TestSetupCredential_GetTokenIsSilentOnly` is the behavioural pin for
`DisableAutomaticAuthentication`: azidentity produces `AuthenticationRequiredError`
*only* on that branch, so removing the option from either credential fails the test.

## Verification tier

Everything in this change is **unit-tested only**. Nothing has been exercised against
a real Microsoft 365 account in this repository.

The upstream equivalents of A1, A4 and A7 were verified live on 2026-09-02 against a
real account (silent refresh retried a cold `device_code` call in 143 ms with no code
emitted; `account.list` immediate and `account.refresh` declining, both of which
previously hung past 60 s). Those results are strong evidence for the *design*, and
the code here is a close port, but they are not evidence about this binary.

The A4 mutation check is the one place where the defect itself was reproduced in this
repository.

## Risks

**The silent refresh adds latency to the auth-error path.** Bounded at 5 s, attempted
only for credentials that cannot escalate. On a miss with an empty cache it returns
without touching the network. The alternative it replaces is a full interactive
sign-in.

**`DisableAutomaticAuthentication` changes Graph SDK behaviour for existing users.**
Previously the bearer-token policy could open a browser window or emit a device code
in the middle of an unrelated tool call. Now it returns an error the middleware turns
into a coordinated prompt. Strict improvement, no re-authentication required.

**An exempted verb acquires a new blocking dependency.** The exemption's value depends
on the handler returning promptly, which the middleware cannot enforce.
`TestRecoveryVerbsStayResponsiveDuringAuth` drives the real middleware against a
credential modelling the shared mutex and asserts a two-second bound, so a future
handler that starts touching the credential fails the build.

**Migration cost: none.** The inferred default is unchanged, so no installation is
asked to re-authenticate and every cached token keeps working.

## Follow-ups

1. Run the `browser` / `dd5fc5c5` sign-in test above and record the outcome here.
2. Port A6 (per-account re-authentication slot) — matters for multi-account setups.
3. Decide the `complete_auth` question: either port the verb (upstream A3) or reword
   the two strings that name it.
4. `internal/auth/filecache.go:35` — `cae` field unused (pre-existing lint finding,
   untouched).
5. 25 files in this repository are not `gofmt`-clean (import ordering and map
   alignment), pre-existing. Only files touched by this change were formatted. A
   repo-wide `gofmt -w .` is a separate mechanical change.

## Origin

Ported from `outlook-local-mcp`, branch `feat/cr-0067-auth-resilience`, head
`4ca720a`. The upstream CR and its validation report — including the three disproved
assumptions and the two methodology cautions — remain the authoritative record of why
each decision was made:

- `docs/cr/CR-0067-authentication-resilience-and-in-band-recovery.md`
- `docs/cr/CR-0067-validation-report.md`
