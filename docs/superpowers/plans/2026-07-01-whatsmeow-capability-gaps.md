# whatsmeow Capability Gaps Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose four untapped whatsmeow capabilities — newsletter completion, quick wins (block/about/group-modes), communities, and poll voting+results — as REST endpoints that match gowa's existing code exactly.

**Architecture:** Each capability is the same 4-layer slice gowa already uses: `domains/<area>` (interface + request/response structs) → `usecase/<area>.go` (validate → `ValidateJidWithLogin`/`MustLogin` → `whatsapp.GetClient().X()` → map) → `ui/rest/<area>.go` (`InitRest*` route + handler using `helpers.*`) → `validations/<area>_validation.go` (ozzo). Feature 4 additionally adds a receive-side handler in `infrastructure/whatsapp` that decrypts poll votes and forwards them through the existing webhook + SSE pipeline.

**Tech Stack:** Go 1.25, Fiber v2, ozzo-validation v4, go.mau.fi/whatsmeow, logrus.

## Global Constraints

- Go module dir is `src/`. All commands run from `src/`.
- whatsmeow pinned at `v0.0.0-20260622185415-5f04eac6dbbb` — do not bump further in this work.
- Between every commit, this MUST be green: `cd src && go fmt ./... && go vet ./... && go build ./... && go test ./...`
- Conventional-commit messages. End each commit body with: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`
- JID resolution: `utils.ValidateJidWithLogin(whatsapp.GetClient(), id)`. Login-only (no JID): `utils.MustLogin(whatsapp.GetClient())`. Phone inputs: `utils.SanitizePhone(&request.Phone)` in the controller before calling the service (mirrors existing user handlers).
- Validation errors: return `pkgError.ValidationError(err.Error())`. Never return a raw 500 for bad input.
- Work on branch `feat/whatsmeow-capability-gaps` (already created; bump + spec already committed there).
- The only package with existing unit tests is `validations/` (see `group_validation_test.go`, `user_privacy_test.go`). TDD is anchored there; usecase/rest/handler layers are verified by `go build` + `go vet` because they require a live WhatsApp client.

---

### Task 1: Newsletter completion (Feature 1)

Extend the existing `newsletter` slice (currently `Unfollow` only) with follow, get-info, info-from-invite, and mute.

**Files:**
- Modify: `src/domains/newsletter/newsletter.go`
- Modify: `src/usecase/newsletter.go`
- Modify: `src/ui/rest/newsletter.go`
- Modify: `src/validations/newsletter_validation.go`
- Test: `src/validations/newsletter_validation_test.go` (create)

**Interfaces:**
- Consumes: `utils.ValidateJidWithLogin`, `utils.MustLogin`, `whatsapp.GetClient()`, `helpers.*`, `pkgError.ValidationError`.
- Produces: `INewsletterUsecase` gains `Follow`, `GetInfo`, `GetInfoWithInvite`, `ToggleMute`. Request structs: `FollowRequest{NewsletterID}`, `GetInfoRequest{NewsletterID}`, `GetInfoWithInviteRequest{Key}`, `ToggleMuteRequest{NewsletterID, Mute}`.

- [ ] **Step 1: Write the failing validation test**

Create `src/validations/newsletter_validation_test.go`:

```go
package validations

import (
	"context"
	"testing"

	domainNewsletter "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/newsletter"
)

func TestValidateFollowNewsletter(t *testing.T) {
	if err := ValidateFollowNewsletter(context.Background(), domainNewsletter.FollowRequest{}); err == nil {
		t.Error("expected error for empty newsletter_id, got nil")
	}
	if err := ValidateFollowNewsletter(context.Background(), domainNewsletter.FollowRequest{NewsletterID: "123@newsletter"}); err != nil {
		t.Errorf("expected no error for valid newsletter_id, got %v", err)
	}
}

func TestValidateGetNewsletterInfo(t *testing.T) {
	if err := ValidateGetNewsletterInfo(context.Background(), domainNewsletter.GetInfoRequest{}); err == nil {
		t.Error("expected error for empty newsletter_id, got nil")
	}
}

func TestValidateGetNewsletterInfoWithInvite(t *testing.T) {
	if err := ValidateGetNewsletterInfoWithInvite(context.Background(), domainNewsletter.GetInfoWithInviteRequest{}); err == nil {
		t.Error("expected error for empty key, got nil")
	}
}

