package handlers

import (
	"strconv"
	"ucode/ucode_go_function_service/api/status_http"
	"ucode/ucode_go_function_service/config"
	"ucode/ucode_go_function_service/pkg/logger"
	"ucode/ucode_go_function_service/services"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	config   config.Config
	log      logger.LoggerI
	services services.ServiceManagerI
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
	offsetStr := c.DefaultQuery("offset", h.config.DefaultOffset)
	return strconv.Atoi(offsetStr)
}

func (h *Handler) getLimitParam(c *gin.Context) (limit int, err error) {
	limitStr := c.DefaultQuery("limit", h.config.DefaultLimit)
	return strconv.Atoi(limitStr)
}

func (h *Handler) getPageParam(c *gin.Context) (page int, err error) {
	pageStr := c.DefaultQuery("page", "1")
	return strconv.Atoi(pageStr)
}
