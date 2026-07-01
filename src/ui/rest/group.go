package rest

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	domainGroup "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/group"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/helpers"
	"github.com/gofiber/fiber/v2"
	"go.mau.fi/whatsmeow"
)

type Group struct {
	Service domainGroup.IGroupUsecase
}

func InitRestGroup(app fiber.Router, service domainGroup.IGroupUsecase) Group {
	rest := Group{Service: service}
	app.Post("/group", rest.CreateGroup)
	app.Post("/group/join-with-link", rest.JoinGroupWithLink)
	app.Get("/group/info-from-link", rest.GetGroupInfoFromLink)
	app.Get("/group/info", rest.GroupInfo)
	app.Post("/group/leave", rest.LeaveGroup)
	app.Get("/group/participants", rest.ListParticipants)
	app.Get("/group/participants/export", rest.ExportParticipants)
	app.Post("/group/participants", rest.AddParticipants)
	app.Post("/group/participants/remove", rest.DeleteParticipants)
	app.Post("/group/participants/promote", rest.PromoteParticipants)
	app.Post("/group/participants/demote", rest.DemoteParticipants)
	app.Get("/group/participant-requests", rest.ListParticipantRequests)
	app.Post("/group/participant-requests/approve", rest.ApproveParticipantRequests)
	app.Post("/group/participant-requests/reject", rest.RejectParticipantRequests)
	app.Post("/group/photo", rest.SetGroupPhoto)
	app.Post("/group/name", rest.SetGroupName)
	app.Post("/group/locked", rest.SetGroupLocked)
	app.Post("/group/announce", rest.SetGroupAnnounce)
	app.Post("/group/topic", rest.SetGroupTopic)
	app.Get("/group/invite-link", rest.GetGroupInviteLink)
	app.Post("/group/join-approval", rest.SetGroupJoinApprovalMode)
	app.Post("/group/member-add-mode", rest.SetGroupMemberAddMode)
	app.Get("/group/sub-groups", rest.GetSubGroups)
	app.Post("/group/link", rest.LinkGroup)
	app.Post("/group/unlink", rest.UnlinkGroup)
	app.Get("/group/linked-participants", rest.GetLinkedParticipants)
	return rest
}

func (controller *Group) JoinGroupWithLink(c *fiber.Ctx) error {
	var request domainGroup.JoinGroupWithLinkRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	response, err := controller.Service.JoinGroupWithLink(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Success joined group",
		Results: map[string]string{
			"group_id": response,
		},
	})
}

func (controller *Group) GetGroupInfoFromLink(c *fiber.Ctx) error {
	var request domainGroup.GetGroupInfoFromLinkRequest
	if err := c.QueryParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid query parameters: "+err.Error())
	}

	response, err := controller.Service.GetGroupInfoFromLink(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success get group info from link", response)
}

func (controller *Group) LeaveGroup(c *fiber.Ctx) error {
	var request domainGroup.LeaveGroupRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	utils.SanitizePhone(&request.GroupID)

	err := controller.Service.LeaveGroup(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success leave group", nil)
}

func (controller *Group) CreateGroup(c *fiber.Ctx) error {
	var request domainGroup.CreateGroupRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	groupID, err := controller.Service.CreateGroup(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: fmt.Sprintf("Success created group with id %s", groupID),
		Results: map[string]string{
			"group_id": groupID,
		},
	})
}

func (controller *Group) ListParticipants(c *fiber.Ctx) error {
	var request domainGroup.GetGroupParticipantsRequest
	if err := c.QueryParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid query parameters: "+err.Error())
	}

	if request.GroupID == "" {
		return helpers.HandleBadRequest(c, "Group ID cannot be empty")
	}

	utils.SanitizePhone(&request.GroupID)

	result, err := controller.Service.GetGroupParticipants(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success getting group participants", result)
}