func TestValidateNewsletterMute(t *testing.T) {
	if err := ValidateNewsletterMute(context.Background(), domainNewsletter.ToggleMuteRequest{}); err == nil {
		t.Error("expected error for empty newsletter_id, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails (compile error)**

Run: `cd src && go test ./validations/ -run TestValidate.*Newsletter -v`
Expected: FAIL — undefined: `ValidateFollowNewsletter`, `domainNewsletter.FollowRequest`, etc.

- [ ] **Step 3: Add domain structs + interface methods**

In `src/domains/newsletter/newsletter.go`, replace the file body with:

```go
package newsletter

import (
	"context"

	"go.mau.fi/whatsmeow/types"
)

type INewsletterUsecase interface {
	Unfollow(ctx context.Context, request UnfollowRequest) (err error)
	Follow(ctx context.Context, request FollowRequest) (err error)
	GetInfo(ctx context.Context, request GetInfoRequest) (response *types.NewsletterMetadata, err error)
	GetInfoWithInvite(ctx context.Context, request GetInfoWithInviteRequest) (response *types.NewsletterMetadata, err error)
	ToggleMute(ctx context.Context, request ToggleMuteRequest) (err error)
}

type UnfollowRequest struct {
	NewsletterID string `json:"newsletter_id" form:"newsletter_id"`
}

type FollowRequest struct {
	NewsletterID string `json:"newsletter_id" form:"newsletter_id"`
}

type GetInfoRequest struct {
	NewsletterID string `json:"newsletter_id" form:"newsletter_id" query:"newsletter_id"`
}

type GetInfoWithInviteRequest struct {
	Key string `json:"key" form:"key" query:"key"`
}

type ToggleMuteRequest struct {
	NewsletterID string `json:"newsletter_id" form:"newsletter_id"`
	Mute         bool   `json:"mute" form:"mute"`
}
```

- [ ] **Step 4: Add validation functions**

Append to `src/validations/newsletter_validation.go`:

```go
func ValidateFollowNewsletter(ctx context.Context, request domainNewsletter.FollowRequest) error {
	err := validation.ValidateStructWithContext(ctx, &request,
		validation.Field(&request.NewsletterID, validation.Required),
	)
	if err != nil {
		return pkgError.ValidationError(err.Error())
	}
	return nil
}

func ValidateGetNewsletterInfo(ctx context.Context, request domainNewsletter.GetInfoRequest) error {
	err := validation.ValidateStructWithContext(ctx, &request,
		validation.Field(&request.NewsletterID, validation.Required),
	)
	if err != nil {
		return pkgError.ValidationError(err.Error())
	}
	return nil
}

func ValidateGetNewsletterInfoWithInvite(ctx context.Context, request domainNewsletter.GetInfoWithInviteRequest) error {
	err := validation.ValidateStructWithContext(ctx, &request,
		validation.Field(&request.Key, validation.Required),
	)
	if err != nil {
		return pkgError.ValidationError(err.Error())
	}
	return nil
}

func ValidateNewsletterMute(ctx context.Context, request domainNewsletter.ToggleMuteRequest) error {
	err := validation.ValidateStructWithContext(ctx, &request,
		validation.Field(&request.NewsletterID, validation.Required),
	)
	if err != nil {
		return pkgError.ValidationError(err.Error())
	}
	return nil
}
```

- [ ] **Step 5: Run validation test to verify it passes**

Run: `cd src && go test ./validations/ -run TestValidate.*Newsletter -v`
Expected: PASS (4 tests).

- [ ] **Step 6: Add usecase methods**

Append to `src/usecase/newsletter.go` (the file already imports `context`, `domainNewsletter`, `whatsapp`, `utils`, `validations`; add `"go.mau.fi/whatsmeow/types"` to its import block):

```go
func (service serviceNewsletter) Follow(ctx context.Context, request domainNewsletter.FollowRequest) (err error) {
	if err = validations.ValidateFollowNewsletter(ctx, request); err != nil {
		return err
	}
	JID, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.NewsletterID)
	if err != nil {
		return err
	}
	return whatsapp.GetClient().FollowNewsletter(ctx, JID)
}

func (service serviceNewsletter) GetInfo(ctx context.Context, request domainNewsletter.GetInfoRequest) (response *types.NewsletterMetadata, err error) {
	if err = validations.ValidateGetNewsletterInfo(ctx, request); err != nil {
		return nil, err
	}
	JID, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.NewsletterID)
	if err != nil {
		return nil, err
	}
	return whatsapp.GetClient().GetNewsletterInfo(ctx, JID)
}

func (service serviceNewsletter) GetInfoWithInvite(ctx context.Context, request domainNewsletter.GetInfoWithInviteRequest) (response *types.NewsletterMetadata, err error) {
	if err = validations.ValidateGetNewsletterInfoWithInvite(ctx, request); err != nil {
		return nil, err
	}
	utils.MustLogin(whatsapp.GetClient())
	return whatsapp.GetClient().GetNewsletterInfoWithInvite(ctx, request.Key)
}

func (service serviceNewsletter) ToggleMute(ctx context.Context, request domainNewsletter.ToggleMuteRequest) (err error) {
	if err = validations.ValidateNewsletterMute(ctx, request); err != nil {
		return err
	}
	JID, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.NewsletterID)
	if err != nil {
		return err
	}
	return whatsapp.GetClient().NewsletterToggleMute(ctx, JID, request.Mute)
}
```

- [ ] **Step 7: Add routes + handlers**

In `src/ui/rest/newsletter.go`, register the routes inside `InitRestNewsletter` (after the existing unfollow line):

```go
	app.Post("/newsletter/follow", rest.Follow)
	app.Get("/newsletter/info", rest.GetInfo)
	app.Get("/newsletter/info-from-invite", rest.GetInfoWithInvite)
	app.Post("/newsletter/mute", rest.ToggleMute)
```

Append the handlers to the same file:

```go
func (controller *Newsletter) Follow(c *fiber.Ctx) error {
	var request domainNewsletter.FollowRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}
	if err := controller.Service.Follow(c.UserContext(), request); err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, "Success follow newsletter", nil)
}

func (controller *Newsletter) GetInfo(c *fiber.Ctx) error {
	var request domainNewsletter.GetInfoRequest
	if err := c.QueryParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid query parameters: "+err.Error())
	}
	response, err := controller.Service.GetInfo(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, "Success get newsletter info", response)
}

func (controller *Newsletter) GetInfoWithInvite(c *fiber.Ctx) error {
	var request domainNewsletter.GetInfoWithInviteRequest
	if err := c.QueryParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid query parameters: "+err.Error())
	}
	response, err := controller.Service.GetInfoWithInvite(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, "Success get newsletter info", response)
}

func (controller *Newsletter) ToggleMute(c *fiber.Ctx) error {
	var request domainNewsletter.ToggleMuteRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}
	if err := controller.Service.ToggleMute(c.UserContext(), request); err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, "Success toggle newsletter mute", nil)
}
```

- [ ] **Step 8: Verify full build + tests**

Run: `cd src && go fmt ./... && go vet ./... && go build ./... && go test ./...`
Expected: no output from fmt/vet/build; all tests `ok`.

- [ ] **Step 9: Commit**

```bash
git add src/domains/newsletter/ src/usecase/newsletter.go src/ui/rest/newsletter.go src/validations/newsletter_validation.go src/validations/newsletter_validation_test.go
git commit -m "feat(newsletter): add follow, info, info-from-invite, mute endpoints

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: User block/unblock/about (Feature 2a)

**Files:**
- Modify: `src/domains/user/interfaces.go`
- Create: `src/domains/user/blocklist.go`
- Modify: `src/usecase/user.go`
- Modify: `src/ui/rest/user.go`
- Modify: `src/validations/user_validation.go`
- Test: `src/validations/user_block_test.go` (create)

**Interfaces:**
- Consumes: `events.BlocklistChangeActionBlock`/`Unblock` (`go.mau.fi/whatsmeow/types/events`), `utils.SanitizePhone`.
- Produces: `IUserUsecase` gains `GetBlocklist`, `Block`, `Unblock`, `SetAbout`. Structs: `BlockRequest{Phone}`, `SetAboutRequest{Status}`, `BlocklistResponse{*types.Blocklist}`.

- [ ] **Step 1: Write the failing validation test**

Create `src/validations/user_block_test.go`:

```go
package validations

import (
	"context"
	"testing"

	domainUser "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/user"
)

func TestValidateBlockUser(t *testing.T) {
	if err := ValidateBlockUser(context.Background(), domainUser.BlockRequest{}); err == nil {
		t.Error("expected error for empty phone, got nil")
	}
	if err := ValidateBlockUser(context.Background(), domainUser.BlockRequest{Phone: "628123@s.whatsapp.net"}); err != nil {
		t.Errorf("expected no error for valid phone, got %v", err)
	}
}

func TestValidateSetAbout(t *testing.T) {
	if err := ValidateSetAbout(context.Background(), domainUser.SetAboutRequest{}); err == nil {
		t.Error("expected error for empty status, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src && go test ./validations/ -run "TestValidateBlockUser|TestValidateSetAbout" -v`
Expected: FAIL — undefined `ValidateBlockUser`, `ValidateSetAbout`, `domainUser.BlockRequest`, `domainUser.SetAboutRequest`.

- [ ] **Step 3: Add domain structs + interface methods**

Create `src/domains/user/blocklist.go`:

```go
package user

import "go.mau.fi/whatsmeow/types"

type BlockRequest struct {
	Phone string `json:"phone" form:"phone"`
}

type SetAboutRequest struct {
	Status string `json:"status" form:"status"`
}

type BlocklistResponse struct {
	Data *types.Blocklist `json:"data"`
}
```

In `src/domains/user/interfaces.go`, add a new interface and embed it in `IUserUsecase`:

```go
// IUserBlocklist handles block/unblock and about operations
type IUserBlocklist interface {
	GetBlocklist(ctx context.Context) (response BlocklistResponse, err error)
	Block(ctx context.Context, request BlockRequest) (response BlocklistResponse, err error)
	Unblock(ctx context.Context, request BlockRequest) (response BlocklistResponse, err error)
	SetAbout(ctx context.Context, request SetAboutRequest) (err error)
}
```

Then add `IUserBlocklist` to the `IUserUsecase` embed list:

```go
type IUserUsecase interface {
	IUserInfo
	IUserProfile
	IUserListing
	IUserPrivacy
	IUserBlocklist
}
```

- [ ] **Step 4: Add validation functions**

Append to `src/validations/user_validation.go` (uses the same `validation`, `pkgError`, `domainUser` imports already present in that file):

```go
func ValidateBlockUser(ctx context.Context, request domainUser.BlockRequest) error {
	err := validation.ValidateStructWithContext(ctx, &request,
		validation.Field(&request.Phone, validation.Required),
	)
	if err != nil {
		return pkgError.ValidationError(err.Error())
	}
	return nil
}

func ValidateSetAbout(ctx context.Context, request domainUser.SetAboutRequest) error {
	err := validation.ValidateStructWithContext(ctx, &request,
		validation.Field(&request.Status, validation.Required),
	)
	if err != nil {
		return pkgError.ValidationError(err.Error())
	}
	return nil
}
```

- [ ] **Step 5: Run validation test to verify it passes**

Run: `cd src && go test ./validations/ -run "TestValidateBlockUser|TestValidateSetAbout" -v`
Expected: PASS.

- [ ] **Step 6: Add usecase methods**

Append to `src/usecase/user.go`. Add `"go.mau.fi/whatsmeow/types/events"` to its import block. Implementation:

```go
func (service serviceUser) GetBlocklist(ctx context.Context) (response domainUser.BlocklistResponse, err error) {
	utils.MustLogin(whatsapp.GetClient())
	blocklist, err := whatsapp.GetClient().GetBlocklist(ctx)
	if err != nil {
		return response, err
	}
	response.Data = blocklist
	return response, nil
}

func (service serviceUser) Block(ctx context.Context, request domainUser.BlockRequest) (response domainUser.BlocklistResponse, err error) {
	if err = validations.ValidateBlockUser(ctx, request); err != nil {
		return response, err
	}
	JID, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.Phone)
	if err != nil {
		return response, err
	}
	blocklist, err := whatsapp.GetClient().UpdateBlocklist(ctx, JID, events.BlocklistChangeActionBlock)
	if err != nil {
		return response, err
	}
	response.Data = blocklist
	return response, nil
}

func (service serviceUser) Unblock(ctx context.Context, request domainUser.BlockRequest) (response domainUser.BlocklistResponse, err error) {
	if err = validations.ValidateBlockUser(ctx, request); err != nil {
		return response, err
	}
	JID, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.Phone)
	if err != nil {
		return response, err
	}
	blocklist, err := whatsapp.GetClient().UpdateBlocklist(ctx, JID, events.BlocklistChangeActionUnblock)
	if err != nil {
		return response, err
	}
	response.Data = blocklist
	return response, nil
}

func (service serviceUser) SetAbout(ctx context.Context, request domainUser.SetAboutRequest) (err error) {
	if err = validations.ValidateSetAbout(ctx, request); err != nil {
		return err
	}
	utils.MustLogin(whatsapp.GetClient())
	return whatsapp.GetClient().SetStatusMessage(ctx, request.Status)
}
```

- [ ] **Step 7: Add routes + handlers**

In `src/ui/rest/user.go` `InitRestUser`, add:

```go
	app.Get("/user/blocklist", rest.UserGetBlocklist)
	app.Post("/user/block", rest.UserBlock)
	app.Post("/user/unblock", rest.UserUnblock)
	app.Post("/user/about", rest.UserSetAbout)
```

Append handlers:

```go
func (controller *User) UserGetBlocklist(c *fiber.Ctx) error {
	response, err := controller.Service.GetBlocklist(c.UserContext())
	if err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, "Success get blocklist", response)
}

func (controller *User) UserBlock(c *fiber.Ctx) error {
	var request domainUser.BlockRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}
	utils.SanitizePhone(&request.Phone)
	response, err := controller.Service.Block(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, "Success block user", response)
}

func (controller *User) UserUnblock(c *fiber.Ctx) error {
	var request domainUser.BlockRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}
	utils.SanitizePhone(&request.Phone)
	response, err := controller.Service.Unblock(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, "Success unblock user", response)
}

func (controller *User) UserSetAbout(c *fiber.Ctx) error {
	var request domainUser.SetAboutRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}
	if err := controller.Service.SetAbout(c.UserContext(), request); err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, "Success set about", nil)
}
```

- [ ] **Step 8: Verify full build + tests**

Run: `cd src && go fmt ./... && go vet ./... && go build ./... && go test ./...`
Expected: clean; all tests `ok`.

- [ ] **Step 9: Commit**

```bash
git add src/domains/user/ src/usecase/user.go src/ui/rest/user.go src/validations/user_validation.go src/validations/user_block_test.go
git commit -m "feat(user): add block, unblock, blocklist and set-about endpoints

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Group admin modes (Feature 2b)

**Files:**
- Modify: `src/domains/group/interfaces.go`
- Create: `src/domains/group/admin_modes.go`
- Modify: `src/usecase/group.go`
- Modify: `src/ui/rest/group.go`
- Modify: `src/validations/group_validation.go`
- Test: `src/validations/group_admin_modes_test.go` (create)

**Interfaces:**
- Consumes: `types.GroupMemberAddModeAdmin`/`AllMember` (`go.mau.fi/whatsmeow/types`).
- Produces: `IGroupSettings` gains `SetGroupJoinApprovalMode`, `SetGroupMemberAddMode`. Structs: `SetGroupJoinApprovalModeRequest{GroupID, Enabled}`, `SetGroupMemberAddModeRequest{GroupID, Mode}`.

- [ ] **Step 1: Write the failing validation test**

Create `src/validations/group_admin_modes_test.go`:

```go
package validations

import (
	"context"
	"testing"

	domainGroup "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/group"
)

func TestValidateSetGroupJoinApprovalMode(t *testing.T) {
	if err := ValidateSetGroupJoinApprovalMode(context.Background(), domainGroup.SetGroupJoinApprovalModeRequest{}); err == nil {
		t.Error("expected error for empty group_id, got nil")
	}
	if err := ValidateSetGroupJoinApprovalMode(context.Background(), domainGroup.SetGroupJoinApprovalModeRequest{GroupID: "123@g.us", Enabled: true}); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateSetGroupMemberAddMode(t *testing.T) {
	if err := ValidateSetGroupMemberAddMode(context.Background(), domainGroup.SetGroupMemberAddModeRequest{GroupID: "123@g.us"}); err == nil {
		t.Error("expected error for empty mode, got nil")
	}
	if err := ValidateSetGroupMemberAddMode(context.Background(), domainGroup.SetGroupMemberAddModeRequest{GroupID: "123@g.us", Mode: "invalid_mode"}); err == nil {
		t.Error("expected error for invalid mode, got nil")
	}
	if err := ValidateSetGroupMemberAddMode(context.Background(), domainGroup.SetGroupMemberAddModeRequest{GroupID: "123@g.us", Mode: "admin_add"}); err != nil {
		t.Errorf("expected no error for admin_add, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src && go test ./validations/ -run "TestValidateSetGroupJoinApprovalMode|TestValidateSetGroupMemberAddMode" -v`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Add domain structs + interface methods**

Create `src/domains/group/admin_modes.go`:

```go
package group

type SetGroupJoinApprovalModeRequest struct {
	GroupID string `json:"group_id" form:"group_id"`
	Enabled bool   `json:"enabled" form:"enabled"`
}

type SetGroupMemberAddModeRequest struct {
	GroupID string `json:"group_id" form:"group_id"`
	Mode    string `json:"mode" form:"mode"` // "admin_add" or "all_member_add"
}
```

In `src/domains/group/interfaces.go`, add to the `IGroupSettings` interface:

```go
	SetGroupJoinApprovalMode(ctx context.Context, request SetGroupJoinApprovalModeRequest) (err error)
	SetGroupMemberAddMode(ctx context.Context, request SetGroupMemberAddModeRequest) (err error)
```

- [ ] **Step 4: Add validation functions**

Append to `src/validations/group_validation.go` (file already imports `validation` and `pkgError` and `domainGroup`):

```go
func ValidateSetGroupJoinApprovalMode(ctx context.Context, request domainGroup.SetGroupJoinApprovalModeRequest) error {
	err := validation.ValidateStructWithContext(ctx, &request,
		validation.Field(&request.GroupID, validation.Required),
	)
	if err != nil {
		return pkgError.ValidationError(err.Error())
	}
	return nil
}

func ValidateSetGroupMemberAddMode(ctx context.Context, request domainGroup.SetGroupMemberAddModeRequest) error {
	err := validation.ValidateStructWithContext(ctx, &request,
		validation.Field(&request.GroupID, validation.Required),
		validation.Field(&request.Mode, validation.Required, validation.In("admin_add", "all_member_add")),
	)
	if err != nil {
		return pkgError.ValidationError(err.Error())
	}
	return nil
}
```

- [ ] **Step 5: Run validation test to verify it passes**

Run: `cd src && go test ./validations/ -run "TestValidateSetGroupJoinApprovalMode|TestValidateSetGroupMemberAddMode" -v`
Expected: PASS.

- [ ] **Step 6: Add usecase methods**

Append to `src/usecase/group.go` (already imports `whatsapp`, `utils`, `validations`, `types`):

```go
func (service serviceGroup) SetGroupJoinApprovalMode(ctx context.Context, request domainGroup.SetGroupJoinApprovalModeRequest) (err error) {
	if err = validations.ValidateSetGroupJoinApprovalMode(ctx, request); err != nil {
		return err
	}
	groupJID, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.GroupID)
	if err != nil {
		return err
	}
	return whatsapp.GetClient().SetGroupJoinApprovalMode(ctx, groupJID, request.Enabled)
}

func (service serviceGroup) SetGroupMemberAddMode(ctx context.Context, request domainGroup.SetGroupMemberAddModeRequest) (err error) {
	if err = validations.ValidateSetGroupMemberAddMode(ctx, request); err != nil {
		return err
	}
	groupJID, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.GroupID)
	if err != nil {
		return err
	}
	return whatsapp.GetClient().SetGroupMemberAddMode(ctx, groupJID, types.GroupMemberAddMode(request.Mode))
}
```

- [ ] **Step 7: Add routes + handlers**

In `src/ui/rest/group.go` `InitRestGroup`, add:

```go
	app.Post("/group/join-approval", rest.SetGroupJoinApprovalMode)
	app.Post("/group/member-add-mode", rest.SetGroupMemberAddMode)
```

Append handlers (mirror the existing `SetGroupAnnounce` handler in this file):

```go
func (controller *Group) SetGroupJoinApprovalMode(c *fiber.Ctx) error {
	var request domainGroup.SetGroupJoinApprovalModeRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}
	if err := controller.Service.SetGroupJoinApprovalMode(c.UserContext(), request); err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, "Success set group join approval mode", nil)
}

func (controller *Group) SetGroupMemberAddMode(c *fiber.Ctx) error {
	var request domainGroup.SetGroupMemberAddModeRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}
	if err := controller.Service.SetGroupMemberAddMode(c.UserContext(), request); err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, "Success set group member add mode", nil)
}
```

- [ ] **Step 8: Verify full build + tests**

Run: `cd src && go fmt ./... && go vet ./... && go build ./... && go test ./...`
Expected: clean; all tests `ok`.

- [ ] **Step 9: Commit**

```bash
git add src/domains/group/ src/usecase/group.go src/ui/rest/group.go src/validations/group_validation.go src/validations/group_admin_modes_test.go
git commit -m "feat(group): add join-approval-mode and member-add-mode endpoints

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Communities / linked groups (Feature 3)

**Files:**
- Modify: `src/domains/group/interfaces.go`
- Create: `src/domains/group/community.go`
- Modify: `src/usecase/group.go`
- Modify: `src/ui/rest/group.go`
- Modify: `src/validations/group_validation.go`
- Test: `src/validations/group_community_test.go` (create)

**Interfaces:**
- Consumes: `whatsmeow` methods `GetSubGroups`, `LinkGroup`, `UnlinkGroup`, `GetLinkedGroupsParticipants`; `types.GroupLinkTarget`, `types.JID`.
- Produces: `IGroupUsecase` (via new `IGroupCommunity`) gains `GetSubGroups`, `LinkGroup`, `UnlinkGroup`, `GetLinkedParticipants`. Structs: `CommunityRequest{CommunityID}`, `LinkGroupRequest{CommunityID, GroupID}`, `SubGroupsResponse{[]*types.GroupLinkTarget}`, `LinkedParticipantsResponse{[]string}`.

- [ ] **Step 1: Write the failing validation test**

Create `src/validations/group_community_test.go`:

```go
package validations

import (
	"context"
	"testing"

	domainGroup "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/group"
)

func TestValidateCommunity(t *testing.T) {
	if err := ValidateCommunity(context.Background(), domainGroup.CommunityRequest{}); err == nil {
		t.Error("expected error for empty community_id, got nil")
	}
	if err := ValidateCommunity(context.Background(), domainGroup.CommunityRequest{CommunityID: "123@g.us"}); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateLinkGroup(t *testing.T) {
	if err := ValidateLinkGroup(context.Background(), domainGroup.LinkGroupRequest{CommunityID: "123@g.us"}); err == nil {
		t.Error("expected error for empty group_id, got nil")
	}
	if err := ValidateLinkGroup(context.Background(), domainGroup.LinkGroupRequest{CommunityID: "123@g.us", GroupID: "456@g.us"}); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src && go test ./validations/ -run "TestValidateCommunity|TestValidateLinkGroup" -v`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Add domain structs + interface methods**

Create `src/domains/group/community.go`:

```go
package group

import "go.mau.fi/whatsmeow/types"

type CommunityRequest struct {
	CommunityID string `json:"community_id" form:"community_id" query:"community_id"`
}

type LinkGroupRequest struct {
	CommunityID string `json:"community_id" form:"community_id"`
	GroupID     string `json:"group_id" form:"group_id"`
}

type SubGroupsResponse struct {
	Data []*types.GroupLinkTarget `json:"data"`
}

type LinkedParticipantsResponse struct {
	Participants []string `json:"participants"`
}
```

In `src/domains/group/interfaces.go`, add a new interface and embed it:

```go
// IGroupCommunity handles community (linked group) operations
type IGroupCommunity interface {
	GetSubGroups(ctx context.Context, request CommunityRequest) (response SubGroupsResponse, err error)
	LinkGroup(ctx context.Context, request LinkGroupRequest) (err error)
	UnlinkGroup(ctx context.Context, request LinkGroupRequest) (err error)
	GetLinkedParticipants(ctx context.Context, request CommunityRequest) (response LinkedParticipantsResponse, err error)
}
```

Add `IGroupCommunity` to the `IGroupUsecase` embed list:

```go
type IGroupUsecase interface {
	IGroupManagement
	IGroupParticipants
	IGroupSettings
	IGroupCommunity
}
```

- [ ] **Step 4: Add validation functions**

Append to `src/validations/group_validation.go`:

```go
func ValidateCommunity(ctx context.Context, request domainGroup.CommunityRequest) error {
	err := validation.ValidateStructWithContext(ctx, &request,
		validation.Field(&request.CommunityID, validation.Required),
	)
	if err != nil {
		return pkgError.ValidationError(err.Error())
	}
	return nil
}

func ValidateLinkGroup(ctx context.Context, request domainGroup.LinkGroupRequest) error {
	err := validation.ValidateStructWithContext(ctx, &request,
		validation.Field(&request.CommunityID, validation.Required),
		validation.Field(&request.GroupID, validation.Required),
	)
	if err != nil {
		return pkgError.ValidationError(err.Error())
	}
	return nil
}
```

- [ ] **Step 5: Run validation test to verify it passes**

Run: `cd src && go test ./validations/ -run "TestValidateCommunity|TestValidateLinkGroup" -v`
Expected: PASS.

- [ ] **Step 6: Add usecase methods**

Append to `src/usecase/group.go`:

```go
func (service serviceGroup) GetSubGroups(ctx context.Context, request domainGroup.CommunityRequest) (response domainGroup.SubGroupsResponse, err error) {
	if err = validations.ValidateCommunity(ctx, request); err != nil {
		return response, err
	}
	communityJID, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.CommunityID)
	if err != nil {
		return response, err
	}
	subGroups, err := whatsapp.GetClient().GetSubGroups(ctx, communityJID)
	if err != nil {
		return response, err
	}
	response.Data = subGroups
	return response, nil
}

func (service serviceGroup) LinkGroup(ctx context.Context, request domainGroup.LinkGroupRequest) (err error) {
	if err = validations.ValidateLinkGroup(ctx, request); err != nil {
		return err
	}
	parentJID, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.CommunityID)
	if err != nil {
		return err
	}
	childJID, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.GroupID)
	if err != nil {
		return err
	}
	return whatsapp.GetClient().LinkGroup(ctx, parentJID, childJID)
}

func (service serviceGroup) UnlinkGroup(ctx context.Context, request domainGroup.LinkGroupRequest) (err error) {
	if err = validations.ValidateLinkGroup(ctx, request); err != nil {
		return err
	}
	parentJID, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.CommunityID)
	if err != nil {
		return err
	}
	childJID, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.GroupID)
	if err != nil {
		return err
	}
	return whatsapp.GetClient().UnlinkGroup(ctx, parentJID, childJID)
}

func (service serviceGroup) GetLinkedParticipants(ctx context.Context, request domainGroup.CommunityRequest) (response domainGroup.LinkedParticipantsResponse, err error) {
	if err = validations.ValidateCommunity(ctx, request); err != nil {
		return response, err
	}
	communityJID, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.CommunityID)
	if err != nil {
		return response, err
	}
	participants, err := whatsapp.GetClient().GetLinkedGroupsParticipants(ctx, communityJID)
	if err != nil {
		return response, err
	}
	response.Participants = make([]string, 0, len(participants))
	for _, p := range participants {
		response.Participants = append(response.Participants, p.String())
	}
	return response, nil
}
```

- [ ] **Step 7: Add routes + handlers**

In `src/ui/rest/group.go` `InitRestGroup`, add:

```go
	app.Get("/group/sub-groups", rest.GetSubGroups)
	app.Post("/group/link", rest.LinkGroup)
	app.Post("/group/unlink", rest.UnlinkGroup)
	app.Get("/group/linked-participants", rest.GetLinkedParticipants)
```

Append handlers:

```go
func (controller *Group) GetSubGroups(c *fiber.Ctx) error {
	var request domainGroup.CommunityRequest
	if err := c.QueryParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid query parameters: "+err.Error())
	}
	response, err := controller.Service.GetSubGroups(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, "Success get sub groups", response)
}

func (controller *Group) LinkGroup(c *fiber.Ctx) error {
	var request domainGroup.LinkGroupRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}
	if err := controller.Service.LinkGroup(c.UserContext(), request); err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, "Success link group", nil)
}

func (controller *Group) UnlinkGroup(c *fiber.Ctx) error {
	var request domainGroup.LinkGroupRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}
	if err := controller.Service.UnlinkGroup(c.UserContext(), request); err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, "Success unlink group", nil)
}

func (controller *Group) GetLinkedParticipants(c *fiber.Ctx) error {
	var request domainGroup.CommunityRequest
	if err := c.QueryParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid query parameters: "+err.Error())
	}
	response, err := controller.Service.GetLinkedParticipants(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, "Success get linked participants", response)
}
```

- [ ] **Step 8: Verify full build + tests**

Run: `cd src && go fmt ./... && go vet ./... && go build ./... && go test ./...`
Expected: clean; all tests `ok`.

- [ ] **Step 9: Commit**

```bash
git add src/domains/group/ src/usecase/group.go src/ui/rest/group.go src/validations/group_validation.go src/validations/group_community_test.go
git commit -m "feat(group): add community (linked group) management endpoints

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Poll vote send endpoint (Feature 4a)

