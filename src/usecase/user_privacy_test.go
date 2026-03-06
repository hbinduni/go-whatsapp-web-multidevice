package usecase

import (
	"testing"

	domainUser "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/user"
	"go.mau.fi/whatsmeow/types"
)

func TestMapPrivacySettings(t *testing.T) {
	settings := &types.PrivacySettings{
		GroupAdd:     types.PrivacySettingContacts,
		LastSeen:     types.PrivacySettingNone,
		Status:       types.PrivacySettingAll,
		Profile:      types.PrivacySettingContactBlacklist,
		ReadReceipts: types.PrivacySettingNone,
		CallAdd:      types.PrivacySettingKnown,
		Online:       types.PrivacySettingMatchLastSeen,
		Messages:     types.PrivacySettingContacts,
		Defense:      types.PrivacySettingOnStandard,
		Stickers:     types.PrivacySettingContactAllowlist,
	}

	got := mapPrivacySettings(settings)
	want := domainUser.MyPrivacySettingResponse{
		GroupAdd:     "contacts",
		LastSeen:     "none",
		Status:       "all",
		Profile:      "contact_blacklist",
		ReadReceipts: "none",
		CallAdd:      "known",
		Online:       "match_last_seen",
		Messages:     "contacts",
		Defense:      "on_standard",
		Stickers:     "contact_allowlist",
	}

	if got != want {
		t.Fatalf("mapPrivacySettings() = %+v, want %+v", got, want)
	}
}

func TestMapPrivacySettingsNil(t *testing.T) {
	got := mapPrivacySettings(nil)
	if got != (domainUser.MyPrivacySettingResponse{}) {
		t.Fatalf("mapPrivacySettings(nil) = %+v, want empty response", got)
	}
}