func (controller *Group) ExportParticipants(c *fiber.Ctx) error {
	var request domainGroup.GetGroupParticipantsRequest
	if err := c.QueryParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid query parameters: "+err.Error())
	}

	if request.GroupID == "" {
		return helpers.HandleBadRequest(c, "Group ID cannot be empty")
	}

	utils.SanitizePhone(&request.GroupID)

	result, err := controller.Service.GetGroupParticipants(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)

	if err := writer.Write([]string{"participant_jid", "phone_number", "lid", "display_name", "role"}); err != nil {
		return helpers.HandleError(c, err)
	}

	for _, participant := range result.Participants {
		role := "member"
		if participant.IsSuperAdmin {
			role = "super_admin"
		} else if participant.IsAdmin {
			role = "admin"
		}

		record := []string{
			participant.JID,
			participant.PhoneNumber,
			participant.LID,
			participant.DisplayName,
			role,
		}

		if err := writer.Write(record); err != nil {
			return helpers.HandleError(c, err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return helpers.HandleError(c, err)
	}

	fileName := fmt.Sprintf("group-%s-participants.csv", strings.ReplaceAll(result.GroupID, "@", "_"))

	c.Type("text/csv; charset=utf-8")
	c.Attachment(fileName)

	return c.Send(buffer.Bytes())
}

func (controller *Group) AddParticipants(c *fiber.Ctx) error {
	return controller.manageParticipants(c, whatsmeow.ParticipantChangeAdd, "Success add participants")
}

func (controller *Group) DeleteParticipants(c *fiber.Ctx) error {
	return controller.manageParticipants(c, whatsmeow.ParticipantChangeRemove, "Success delete participants")
}

func (controller *Group) PromoteParticipants(c *fiber.Ctx) error {
	return controller.manageParticipants(c, whatsmeow.ParticipantChangePromote, "Success promote participants")
}

func (controller *Group) DemoteParticipants(c *fiber.Ctx) error {
	return controller.manageParticipants(c, whatsmeow.ParticipantChangeDemote, "Success demote participants")
}

func (controller *Group) ListParticipantRequests(c *fiber.Ctx) error {
	var request domainGroup.GetGroupRequestParticipantsRequest
	if err := c.QueryParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid query parameters: "+err.Error())
	}

	if request.GroupID == "" {
		return helpers.HandleBadRequest(c, "Group ID cannot be empty")
	}

	utils.SanitizePhone(&request.GroupID)

	result, err := controller.Service.GetGroupRequestParticipants(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success getting list requested participants", result)
}

func (controller *Group) ApproveParticipantRequests(c *fiber.Ctx) error {
	return controller.handleRequestedParticipants(c, whatsmeow.ParticipantChangeApprove, "Success approve requested participants")
}

func (controller *Group) RejectParticipantRequests(c *fiber.Ctx) error {
	return controller.handleRequestedParticipants(c, whatsmeow.ParticipantChangeReject, "Success reject requested participants")
}

// Generalized participant management handler
func (controller *Group) manageParticipants(c *fiber.Ctx, action whatsmeow.ParticipantChange, successMsg string) error {
	var request domainGroup.ParticipantRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	utils.SanitizePhone(&request.GroupID)
	request.Action = action

	result, err := controller.Service.ManageParticipant(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, successMsg, result)
}

// Generalized requested participants handler
func (controller *Group) handleRequestedParticipants(c *fiber.Ctx, action whatsmeow.ParticipantRequestChange, successMsg string) error {
	var request domainGroup.GroupRequestParticipantsRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	utils.SanitizePhone(&request.GroupID)
	request.Action = action

	result, err := controller.Service.ManageGroupRequestParticipants(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, successMsg, result)
}

func (controller *Group) SetGroupPhoto(c *fiber.Ctx) error {
	var request domainGroup.SetGroupPhotoRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	utils.SanitizePhone(&request.GroupID)

	file, err := c.FormFile("photo")
	if err == nil {
		logrus.Printf("INFO: Received group photo - Filename: %s, Size: %d bytes, ContentType: %s",
			file.Filename, file.Size, file.Header.Get("Content-Type"))

		// Basic validation only - processing will be done in usecase
		if err := utils.ValidateGroupPhotoFormat(file); err != nil {
			logrus.Printf("ERROR: Group photo validation failed - %v", err)
			return helpers.HandleBadRequest(c, fmt.Sprintf("Image validation failed: %v", err))
		}

		request.Photo = file
	} else {
		logrus.Printf("DEBUG: No photo file provided - Error: %v", err)
	}

	pictureID, err := controller.Service.SetGroupPhoto(c.UserContext(), request)
	if err != nil {
		logrus.Printf("ERROR: WhatsApp service failed to set group photo - %v", err)
		return helpers.HandleError(c, err)
	}

	message := "Success update group photo"
	if request.Photo == nil {
		message = "Success remove group photo"
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: message,
		Results: domainGroup.SetGroupPhotoResponse{
			PictureID: pictureID,
			Message:   message,
		},
	})
}