**Files:**
- Create: `src/domains/send/poll_vote.go`
- Modify: `src/domains/send/interfaces.go`
- Modify: `src/usecase/send_misc.go`
- Modify: `src/ui/rest/send.go`
- Modify: `src/validations/send_validation.go`
- Test: `src/validations/send_poll_vote_test.go` (create)

**Interfaces:**
- Consumes: `chatStorageRepo.GetMessageByID(id) (*chatstorage.Message, error)`, `whatsapp.GetClient().BuildPollVote`, `service.wrapSendMessage`, `types.MessageInfo`/`types.MessageSource`, `types.ParseJID`.
- Produces: `ISendUsecase` gains `SendPollVote`. Struct: `PollVoteRequest{Phone, PollMessageID, OptionNames}`.

- [ ] **Step 1: Write the failing validation test**

Create `src/validations/send_poll_vote_test.go`:

```go
package validations

import (
	"context"
	"testing"

	domainSend "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/send"
)

func TestValidateSendPollVote(t *testing.T) {
	// missing phone
	if err := ValidateSendPollVote(context.Background(), domainSend.PollVoteRequest{PollMessageID: "ABC", OptionNames: []string{"Yes"}}); err == nil {
		t.Error("expected error for empty phone, got nil")
	}
	// missing poll_message_id
	if err := ValidateSendPollVote(context.Background(), domainSend.PollVoteRequest{BaseRequest: domainSend.BaseRequest{Phone: "628@s.whatsapp.net"}, OptionNames: []string{"Yes"}}); err == nil {
		t.Error("expected error for empty poll_message_id, got nil")
	}
	// valid (empty OptionNames is allowed = retract vote)
	if err := ValidateSendPollVote(context.Background(), domainSend.PollVoteRequest{BaseRequest: domainSend.BaseRequest{Phone: "628@s.whatsapp.net"}, PollMessageID: "ABC"}); err != nil {
		t.Errorf("expected no error for retract vote, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src && go test ./validations/ -run TestValidateSendPollVote -v`
