package usecase

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"time"

	domainUser "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/user"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/validations"
	"github.com/disintegration/imaging"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/types"
)

type serviceUser struct {
	// Remove the WaCli field - we'll use the global client instead
}

func NewUserService() domainUser.IUserUsecase {
	return &serviceUser{}
}

func (service serviceUser) Info(ctx context.Context, request domainUser.InfoRequest) (response domainUser.InfoResponse, err error) {
	err = validations.ValidateUserInfo(ctx, request)
	if err != nil {
		return response, err
	}
	var jids []types.JID
	dataWaRecipient, err := utils.ValidateJidWithLogin(whatsapp.GetClientFromContext(ctx), request.Phone)
	if err != nil {
		return response, err
	}

	jids = append(jids, dataWaRecipient)
	resp, err := whatsapp.GetClientFromContext(ctx).GetUserInfo(ctx, jids)
	if err != nil {
		return response, err
	}

	for _, userInfo := range resp {
		var device []domainUser.InfoResponseDataDevice
		for _, j := range userInfo.Devices {
			device = append(device, domainUser.InfoResponseDataDevice{
				User:   j.User,
				Agent:  j.RawAgent,
				Device: utils.GetPlatformName(int(j.Device)),
				Server: j.Server,
				AD:     j.ADString(),
			})
		}

		data := domainUser.InfoResponseData{
			Status:    userInfo.Status,
			PictureID: userInfo.PictureID,
			Devices:   device,
		}
		if userInfo.VerifiedName != nil {
			data.VerifiedName = fmt.Sprintf("%v", *userInfo.VerifiedName)
		}
		response.Data = append(response.Data, data)
	}

	return response, nil
}

func (service serviceUser) Avatar(ctx context.Context, request domainUser.AvatarRequest) (response domainUser.AvatarResponse, err error) {
	// Create a child context with 2s timeout. Note: this respects the parent's deadline
	// if it's shorter (standard Go context behavior for request scoping)
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err = validations.ValidateUserAvatar(ctx, request); err != nil {
		return response, err
	}

	dataWaRecipient, err := utils.ValidateJidWithLogin(whatsapp.GetClientFromContext(ctx), request.Phone)
	if err != nil {
		return response, err
	}

	pic, err := whatsapp.GetClientFromContext(ctx).GetProfilePictureInfo(ctx, dataWaRecipient, &whatsmeow.GetProfilePictureParams{
		Preview:     request.IsPreview,
		IsCommunity: request.IsCommunity,
	})
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return response, pkgError.ContextError("timeout getting avatar")
		}
		return response, err
	}
	if pic == nil {
		return response, errors.New("no avatar found")
	}

	response.URL = pic.URL
	response.ID = pic.ID
	response.Type = pic.Type
	return response, nil
}

func (service serviceUser) MyListGroups(ctx context.Context) (response domainUser.MyListGroupsResponse, err error) {
	utils.MustLogin(whatsapp.GetClientFromContext(ctx))

	groups, err := whatsapp.GetClientFromContext(ctx).GetJoinedGroups(ctx)
	if err != nil {
		return
	}

	for _, group := range groups {
		response.Data = append(response.Data, *group)
	}
	return response, nil
}

func (service serviceUser) MyListNewsletter(ctx context.Context) (response domainUser.MyListNewsletterResponse, err error) {
	utils.MustLogin(whatsapp.GetClientFromContext(ctx))

	datas, err := whatsapp.GetClientFromContext(ctx).GetSubscribedNewsletters(ctx)
	if err != nil {
		return
	}

	for _, data := range datas {
		response.Data = append(response.Data, *data)
	}
	return response, nil
}

func (service serviceUser) MyPrivacySetting(ctx context.Context) (response domainUser.MyPrivacySettingResponse, err error) {
	utils.MustLogin(whatsapp.GetClientFromContext(ctx))

	resp, err := whatsapp.GetClientFromContext(ctx).TryFetchPrivacySettings(ctx, true)
	if err != nil {
		return
	}

	response.GroupAdd = string(resp.GroupAdd)
	response.Status = string(resp.Status)
	response.ReadReceipts = string(resp.ReadReceipts)
	response.Profile = string(resp.Profile)
	return response, nil
}

