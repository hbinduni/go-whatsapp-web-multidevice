package validations

import (
	"context"

	domainUser "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/user"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"go.mau.fi/whatsmeow/types"
)

func ValidateUserInfo(ctx context.Context, request domainUser.InfoRequest) error {
	err := validation.ValidateStructWithContext(ctx, &request,
		validation.Field(&request.Phone, validation.Required),
	)

	if err != nil {
		return pkgError.ValidationError(err.Error())
	}

	return nil
}
func ValidateUserAvatar(ctx context.Context, request domainUser.AvatarRequest) error {
	err := validation.ValidateStructWithContext(ctx, &request,
		validation.Field(&request.Phone, validation.Required),
		validation.Field(&request.IsCommunity, validation.When(request.IsCommunity, validation.Required, validation.In(true, false))),
		validation.Field(&request.IsPreview, validation.When(request.IsPreview, validation.Required, validation.In(true, false))),
	)

	if err != nil {
		return pkgError.ValidationError(err.Error())
	}

	return nil
}

func ValidateBusinessProfile(ctx context.Context, request domainUser.BusinessProfileRequest) error {
	err := validation.ValidateStructWithContext(ctx, &request,
		validation.Field(&request.Phone, validation.Required),
	)

	if err != nil {
		return pkgError.ValidationError(err.Error())
	}

	return nil
}

func ValidateUpdatePrivacySetting(_ context.Context, request domainUser.UpdatePrivacySettingRequest) error {
	if request.Name == "" {
		return pkgError.ValidationError("name: cannot be blank.")
	}
	if request.Value == "" {
		return pkgError.ValidationError("value: cannot be blank.")
	}

	allowedValuesByName := map[string]map[string]struct{}{
		string(types.PrivacySettingTypeGroupAdd): {
			string(types.PrivacySettingAll): {}, string(types.PrivacySettingContacts): {}, string(types.PrivacySettingContactBlacklist): {}, string(types.PrivacySettingNone): {},
		},
		string(types.PrivacySettingTypeLastSeen): {
			string(types.PrivacySettingAll): {}, string(types.PrivacySettingContacts): {}, string(types.PrivacySettingContactBlacklist): {}, string(types.PrivacySettingNone): {},
		},
		string(types.PrivacySettingTypeStatus): {
			string(types.PrivacySettingAll): {}, string(types.PrivacySettingContacts): {}, string(types.PrivacySettingContactBlacklist): {}, string(types.PrivacySettingNone): {},
		},
		string(types.PrivacySettingTypeProfile): {
			string(types.PrivacySettingAll): {}, string(types.PrivacySettingContacts): {}, string(types.PrivacySettingContactBlacklist): {}, string(types.PrivacySettingNone): {},
		},
		string(types.PrivacySettingTypeReadReceipts): {
			string(types.PrivacySettingAll): {}, string(types.PrivacySettingNone): {},
		},
		string(types.PrivacySettingTypeOnline): {
			string(types.PrivacySettingAll): {}, string(types.PrivacySettingMatchLastSeen): {},
		},
		string(types.PrivacySettingTypeCallAdd): {
			string(types.PrivacySettingAll): {}, string(types.PrivacySettingKnown): {},
		},
		string(types.PrivacySettingTypeMessages): {
			string(types.PrivacySettingAll): {}, string(types.PrivacySettingContacts): {},
		},
		string(types.PrivacySettingTypeDefense): {
			string(types.PrivacySettingOnStandard): {}, string(types.PrivacySettingOff): {},
		},
		string(types.PrivacySettingTypeStickers): {
			string(types.PrivacySettingContacts): {}, string(types.PrivacySettingContactAllowlist): {}, string(types.PrivacySettingNone): {},
		},
	}

	allowedValues, ok := allowedValuesByName[request.Name]
	if !ok {
		return pkgError.ValidationError("name: invalid privacy setting type")
	}
	if _, ok = allowedValues[request.Value]; !ok {
		return pkgError.ValidationError("value: invalid value for name '" + request.Name + "'")
	}

	return nil
}

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