Expected: FAIL — undefined `ValidateSendPollVote`, `domainSend.PollVoteRequest`.

- [ ] **Step 3: Add domain struct + interface method**

Create `src/domains/send/poll_vote.go`:

```go
package send

type PollVoteRequest struct {
	BaseRequest
	PollMessageID string   `json:"poll_message_id" form:"poll_message_id"`
	OptionNames   []string `json:"option_names" form:"option_names"`
}
```

In `src/domains/send/interfaces.go`, add to the `ISendUsecase` interface (next to `SendPoll`):

```go
	SendPollVote(ctx context.Context, request PollVoteRequest) (response GenericResponse, err error)
```

- [ ] **Step 4: Add validation function**

Append to `src/validations/send_validation.go`:

```go
func ValidateSendPollVote(ctx context.Context, request domainSend.PollVoteRequest) error {
	err := validation.ValidateStructWithContext(ctx, &request,
		validation.Field(&request.Phone, validation.Required),
		validation.Field(&request.PollMessageID, validation.Required),
	)
	if err != nil {
		return pkgError.ValidationError(err.Error())
	}
	return nil
}
```

- [ ] **Step 5: Run validation test to verify it passes**

Run: `cd src && go test ./validations/ -run TestValidateSendPollVote -v`
Expected: PASS.

