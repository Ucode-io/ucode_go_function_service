package handlers

import (
	"context"
	"encoding/json"
	"time"
	"ucode/ucode_go_function_service/api/models"
	as "ucode/ucode_go_function_service/genproto/auth_service"
	nb "ucode/ucode_go_function_service/genproto/new_object_builder_service"
	obs "ucode/ucode_go_function_service/genproto/object_builder_service"
	"ucode/ucode_go_function_service/pkg/logger"
	"ucode/ucode_go_function_service/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) versionHistory(req *models.CreateVersionHistoryRequest) error {
	var (
		current  = map[string]any{"data": req.Current}
		previous = map[string]any{"data": req.Previous}
		request  = map[string]any{"data": req.Request}
		response = map[string]any{"data": req.Response}
		user     = ""
	)

	if req.Current == nil {
		current["data"] = make(map[string]any)
	}
	if req.Previous == nil {
		previous["data"] = make(map[string]any)
	}
	if req.Request == nil {
		request["data"] = make(map[string]any)
	}
	if req.Response == nil {
		response["data"] = make(map[string]any)
	}

	if util.IsValidUUID(req.UserInfo) {
		info, err := h.services.AuthService().User().GetUserByID(
			context.Background(), &as.UserPrimaryKey{Id: req.UserInfo},
		)
		if err == nil {
			if info.Login != "" {
				user = info.Login
			} else {
				user = info.Phone
			}
		}
	}

	_, err := req.Services.GetBuilderServiceByType(req.NodeType).VersionHistory().Create(
		context.Background(), &obs.CreateVersionHistoryRequest{
			Id:                uuid.NewString(),
			ProjectId:         req.ProjectId,
			ActionSource:      req.ActionSource,
			ActionType:        req.ActionType,
			Previus:           fromMapToString(previous),
			Current:           fromMapToString(current),
			UsedEnvrironments: req.UsedEnvironments,
			Date:              time.Now().Format("2006-01-02T15:04:05.000Z"),
			UserInfo:          user,
			Request:           fromMapToString(request),
			Response:          fromMapToString(response),
			ApiKey:            req.ApiKey,
			Type:              req.Type,
			TableSlug:         req.TableSlug,
		},
	)
	if err != nil {
		h.log.Error("Error while create version history", logger.Any("err", err))
		return err
	}
	return nil
}

func (h *Handler) versionHistoryGo(c *gin.Context, req *models.CreateVersionHistoryRequest) error {
	var (
		current  = map[string]any{"data": req.Current}
		previous = map[string]any{"data": req.Previous}
		request  = map[string]any{"data": req.Request}
		response = map[string]any{"data": req.Response}
		user     = ""
	)

	if req.Current == nil {
		current["data"] = make(map[string]any)
	}
	if req.Previous == nil {
		previous["data"] = make(map[string]any)
	}
	if req.Request == nil {
		request["data"] = make(map[string]any)
	}
	if req.Response == nil {
		response["data"] = make(map[string]any)
	}

	if util.IsValidUUID(req.UserInfo) {
		info, err := h.services.AuthService().User().GetUserByID(
			context.Background(), &as.UserPrimaryKey{Id: req.UserInfo},
		)
		if err == nil {
			if info.Login != "" {
				user = info.Login
			} else {
				user = info.Phone
			}
		}
	}

	_, err := req.Services.GoObjectBuilderService().VersionHistory().Create(
		c, &nb.CreateVersionHistoryRequest{
			Id:                uuid.NewString(),
			ProjectId:         req.ProjectId,
			ActionSource:      req.ActionSource,
			ActionType:        req.ActionType,
			Previus:           fromMapToString(previous),
			Current:           fromMapToString(current),
			UsedEnvrironments: req.UsedEnvironments,
			Date:              time.Now().Format("2006-01-02T15:04:05.000Z"),
			UserInfo:          user,
			Request:           fromMapToString(request),
			Response:          fromMapToString(response),
			ApiKey:            req.ApiKey,
			Type:              req.Type,
			TableSlug:         req.TableSlug,
			VersionId:         req.VersionId,
		},
	)
	if err != nil {
		return err
	}
	return nil
}

func fromMapToString(req map[string]any) string {
	reqString, err := json.Marshal(req)
	if err != nil {
		return ""
	}
	return string(reqString)
}
