package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"ucode/ucode_go_function_service/api/models"
	"ucode/ucode_go_function_service/api/status_http"
	"ucode/ucode_go_function_service/config"
	"ucode/ucode_go_function_service/genproto/auth_service"
	"ucode/ucode_go_function_service/genproto/new_object_builder_service"
	"ucode/ucode_go_function_service/genproto/object_builder_service"
	"ucode/ucode_go_function_service/pkg/helper"
	"ucode/ucode_go_function_service/pkg/logger"
	"ucode/ucode_go_function_service/pkg/util"
	"ucode/ucode_go_function_service/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	cfg      config.Config
	log      logger.LoggerI
	services services.ServiceManagerI
}

func NewHandler(cfg config.Config, log logger.LoggerI, svcs services.ServiceManagerI) Handler {
	return Handler{
		cfg:      cfg,
		log:      log,
		services: svcs,
	}
}

func (h *Handler) handleResponse(c *gin.Context, status status_http.Status, data interface{}) {
	switch code := status.Code; {
	case code < 400:
	default:
		h.log.Error(
			"response",
			logger.Int("code", status.Code),
			logger.String("status", status.Status),
			logger.Any("description", status.Description),
			logger.Any("data", data),
			logger.Any("custom_message", status.CustomMessage),
		)
	}

	c.JSON(status.Code, status_http.Response{
		Status:        status.Status,
		Description:   status.Description,
		Data:          data,
		CustomMessage: status.CustomMessage,
	})
}

func (h *Handler) getOffsetParam(c *gin.Context) (offset int, err error) {
	offsetStr := c.DefaultQuery("offset", h.cfg.DefaultOffset)
	return strconv.Atoi(offsetStr)
}

func (h *Handler) getLimitParam(c *gin.Context) (limit int, err error) {
	limitStr := c.DefaultQuery("limit", h.cfg.DefaultLimit)
	return strconv.Atoi(limitStr)
}

func (h *Handler) getPageParam(c *gin.Context) (page int, err error) {
	pageStr := c.DefaultQuery("page", "1")
	return strconv.Atoi(pageStr)
}

func (h *Handler) AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			ok          bool
			res         = &auth_service.HasAccessSuperAdminRes{}
			data        = make(map[string]interface{})
			bearerToken = c.GetHeader("Authorization")
			strArr      = strings.Split(bearerToken, " ")
		)

		if len(strArr) < 1 && (strArr[0] != "Bearer" && strArr[0] != "API-KEY") {
			h.log.Error("---ERR->Unexpected token format")
			_ = c.AbortWithError(http.StatusForbidden, errors.New("token error: wrong format"))
			return
		}

		switch strArr[0] {
		case "Bearer":
			res, ok = h.adminHasAccess(c)
			if !ok {
				_ = c.AbortWithError(401, errors.New("unauthorized"))
				return
			}

			var (
				resourceId    = c.GetHeader("Resource-Id")
				environmentId = c.GetHeader("Environment-Id")
				projectId     = c.DefaultQuery("project-id", "")
			)

			if res.ProjectId != "" {
				projectId = res.ProjectId
			}
			if res.EnvId != "" {
				environmentId = res.EnvId
			}

			apiJson, err := json.Marshal(res)
			if err != nil {
				h.handleResponse(c, status_http.BadRequest, "cant get auth info")
				c.Abort()
				return
			}

			err = json.Unmarshal(apiJson, &data)
			if err != nil {
				h.handleResponse(c, status_http.BadRequest, "cant get auth info")
				c.Abort()
				return
			}

			c.Set("auth", models.AuthData{
				Type: "BEARER",
				Data: data,
			})

			c.Set("environment_id", environmentId)
			c.Set("resource_id", resourceId)
			c.Set("project_id", projectId)
		case "API-KEY":
			var appId = c.GetHeader("X-API-KEY")

			apiKey, err := h.services.AuthService().ApiKey().GetEnvID(
				c.Request.Context(),
				&auth_service.GetReq{
					Id: appId,
				},
			)
			if err != nil {
				h.handleResponse(c, status_http.BadRequest, err.Error())
				c.Abort()
				return
			}

			apiJson, err := json.Marshal(apiKey)
			if err != nil {
				h.handleResponse(c, status_http.BadRequest, "cant get auth info")
				c.Abort()
				return
			}

			if err = json.Unmarshal(apiJson, &data); err != nil {
				h.handleResponse(c, status_http.BadRequest, "cant get auth info")
				c.Abort()
				return
			}

			c.Set("auth", models.AuthData{
				Type: "API-KEY",
				Data: data,
			})
			c.Set("environment_id", apiKey.GetEnvironmentId())
			c.Set("project_id", apiKey.GetProjectId())
		default:
			err := errors.New("error invalid authorization method")
			h.log.Error("--AuthMiddleware--", logger.Error(err))
			h.handleResponse(c, status_http.BadRequest, err.Error())
			c.Abort()
		}

		c.Set("Auth_Admin", res)

		c.Next()
	}
}