- [ ] **Step 6: Add usecase method**

Append to `src/usecase/send_misc.go`. Ensure these imports exist in that file's import block: `"go.mau.fi/whatsmeow/types"`, `pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"` (add any missing). Implementation:

```go
func (service serviceSend) SendPollVote(ctx context.Context, request domainSend.PollVoteRequest) (response domainSend.GenericResponse, err error) {
	if err = validations.ValidateSendPollVote(ctx, request); err != nil {
		return response, err
	}
	recipient, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.BaseRequest.Phone)
	if err != nil {
		return response, err
	}

	// Reconstruct the original poll's MessageInfo from chat storage.
	stored, err := service.chatStorageRepo.GetMessageByID(request.PollMessageID)
	if err != nil || stored == nil {
		return response, pkgError.ValidationError("poll message not found in storage; cannot vote on an unknown poll")
	}
	chatJID, err := types.ParseJID(stored.ChatJID)
	if err != nil {
		return response, pkgError.ValidationError("stored poll has an invalid chat JID")
	}
	senderJID, err := types.ParseJID(stored.Sender)
	if err != nil {
		return response, pkgError.ValidationError("stored poll has an invalid sender JID")
	}
	pollInfo := &types.MessageInfo{
		ID: request.PollMessageID,
		MessageSource: types.MessageSource{
			Chat:     chatJID,
			Sender:   senderJID,
			IsFromMe: stored.IsFromMe,
		},
		Timestamp: stored.Timestamp,
	}

	msg, err := whatsapp.GetClient().BuildPollVote(ctx, pollInfo, request.OptionNames)
	if err != nil {
		return response, err
	}

	ts, err := service.wrapSendMessage(ctx, recipient, msg, "🗳️ poll vote")
	if err != nil {
		return response, err
	}
	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Send poll vote success %s (server timestamp: %s)", request.BaseRequest.Phone, ts.Timestamp.String())
	return response, nil
}
```