func (service serviceUser) MyListContacts(ctx context.Context) (response domainUser.MyListContactsResponse, err error) {
	utils.MustLogin(whatsapp.GetClientFromContext(ctx))

	contacts, err := whatsapp.GetClientFromContext(ctx).Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		return
	}

	for jid, contact := range contacts {
		response.Data = append(response.Data, domainUser.MyListContactsResponseData{
			JID:  jid,
			Name: contact.FullName,
		})
	}

	return response, nil
}

func (service serviceUser) ChangeAvatar(ctx context.Context, request domainUser.ChangeAvatarRequest) (err error) {
	utils.MustLogin(whatsapp.GetClientFromContext(ctx))

	file, err := request.Avatar.Open()
	if err != nil {
		return err
	}
	defer file.Close()

	// Read original image
	srcImage, err := imaging.Decode(file)
	if err != nil {
		return fmt.Errorf("failed to decode image: %v", err)
	}

	// Get original dimensions
	bounds := srcImage.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate new dimensions for 1:1 aspect ratio
	size := width
	if height < width {
		size = height
	}
	if size > 640 {
		size = 640
	}

	// Create a square crop from the center
	left := (width - size) / 2
	top := (height - size) / 2
	croppedImage := imaging.Crop(srcImage, image.Rect(left, top, left+size, top+size))

	// Resize if needed
	if size > 640 {
		croppedImage = imaging.Resize(croppedImage, 640, 640, imaging.Lanczos)
	}

	// Convert to bytes
	var buf bytes.Buffer
	err = imaging.Encode(&buf, croppedImage, imaging.JPEG, imaging.JPEGQuality(80))
	if err != nil {
		return fmt.Errorf("failed to encode image: %v", err)
	}

	_, err = whatsapp.GetClientFromContext(ctx).SetGroupPhoto(ctx, types.JID{}, buf.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (service serviceUser) ChangePushName(ctx context.Context, request domainUser.ChangePushNameRequest) (err error) {
	utils.MustLogin(whatsapp.GetClientFromContext(ctx))

	err = whatsapp.GetClientFromContext(ctx).SendAppState(ctx, appstate.BuildSettingPushName(request.PushName))
	if err != nil {
		return err
	}
	return nil
}

func (service serviceUser) IsOnWhatsApp(ctx context.Context, request domainUser.CheckRequest) (response domainUser.CheckResponse, err error) {
	utils.MustLogin(whatsapp.GetClientFromContext(ctx))

	utils.SanitizePhone(&request.Phone)

	response.IsOnWhatsApp = utils.IsOnWhatsapp(whatsapp.GetClientFromContext(ctx), request.Phone)

	return response, nil
}

func (service serviceUser) BusinessProfile(ctx context.Context, request domainUser.BusinessProfileRequest) (response domainUser.BusinessProfileResponse, err error) {
	err = validations.ValidateBusinessProfile(ctx, request)
	if err != nil {
		return response, err
	}

	dataWaRecipient, err := utils.ValidateJidWithLogin(whatsapp.GetClientFromContext(ctx), request.Phone)
	if err != nil {
		return response, err
	}

	profile, err := whatsapp.GetClientFromContext(ctx).GetBusinessProfile(ctx, dataWaRecipient)
	if err != nil {
		return response, err
	}

	// Convert profile to response format
	response.JID = dataWaRecipient.String()
	response.Email = profile.Email
	response.Address = profile.Address

	// Convert categories
	for _, category := range profile.Categories {
		response.Categories = append(response.Categories, domainUser.BusinessProfileCategory{
			ID:   category.ID,
			Name: category.Name,
		})
	}

	// Convert profile options
	if profile.ProfileOptions != nil {
		response.ProfileOptions = make(map[string]string)
		for key, value := range profile.ProfileOptions {
			response.ProfileOptions[key] = value
		}
	}

	response.BusinessHoursTimeZone = profile.BusinessHoursTimeZone

	// Convert business hours
	for _, hours := range profile.BusinessHours {
		response.BusinessHours = append(response.BusinessHours, domainUser.BusinessProfileHoursConfig{
			DayOfWeek: hours.DayOfWeek,
			Mode:      hours.Mode,
			OpenTime:  utils.FormatBusinessHourTime(hours.OpenTime),
			CloseTime: utils.FormatBusinessHourTime(hours.CloseTime),
		})
	}

	return response, nil
}
