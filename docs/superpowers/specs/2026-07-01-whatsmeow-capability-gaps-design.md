# Design: Expose untapped whatsmeow capabilities

**Date:** 2026-07-01
**Status:** Approved (pending spec review)
**whatsmeow:** `v0.0.0-20260622185415-5f04eac6dbbb` (bumped from `563bcaa0f632`)

## Context

The whatsmeow bump (2026-05-25 → 2026-06-22) was a maintenance-only delta — proto
updates, internal refactors, bugfixes — with no new user-facing features and no
breaking changes to gowa's usage. A capability audit (whatsmeow exposes 151 public
`*Client` methods; gowa wires 47) surfaced the real gaps: features whatsmeow has long
supported that gowa never finished wiring. This spec covers four of them, chosen by the
maintainer.

## Goals

Expose four capability groups as REST endpoints, each indistinguishable from existing
gowa code:

1. **Newsletter completion** — fix the follow/unfollow asymmetry; add info + mute.
2. **Quick wins** — block/unblock, set "About", two group admin modes.
3. **Communities** — linked-group (community) management.
4. **Poll voting + results** — vote on polls; forward decrypted results via webhook/SSE.

## Non-goals

- Calls, status/stories posting, contact-QR, presence subscription (deferred gaps).
- `SetGroupDescription` — redundant with the existing `POST /group/topic` (in WhatsApp
  the group "description" is the topic; adding it would create two endpoints fighting
  over one field).
- Poll-result persistence/aggregation — explicitly out of scope (see Feature 4).

## Shared architecture

Every feature follows gowa's existing 4-layer slice. New code matches existing code
exactly:

```
domains/<area>/         interface method + Request/Response structs (json+form tags)
usecase/<area>.go       validate → ValidateJidWithLogin|MustLogin → GetClient().X() → map
ui/rest/<area>.go       InitRest* route registration + handler (BodyParser/QueryParser,
                        helpers.HandleSuccess / HandleError / HandleBadRequest)
validations/<area>_validation.go   ozzo-validation, returns pkgError.ValidationError
```

JID resolution uses `utils.ValidateJidWithLogin(whatsapp.GetClient(), id)` (the same
helper every existing handler uses). Login-only endpoints (no JID arg) use
`utils.MustLogin(whatsapp.GetClient())`. No new usecase wiring in `cmd/root.go` is needed
for features that extend an existing domain (newsletter, user, group, send); only
new route registrations inside the existing `InitRest*` functions.

### Sequencing

F1 → F2 → F3 → F4 (ascending effort; F4 last, only one touching the event pipeline).
Each feature is one commit; `go fmt && go vet && go build ./... && go test ./...` green
between commits.

---

## Feature 1 — Newsletter completion

Extends the existing `domains/newsletter` slice (currently `Unfollow` only).

| Capability | Route | whatsmeow call |
|---|---|---|
| Follow | `POST /newsletter/follow` `{newsletter_id}` | `FollowNewsletter(ctx, jid)` |
| Get info | `GET /newsletter/info?newsletter_id=` | `GetNewsletterInfo(ctx, jid)` |
| Info from invite | `GET /newsletter/info-from-invite?key=` | `GetNewsletterInfoWithInvite(ctx, key)` |
| Mute/unmute | `POST /newsletter/mute` `{newsletter_id, mute:bool}` | `NewsletterToggleMute(ctx, jid, mute)` |

- Info endpoints return `*types.NewsletterMetadata` directly (same shape the existing
  `GET /user/my/newsletters` already returns — `response.Data = append(..., *data)`).
- `info-from-invite` takes a raw invite key/code (string), not a JID — no JID validation.
- New interface methods on `INewsletterUsecase`: `Follow`, `GetInfo`, `GetInfoWithInvite`,
  `ToggleMute`.

## Feature 2 — Quick wins

Split across `domains/user` and `domains/group`.

| Capability | Route | whatsmeow call |
|---|---|---|
| List blocked | `GET /user/blocklist` | `GetBlocklist(ctx)` |
| Block | `POST /user/block` `{phone}` | `UpdateBlocklist(ctx, jid, BlocklistChangeActionBlock)` |
| Unblock | `POST /user/unblock` `{phone}` | `UpdateBlocklist(ctx, jid, BlocklistChangeActionUnblock)` |
| Set "About" | `POST /user/about` `{status}` | `SetStatusMessage(ctx, msg)` |
| Join-approval mode | `POST /group/join-approval` `{group_id, enabled:bool}` | `SetGroupJoinApprovalMode(ctx, jid, mode)` |
| Member-add mode | `POST /group/member-add-mode` `{group_id, mode}` | `SetGroupMemberAddMode(ctx, jid, GroupMemberAddMode)` |

- Block/unblock use distinct verbs (`/block`, `/unblock`) rather than one action param —
  cleaner REST, both call `UpdateBlocklist` with the matching `events.BlocklistChangeAction`
  constant (`"block"` / `"unblock"`). Both return the updated `*types.Blocklist`.