- [ ] **Step 7: Add route + handler**

In `src/ui/rest/send.go`, register inside the send `InitRest*`/route block (next to the existing poll route):

```go
	app.Post("/send/poll/vote", rest.SendPollVote)
```

Append the handler (mirror the existing `SendPoll` handler in this file — same struct/parse/call/return shape):

```go
func (controller *Send) SendPollVote(c *fiber.Ctx) error {
	var request domainSend.PollVoteRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}
	utils.SanitizePhone(&request.Phone)
	response, err := controller.Service.SendPollVote(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, response.Status, response)
}
```

(Confirmed: `src/ui/rest/send.go` uses `func InitRestSend(...)`, `type Send struct`, receiver `*Send`, and registers `/send/poll` at line 25 — this matches the handler above. Add the new route line right after the `/send/poll` line.)

- [ ] **Step 8: Verify full build + tests**

Run: `cd src && go fmt ./... && go vet ./... && go build ./... && go test ./...`
Expected: clean; all tests `ok`.

- [ ] **Step 9: Commit**

```bash
git add src/domains/send/ src/usecase/send_misc.go src/ui/rest/send.go src/validations/send_validation.go src/validations/send_poll_vote_test.go
git commit -m "feat(send): add poll vote endpoint

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Poll-update decrypt + webhook/SSE forward (Feature 4b)

When a poll vote arrives it is an `events.Message` carrying a `PollUpdateMessage` (encrypted). Detect it, decrypt with `DecryptPollVote`, build a forward payload, and emit it via SSE + the existing webhook path. The testable unit is a pure payload builder; the handler wires it to the live client.

**Files:**
- Create: `src/infrastructure/whatsapp/event_poll.go`
- Modify: `src/infrastructure/whatsapp/handlers.go:205-208` (add poll detection next to the reaction check)
- Create: `src/ui/sse/poll.go` (new `BroadcastPollVote` helper)
- Test: `src/infrastructure/whatsapp/event_poll_test.go` (create)

**Interfaces:**
- Consumes: `whatsapp.GetClient().DecryptPollVote(ctx, evt) (*waE2E.PollVoteMessage, error)`, `sse.BroadcastMessage`/existing SSE hub, and the existing `forwardPayloadToConfiguredWebhooks(ctx, payload map[string]any, eventName string) error` (in `webhook_forward.go` — loops `config.WhatsappWebhook` and calls `submitWebhook`, handling the zero-webhook case).
- Produces: `buildPollVotePayload(messageID, chatJID, senderJID string, isFromMe bool, ts time.Time, vote *waE2E.PollVoteMessage) map[string]any`; `sse.BroadcastPollVote(messageID, chatJID, senderJID string, selectedHashes []string, timestamp time.Time, isFromMe bool)`; `handlePollUpdateMessage(ctx, evt)`.

- [ ] **Step 1: Write the failing test for the pure payload builder**

Create `src/infrastructure/whatsapp/event_poll_test.go`:

```go
package whatsapp

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
)

