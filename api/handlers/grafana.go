package handlers

import (
	"fmt"
	"ucode/ucode_go_function_service/api/models"
	"ucode/ucode_go_function_service/api/status_http"
	"ucode/ucode_go_function_service/pkg/grafana"

	"github.com/gin-gonic/gin"
)

// Grafana godoc
// @ID grafana_function_logs
// @Router /v2/grafana/loki [POST]
// @Summary Grafana Function Logs
// @Description Grafana Function Logs
// @Tags Grafana
// @Accept json
// @Produce json
// @Param Grafana body models.GetGrafanaFunctionLogRequest true "GetGrafanaFunctionLogRequest"
// @Success 200 {object} status_http.Response{data=string} "Success"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *Handler) GetGrafanaFunctionLogs(c *gin.Context) {
	var request = models.GetGrafanaFunctionLogRequest{}

	if err := c.ShouldBindJSON(&request); err != nil {
		h.handleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	var url = fmt.Sprintf("%s/api/ds/query?ds_type=loki&requestId=explore_BWO_1", h.cfg.GrafanaBaseUrl)

	resp, err := grafana.GetFunctionLogs(request, h.cfg.GrafanaAuth, url)
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status_http.OK, resp)
}

// Grafana godoc
// @ID grafana_function_list
// @Router /v2/grafana/function [POST]
// @Summary Grafana Function List
// @Description Grafana Function List
// @Tags Grafana
// @Accept json
// @Produce json
// @Param namespace query string true "namespace"
// @Param start query string true "start"
// @Param end query string true "end"
// @Success 200 {object} status_http.Response{data=string} "Success"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *Handler) GetGrafanaFunctionList(c *gin.Context) {
	var (
		namespace = c.Query("namespace")
		start     = c.Query("start")
		end       = c.Query("end")
	)

	var url = fmt.Sprintf("%s/api/datasources/uid/loki/resources/series?match[]=%s&start=%s&end=%s", h.cfg.GrafanaBaseUrl, namespace, start, end)

	resp, err := grafana.GetFunctionList(h.cfg.GrafanaAuth, url)
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status_http.OK, resp)
}