- `member-add-mode` `mode` field validated to one of `admin_add` | `all_member_add`
  (the `types.GroupMemberAddMode` constants).
- Route chosen as `/user/about` (not `/user/status`) to avoid confusion with
  status/stories broadcasts.

## Feature 3 — Communities (linked groups)

Extends `domains/group` (mirrors whatsmeow's own placement of these in `group.go`).
A "community" is a parent group JID with linked child groups.

| Capability | Route | whatsmeow call |
|---|---|---|
| List sub-groups | `GET /group/sub-groups?community_id=` | `GetSubGroups(ctx, jid)` → `[]*types.GroupLinkTarget` |
| Link group | `POST /group/link` `{community_id, group_id}` | `LinkGroup(ctx, parent, child)` |
| Unlink group | `POST /group/unlink` `{community_id, group_id}` | `UnlinkGroup(ctx, parent, child)` |
| Community participants | `GET /group/linked-participants?community_id=` | `GetLinkedGroupsParticipants(ctx, jid)` |

- `link`/`unlink` resolve two JIDs (parent community + child group), both via
  `ValidateJidWithLogin`.
- `sub-groups` returns the `GroupLinkTarget` slice; `linked-participants` returns
  `[]types.JID` rendered as strings.

## Feature 4 — Poll voting + results

Split: a send-side vote endpoint (`domains/send`) and a receive-side decrypt-and-forward
handler (`infrastructure/whatsapp`).

### Vote (send side)

| Capability | Route | whatsmeow call |
|---|---|---|
| Vote on poll | `POST /send/poll/vote` `{phone, poll_message_id, option_names[]}` | `BuildPollVote(ctx, pollInfo, optionNames)` → `SendMessage` |

- `BuildPollVote` needs the original poll's `*types.MessageInfo`. Resolve it from
  chatstorage by `poll_message_id` (the message-ops repo already stores message info),
  constructing a minimal `types.MessageInfo{ID, Chat, Sender, ...}`. If the poll message
  isn't in storage, return a validation error instructing the caller to supply chat/sender.
- `option_names` are the exact option strings the voter selects (empty slice = retract).

### Results (receive side) — forward via webhook/SSE, no persistence

Poll votes already arrive as `events.Message` carrying a `PollUpdateMessage` and already
flow through `handleMessage` → `handleWebhookForward`. The payload is **encrypted** today,
so consumers receive an opaque blob. The change:

1. In `handleMessage` (or a dedicated `handlePollUpdate` called from it), detect
   `evt.Message.GetPollUpdateMessage() != nil`.
2. Call `client.DecryptPollVote(ctx, evt)` → `*waE2E.PollVoteMessage` (returns SHA-256
   hashes of the selected option names).
3. Best-effort resolve hashes back to option text using the original poll's options if
   that poll message is in chatstorage; otherwise forward the raw selected-option hashes.
4. Forward the decrypted vote through the existing webhook path and add an SSE broadcast
   (modeled on `sse.BroadcastReceipt`) so live clients see vote updates.

No new DB table, no aggregation, no vote-change dedup — consumers aggregate downstream.
This matches gowa's existing "decrypt + forward event" pattern.

## Error handling

- All input validation via ozzo `validation.Required` (+ enum `validation.In` for
  `member-add-mode`), returning `pkgError.ValidationError`.
- whatsmeow errors propagate through `helpers.HandleError` unchanged (existing behavior).
- Poll-vote: missing/unknown poll message → `pkgError.ValidationError`, not a 500.
- F4 receive side is best-effort: a decrypt failure logs (matching existing
  `logrus.Error` forwarding pattern) and does not break message handling.

## Testing

- Validation unit tests per feature in `validations/` (the only layer with existing
  table-test coverage — `group_validation_test.go`, `user_privacy_test.go` are the
  templates). Cover required-field and enum cases.
- F4 decrypt/forward: a focused test that a `PollUpdateMessage` is routed to the
  decrypt-and-forward path (mirroring `webhook_forward_test.go`).
- Each feature: `go build ./... && go test ./...` green before its commit.

## Files touched (summary)

- F1: `domains/newsletter/newsletter.go`, `usecase/newsletter.go`,
  `ui/rest/newsletter.go`, `validations/newsletter_validation.go`.
- F2: `domains/user/*`, `usecase/user.go`, `ui/rest/user.go`,
  `validations/user_validation.go`; `domains/group/*`, `usecase/group.go`,
  `ui/rest/group.go`, `validations/group_validation.go`.
- F3: `domains/group/*`, `usecase/group.go`, `ui/rest/group.go`,
  `validations/group_validation.go`.
- F4: `domains/send/*`, `usecase/send.go` (+ `send_*`), `ui/rest/send.go`,
  `validations/send_validation.go`; `infrastructure/whatsapp/handlers.go`,
  `ui/sse/` (new broadcast helper).
- No `cmd/root.go` changes (all extend existing domains).