func TestBuildPollVotePayload(t *testing.T) {
	vote := &waE2E.PollVoteMessage{
		SelectedOptions: [][]byte{{0x01, 0x02}, {0x03, 0x04}},
	}
	ts := time.Unix(1700000000, 0)
	payload := buildPollVotePayload("MSGID", "123@g.us", "456@s.whatsapp.net", false, ts, vote)

	if payload["message_id"] != "MSGID" {
		t.Errorf("message_id = %v, want MSGID", payload["message_id"])
	}
	if payload["chat_jid"] != "123@g.us" {
		t.Errorf("chat_jid = %v, want 123@g.us", payload["chat_jid"])
	}
	hashes, ok := payload["selected_option_hashes"].([]string)
	if !ok || len(hashes) != 2 {
		t.Fatalf("selected_option_hashes = %v, want 2 hex strings", payload["selected_option_hashes"])
	}
	if hashes[0] != "0102" {
		t.Errorf("hashes[0] = %s, want 0102", hashes[0])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src && go test ./infrastructure/whatsapp/ -run TestBuildPollVotePayload -v`
Expected: FAIL — undefined `buildPollVotePayload`.

- [ ] **Step 3: Implement the payload builder + handler**

Create `src/infrastructure/whatsapp/event_poll.go`:

```go
package whatsapp

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/sse"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
)

// buildPollVotePayload converts a decrypted poll vote into a forwardable payload.
// SelectedOptions are SHA-256 hashes of the option names (whatsmeow does not
// resolve them to text); consumers match them against the poll's option hashes.
func buildPollVotePayload(messageID, chatJID, senderJID string, isFromMe bool, ts time.Time, vote *waE2E.PollVoteMessage) map[string]any {
	hashes := make([]string, 0, len(vote.GetSelectedOptions()))
	for _, opt := range vote.GetSelectedOptions() {
		hashes = append(hashes, hex.EncodeToString(opt))
	}
	return map[string]any{
		"event":                  "poll_vote",
		"message_id":             messageID,
		"chat_jid":               chatJID,
		"sender_jid":             senderJID,
		"is_from_me":             isFromMe,
		"selected_option_hashes": hashes,
		"timestamp":              ts,
	}
}

// handlePollUpdateMessage decrypts an inbound poll vote and forwards it via SSE
// and (if configured) the webhook. Best-effort: failures are logged, never fatal.
func handlePollUpdateMessage(ctx context.Context, evt *events.Message) {
	client := GetClient()
	normalizedChatJID := NormalizeJIDFromLID(ctx, evt.Info.Chat, client)
	normalizedSenderJID := NormalizeJIDFromLID(ctx, evt.Info.Sender, client)

	vote, err := client.DecryptPollVote(ctx, evt)
	if err != nil {
		logrus.Errorf("Failed to decrypt poll vote %s: %v", evt.Info.ID, err)
		return
	}

	hashes := make([]string, 0, len(vote.GetSelectedOptions()))
	for _, opt := range vote.GetSelectedOptions() {
		hashes = append(hashes, hex.EncodeToString(opt))
	}

	sse.BroadcastPollVote(
		evt.Info.ID,
		normalizedChatJID.String(),
		normalizedSenderJID.String(),
		hashes,
		evt.Info.Timestamp,
		evt.Info.IsFromMe,
	)

	payload := buildPollVotePayload(
		evt.Info.ID,
		normalizedChatJID.String(),
		normalizedSenderJID.String(),
		evt.Info.IsFromMe,
		evt.Info.Timestamp,
		vote,
	)
	go func() {
		if err := forwardPayloadToConfiguredWebhooks(ctx, payload, "poll_vote"); err != nil {
			logrus.Errorf("Failed to forward poll vote to webhook: %v", err)
		}
	}()
}
```

(`forwardPayloadToConfiguredWebhooks(ctx, payload, eventName)` already exists in `webhook_forward.go`; it loops `config.WhatsappWebhook`, calls `submitWebhook` per URL, signs/retries, and no-ops when zero webhooks are configured. No new webhook code is needed — do NOT modify `webhook.go`.)

- [ ] **Step 4: Add the SSE broadcast helper**

Create `src/ui/sse/poll.go`:

```go
package sse

import "time"

// BroadcastPollVote pushes a decrypted poll vote to connected SSE clients.
func BroadcastPollVote(messageID, chatJID, senderJID string, selectedHashes []string, timestamp time.Time, isFromMe bool) {
	BroadcastMessage(EventPollVote, "POLL_VOTE", "Poll vote received", map[string]any{
		"message_id":             messageID,
		"chat_jid":               chatJID,
		"sender_jid":             senderJID,
		"selected_option_hashes": selectedHashes,
		"timestamp":              timestamp,
		"is_from_me":             isFromMe,
	})
}
```

Add the `EventPollVote` constant next to the other `EventType` constants in `src/ui/sse/sse.go` (read the existing `EventType` const block first; follow its exact naming, e.g.):

```go
	EventPollVote EventType = "poll_vote"
```

> Note: confirm the `EventType` constant style and the `BroadcastMessage(eventType, code, message, data)` signature in `sse.go` (already verified to exist at line 173) before editing.

- [ ] **Step 5: Wire detection into handleMessage**

In `src/infrastructure/whatsapp/handlers.go`, immediately after the reaction-message block (the `if reactionMessage := ...; reactionMessage != nil { handleReactionMessage(...); return }` at lines 205-208), add:

```go
	// Check if this is a poll vote (PollUpdateMessage)
	if pollUpdate := evt.Message.GetPollUpdateMessage(); pollUpdate != nil {
		handlePollUpdateMessage(ctx, evt)
		return
	}
```

- [ ] **Step 6: Run the unit test + full build**

Run: `cd src && go test ./infrastructure/whatsapp/ -run TestBuildPollVotePayload -v`
Expected: PASS.

Run: `cd src && go fmt ./... && go vet ./... && go build ./... && go test ./...`
Expected: clean; all tests `ok`.

- [ ] **Step 7: Commit**

```bash
git add src/infrastructure/whatsapp/event_poll.go src/infrastructure/whatsapp/event_poll_test.go src/infrastructure/whatsapp/handlers.go src/ui/sse/poll.go src/ui/sse/sse.go
git commit -m "feat(poll): decrypt inbound poll votes and forward via SSE + webhook

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Final verification (after Task 6)

- [ ] Run the full suite once more: `cd src && go fmt ./... && go vet ./... && go build ./... && go test ./...`
- [ ] Manually skim `git log --oneline feat/whatsmeow-capability-gaps` — expect 6 feature commits + the earlier bump/spec commit.
- [ ] Update `docs/openapi.yaml` / README endpoint list if the project documents endpoints there (check first; out of scope if no such file is maintained).

## Spec coverage check

- F1 Newsletter (follow/info/info-from-invite/mute) → Task 1 ✓
- F2 Quick wins: block/unblock/blocklist/about → Task 2 ✓; group join-approval + member-add-mode → Task 3 ✓ (SetGroupDescription intentionally excluded per spec)
- F3 Communities (sub-groups/link/unlink/linked-participants) → Task 4 ✓
- F4 Poll vote (send) → Task 5 ✓; poll results decrypt+forward via webhook/SSE → Task 6 ✓ (no persistence/aggregation, per spec)
