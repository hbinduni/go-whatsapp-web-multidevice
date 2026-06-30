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
	"go.mau.fi/whatsmeow/types/events"
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
	dataWaRecipient, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.Phone)
	if err != nil {
		return response, err
	}

	jids = append(jids, dataWaRecipient)
	resp, err := whatsapp.GetClient().GetUserInfo(ctx, jids)
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
	// Use context with timeout instead of busy-wait polling
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err = validations.ValidateUserAvatar(ctx, request); err != nil {
		return response, err
	}

	dataWaRecipient, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.Phone)
	if err != nil {
		return response, err
	}

	pic, err := whatsapp.GetClient().GetProfilePictureInfo(ctx, dataWaRecipient, &whatsmeow.GetProfilePictureParams{
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
	utils.MustLogin(whatsapp.GetClient())

	groups, err := whatsapp.GetClient().GetJoinedGroups(ctx)
	if err != nil {
		return
	}

	for _, group := range groups {
		response.Data = append(response.Data, *group)
	}
	return response, nil
}

func (service serviceUser) MyListNewsletter(ctx context.Context) (response domainUser.MyListNewsletterResponse, err error) {
	utils.MustLogin(whatsapp.GetClient())

	datas, err := whatsapp.GetClient().GetSubscribedNewsletters(ctx)
	if err != nil {
		return
	}

	for _, data := range datas {
		response.Data = append(response.Data, *data)
	}
	return response, nil
}

func (service serviceUser) MyPrivacySetting(ctx context.Context) (response domainUser.MyPrivacySettingResponse, err error) {
	utils.MustLogin(whatsapp.GetClient())

	resp, err := whatsapp.GetClient().TryFetchPrivacySettings(ctx, true)
	if err != nil {
		return
	}

	return mapPrivacySettings(resp), nil
}

func (service serviceUser) UpdateMyPrivacySetting(ctx context.Context, request domainUser.UpdatePrivacySettingRequest) (response domainUser.MyPrivacySettingResponse, err error) {
	utils.MustLogin(whatsapp.GetClient())

	if err = validations.ValidateUpdatePrivacySetting(ctx, request); err != nil {
		return response, err
	}

	settings, err := whatsapp.GetClient().SetPrivacySetting(
		ctx,
		types.PrivacySettingType(request.Name),
		types.PrivacySetting(request.Value),
	)
	if err != nil {
		return response, err
	}

	return mapPrivacySettings(&settings), nil
}

func (service serviceUser) MyListContacts(ctx context.Context) (response domainUser.MyListContactsResponse, err error) {
	utils.MustLogin(whatsapp.GetClient())

	contacts, err := whatsapp.GetClient().Store.Contacts.GetAllContacts(ctx)
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
	utils.MustLogin(whatsapp.GetClient())

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

	_, err = whatsapp.GetClient().SetGroupPhoto(ctx, types.JID{}, buf.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (service serviceUser) ChangePushName(ctx context.Context, request domainUser.ChangePushNameRequest) (err error) {
	utils.MustLogin(whatsapp.GetClient())

	err = whatsapp.GetClient().SendAppState(ctx, appstate.BuildSettingPushName(request.PushName))
	if err != nil {
		return err
	}
	return nil
}

func (service serviceUser) IsOnWhatsApp(ctx context.Context, request domainUser.CheckRequest) (response domainUser.CheckResponse, err error) {
	utils.MustLogin(whatsapp.GetClient())

	utils.SanitizePhone(&request.Phone)

	response.IsOnWhatsApp = utils.IsOnWhatsapp(whatsapp.GetClient(), request.Phone)

	return response, nil
}

func mapPrivacySettings(resp *types.PrivacySettings) (response domainUser.MyPrivacySettingResponse) {
	if resp == nil {
		return response
	}

	response.GroupAdd = string(resp.GroupAdd)
	response.LastSeen = string(resp.LastSeen)
	response.Status = string(resp.Status)
	response.Profile = string(resp.Profile)
	response.ReadReceipts = string(resp.ReadReceipts)
	response.CallAdd = string(resp.CallAdd)
	response.Online = string(resp.Online)
	response.Messages = string(resp.Messages)
	response.Defense = string(resp.Defense)
	response.Stickers = string(resp.Stickers)

	return response
}

func (service serviceUser) BusinessProfile(ctx context.Context, request domainUser.BusinessProfileRequest) (response domainUser.BusinessProfileResponse, err error) {
	err = validations.ValidateBusinessProfile(ctx, request)
	if err != nil {
		return response, err
	}

	dataWaRecipient, err := utils.ValidateJidWithLogin(whatsapp.GetClient(), request.Phone)
	if err != nil {
		return response, err
	}

	profile, err := whatsapp.GetClient().GetBusinessProfile(ctx, dataWaRecipient)
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