func (h *Handler) adminHasAccess(c *gin.Context) (*auth_service.HasAccessSuperAdminRes, bool) {
	var (
		bearerToken = c.GetHeader("Authorization")
		strArr      = strings.Split(bearerToken, " ")
	)

	if len(strArr) != 2 || strArr[0] != "Bearer" {
		h.handleResponse(c, status_http.Forbidden, "token error: wrong format")
		return nil, false
	}

	var accessToken = strArr[1]

	service, conn, err := h.services.AuthService().Session(c)
	if err != nil {
		return nil, false
	}
	defer conn.Close()

	resp, err := service.HasAccessSuperAdmin(
		c.Request.Context(),
		&auth_service.HasAccessSuperAdminReq{
			AccessToken: accessToken,
			Path:        helper.GetURLWithTableSlug(c),
			Method:      c.Request.Method,
		},
	)
	if err != nil {
		errr := status.Error(codes.PermissionDenied, "Permission denied")
		if errr.Error() == err.Error() {
			h.handleResponse(c, status_http.BadRequest, err.Error())
			return nil, false
		}
		errr = status.Error(codes.InvalidArgument, "User has been expired")
		if errr.Error() == err.Error() {
			h.handleResponse(c, status_http.Forbidden, err.Error())
			return nil, false
		}
		h.handleResponse(c, status_http.Unauthorized, err.Error())
		return nil, false
	}

	return resp, true
}

func (h *Handler) adminAuthInfo(c *gin.Context) (result *auth_service.HasAccessSuperAdminRes, err error) {
	data, ok := c.Get("Auth_Admin")
	if !ok {
		h.handleResponse(c, status_http.Forbidden, "token error: wrong format")
		c.Abort()
		return nil, errors.New("token error: wrong format")
	}

	accessResponse, ok := data.(*auth_service.HasAccessSuperAdminRes)
	if !ok {
		h.handleResponse(c, status_http.Forbidden, "token error: wrong format")
		c.Abort()
		return nil, errors.New("token error: wrong format")
	}

	return accessResponse, nil
}

func (h *Handler) versionHistory(req *models.CreateVersionHistoryRequest) error {
	var (
		current  = map[string]interface{}{"data": req.Current}
		previous = map[string]interface{}{"data": req.Previous}
		request  = map[string]interface{}{"data": req.Request}
		response = map[string]interface{}{"data": req.Response}
		user     = ""
	)

	if req.Current == nil {
		current["data"] = make(map[string]interface{})
	}
	if req.Previous == nil {
		previous["data"] = make(map[string]interface{})
	}
	if req.Request == nil {
		request["data"] = make(map[string]interface{})
	}
	if req.Response == nil {
		response["data"] = make(map[string]interface{})
	}

	if util.IsValidUUID(req.UserInfo) {
		info, err := h.authService.User().GetUserByID(
			context.Background(),
			&auth_service.UserPrimaryKey{
				Id: req.UserInfo,
			},
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
		context.Background(),
		&object_builder_service.CreateVersionHistoryRequest{
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
		current  = map[string]interface{}{"data": req.Current}
		previous = map[string]interface{}{"data": req.Previous}
		request  = map[string]interface{}{"data": req.Request}
		response = map[string]interface{}{"data": req.Response}
		user     = ""
	)

	if req.Current == nil {
		current["data"] = make(map[string]interface{})
	}
	if req.Previous == nil {
		previous["data"] = make(map[string]interface{})
	}
	if req.Request == nil {
		request["data"] = make(map[string]interface{})
	}
	if req.Response == nil {
		response["data"] = make(map[string]interface{})
	}

	if util.IsValidUUID(req.UserInfo) {
		info, err := h.authService.User().GetUserByID(
			context.Background(),
			&auth_service.UserPrimaryKey{
				Id: req.UserInfo,
			},
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
		c,
		&new_object_builder_service.CreateVersionHistoryRequest{
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
		log.Println("ERROR FROM VERSION CREATE >>>>>", err)
		return err
	}
	return nil
}

func fromMapToString(req map[string]interface{}) string {
	reqString, err := json.Marshal(req)
	if err != nil {
		return ""
	}
	return string(reqString)
}
