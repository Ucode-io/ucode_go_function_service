package handlers

import (
	"context"

	"ucode/ucode_go_function_service/api/models"
	status "ucode/ucode_go_function_service/api/status_http"
	pb "ucode/ucode_go_function_service/genproto/company_service"
	nb "ucode/ucode_go_function_service/genproto/new_object_builder_service"
	obs "ucode/ucode_go_function_service/genproto/object_builder_service"
	"ucode/ucode_go_function_service/pkg/helper"
	"ucode/ucode_go_function_service/pkg/util"

	"github.com/gin-gonic/gin"
)

// CreateAutomation godoc
// @Security ApiKeyAuth
// @ID create_automation
// @Router /v1/collections/{collection}/automation [POST]
// @Summary Create Automation
// @Description Create Automation
// @Tags Automation
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param Automation body obs.CreateCustomEventRequest true "AutomationRequestBody"
// @Success 201 {object} status.Response{data=obs.CustomEvent} "Automation data"
// @Response 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) CreateAutomation(c *gin.Context) {
	var (
		customevent models.CreateCustomEventRequest
		resp        *obs.CustomEvent
	)

	if err := c.ShouldBindJSON(&customevent); err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, status.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, status.BadRequest, "error getting environment id | not valid")
		return
	}

	resource, err := h.services.CompanyService().ServiceResource().GetSingle(
		ctx, &pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	structData, err := helper.ConvertMapToStruct(customevent.Attributes)
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = h.services.GetBuilderServiceByType(resource.NodeType).CustomEvent().Create(
			ctx, &obs.CreateCustomEventRequest{
				TableSlug:  customevent.TableSlug,
				EventPath:  customevent.EventPath,
				Label:      customevent.Label,
				Icon:       customevent.Icon,
				Url:        customevent.Url,
				Disable:    customevent.Disable,
				ActionType: customevent.ActionType,
				Method:     customevent.Method,
				Attributes: structData,
				ProjectId:  resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, status.Created, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := h.services.GoObjectBuilderService().CustomEvent().Create(
			ctx,
			&nb.CreateCustomEventRequest{
				TableSlug:  customevent.TableSlug,
				EventPath:  customevent.EventPath,
				Label:      customevent.Label,
				Icon:       customevent.Icon,
				Url:        customevent.Url,
				Disable:    customevent.Disable,
				ActionType: customevent.ActionType,
				Method:     customevent.Method,
				Attributes: structData,
				ProjectId:  resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, status.Created, resp)
	}

}

// GetByIdAutomation godoc
// @Security ApiKeyAuth
// @ID get_automatio_by_id
// @Router /v1/collections/{collection}/automation/{id} [GET]
// @Summary Get Automation by id
// @Description Get Automation by id
// @Tags Automation
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param id path string true "id"
// @Success 200 {object} status.Response{data=obs.CustomEvent} "AutomationBody"
// @Response 400 {object} status.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) GetByIdAutomation(c *gin.Context) {
	var customeventID = c.Param("id")

	if !util.IsValidUUID(customeventID) {
		h.handleResponse(c, status.InvalidArgument, "automation id is an invalid uuid")
		return
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, status.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, status.BadRequest, "error getting environment id | not valid")
		return
	}

	resource, err := h.services.CompanyService().ServiceResource().GetSingle(
		ctx, &pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := h.services.GetBuilderServiceByType(resource.NodeType).CustomEvent().GetSingle(
			ctx, &obs.CustomEventPrimaryKey{
				Id:        customeventID,
				ProjectId: resource.EnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, status.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := h.services.GoObjectBuilderService().CustomEvent().GetSingle(
			ctx, &nb.CustomEventPrimaryKey{
				Id:        customeventID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, status.OK, resp)
	}

}

// GetAllAutomation godoc
// @Security ApiKeyAuth
// @ID get_all_automation
// @Router /v1/collections/{collection}/automation [GET]
// @Summary Get all automation
// @Description Get all automation
// @Tags Automation
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param filters query obs.GetCustomEventsListRequest true "filters"
// @Success 200 {object} status.Response{data=obs.GetCustomEventsListResponse} "AutomationBody"
// @Response 400 {object} status.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) GetAllAutomation(c *gin.Context) {
	var resp *obs.GetCustomEventsListResponse

	authInfo, err := h.GetAuthInfo(c)
	if err != nil {
		h.handleResponse(c, status.Forbidden, err.Error())
		return
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, status.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, status.BadRequest, "error getting environment id | not valid")
		return
	}

	resource, err := h.services.CompanyService().ServiceResource().GetSingle(
		ctx, &pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = h.services.GetBuilderServiceByType(resource.NodeType).CustomEvent().GetList(
			ctx, &obs.GetCustomEventsListRequest{
				TableSlug: c.DefaultQuery("table_slug", ""),
				RoleId:    authInfo.GetRoleId(),
				ProjectId: resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, status.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := h.services.GoObjectBuilderService().CustomEvent().GetList(
			ctx, &nb.GetCustomEventsListRequest{
				TableSlug: c.DefaultQuery("table_slug", ""),
				RoleId:    authInfo.GetRoleId(),
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, status.OK, resp)
	}
}

// UpdateAutomation godoc
// @Security ApiKeyAuth
// @ID update_automation
// @Router /v1/collections/{collection}/automation [PUT]
// @Summary Update Automation
// @Description Update automation
// @Tags Automation
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param Customevent body models.CustomEvent true "UpdateAutomationRequestBody"
// @Success 204
// @Response 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) UpdateAutomation(c *gin.Context) {
	var customevent models.CustomEvent

	if err := c.ShouldBindJSON(&customevent); err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, status.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, status.BadRequest, "error getting environment id | not valid")
		return
	}

	resource, err := h.services.CompanyService().ServiceResource().GetSingle(
		ctx, &pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	structData, err := helper.ConvertMapToStruct(customevent.Attributes)
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := h.services.GetBuilderServiceByType(resource.NodeType).CustomEvent().Update(
			ctx, &obs.CustomEvent{
				Id:         customevent.Id,
				TableSlug:  customevent.TableSlug,
				EventPath:  customevent.EventPath,
				Label:      customevent.Label,
				Icon:       customevent.Icon,
				Url:        customevent.Url,
				Disable:    customevent.Disable,
				ActionType: customevent.ActionType,
				Method:     customevent.Method,
				Attributes: structData,
				ProjectId:  resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, status.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := h.services.GoObjectBuilderService().CustomEvent().Update(
			ctx, &nb.CustomEvent{
				Id:         customevent.Id,
				TableSlug:  customevent.TableSlug,
				EventPath:  customevent.EventPath,
				Label:      customevent.Label,
				Icon:       customevent.Icon,
				Url:        customevent.Url,
				Disable:    customevent.Disable,
				ActionType: customevent.ActionType,
				Method:     customevent.Method,
				Attributes: structData,
				ProjectId:  resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, status.OK, resp)
	}
}

// DeleteAutomation godoc
// @Security ApiKeyAuth
// @ID delete_automation
// @Router /v1/collections/{collection}/automation/{id} [DELETE]
// @Summary Delete Automation
// @Description Delete Automation
// @Tags Automation
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param id path string true "id"
// @Success 204
// @Response 400 {object} status.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) DeleteAutomation(c *gin.Context) {
	var customeventID = c.Param("id")

	if !util.IsValidUUID(customeventID) {
		h.handleResponse(c, status.InvalidArgument, "Customevent id is an invalid uuid")
		return
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, status.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, status.BadRequest, "error getting environment id | not valid")
		return
	}

	resource, err := h.services.CompanyService().ServiceResource().GetSingle(
		ctx, &pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := h.services.GetBuilderServiceByType(resource.NodeType).CustomEvent().Delete(
			ctx, &obs.CustomEventPrimaryKey{
				Id:        customeventID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, status.NoContent, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := h.services.GoObjectBuilderService().CustomEvent().Delete(
			ctx, &nb.CustomEventPrimaryKey{
				Id:        customeventID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, status.NoContent, resp)
	}
}