func (controller *Group) SetGroupName(c *fiber.Ctx) error {
	var request domainGroup.SetGroupNameRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	utils.SanitizePhone(&request.GroupID)

	err := controller.Service.SetGroupName(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, fmt.Sprintf("Success update group name to '%s'", request.Name), nil)
}

func (controller *Group) SetGroupLocked(c *fiber.Ctx) error {
	var request domainGroup.SetGroupLockedRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	utils.SanitizePhone(&request.GroupID)

	err := controller.Service.SetGroupLocked(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	message := "Success set group as unlocked"
	if request.Locked {
		message = "Success set group as locked"
	}

	return helpers.HandleSuccess(c, message, nil)
}

func (controller *Group) SetGroupAnnounce(c *fiber.Ctx) error {
	var request domainGroup.SetGroupAnnounceRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	utils.SanitizePhone(&request.GroupID)

	err := controller.Service.SetGroupAnnounce(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	message := "Success disable announce mode"
	if request.Announce {
		message = "Success enable announce mode"
	}

	return helpers.HandleSuccess(c, message, nil)
}

func (controller *Group) SetGroupTopic(c *fiber.Ctx) error {
	var request domainGroup.SetGroupTopicRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	utils.SanitizePhone(&request.GroupID)

	err := controller.Service.SetGroupTopic(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	message := "Success update group topic"
	if request.Topic == "" {
		message = "Success remove group topic"
	}

	return helpers.HandleSuccess(c, message, nil)
}

// GroupInfo handles the /group/info endpoint to fetch group information
func (controller *Group) GroupInfo(c *fiber.Ctx) error {
	var request domainGroup.GroupInfoRequest
	if err := c.QueryParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid query parameters: "+err.Error())
	}

	utils.SanitizePhone(&request.GroupID)

	response, err := controller.Service.GroupInfo(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success get group info", response.Data)
}

func (controller *Group) GetGroupInviteLink(c *fiber.Ctx) error {
	var request domainGroup.GetGroupInviteLinkRequest
	if err := c.QueryParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid query parameters: "+err.Error())
	}

	utils.SanitizePhone(&request.GroupID)

	response, err := controller.Service.GetGroupInviteLink(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}

	return helpers.HandleSuccess(c, "Success get group invite link", response)
}

func (controller *Group) SetGroupJoinApprovalMode(c *fiber.Ctx) error {
	var request domainGroup.SetGroupJoinApprovalModeRequest
	if err := c.BodyParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid request body: "+err.Error())
	}

	utils.SanitizePhone(&request.GroupID)

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

	utils.SanitizePhone(&request.GroupID)

	if err := controller.Service.SetGroupMemberAddMode(c.UserContext(), request); err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, "Success set group member add mode", nil)
}

func (controller *Group) GetSubGroups(c *fiber.Ctx) error {
	var request domainGroup.CommunityRequest
	if err := c.QueryParser(&request); err != nil {
		return helpers.HandleBadRequest(c, "Invalid query parameters: "+err.Error())
	}
	utils.SanitizePhone(&request.CommunityID)
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
	utils.SanitizePhone(&request.CommunityID)
	utils.SanitizePhone(&request.GroupID)
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
	utils.SanitizePhone(&request.CommunityID)
	utils.SanitizePhone(&request.GroupID)
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
	utils.SanitizePhone(&request.CommunityID)
	response, err := controller.Service.GetLinkedParticipants(c.UserContext(), request)
	if err != nil {
		return helpers.HandleError(c, err)
	}
	return helpers.HandleSuccess(c, "Success get linked participants", response)
}
