package function

import (
	"fmt"
	"net/http"
	"ucode/ucode_go_function_service/api/models"
	"ucode/ucode_go_function_service/config"
	"ucode/ucode_go_function_service/pkg/util"
)

type HandlerFunc func(string, config.Config, models.NewInvokeFunctionRequest) (models.InvokeFunctionResponse, error)

var FuncHandlers = map[string]HandlerFunc{
	"FUNCTION": ExecOpenFaaS,
	"KNATIVE":  ExecKnative,
}

func ExecOpenFaaS(path string, cfg config.Config, req models.NewInvokeFunctionRequest) (models.InvokeFunctionResponse, error) {
	url := fmt.Sprintf("%s%s", cfg.OpeFassBaseUrl, path)
	resp, err := util.DoRequest(url, http.MethodPost, req)
	if err != nil {
		return models.InvokeFunctionResponse{}, err
	}

	return resp, nil
}

func ExecKnative(path string, cfg config.Config, req models.NewInvokeFunctionRequest) (models.InvokeFunctionResponse, error) {
	url := fmt.Sprintf("http://%s.%s", path, cfg.KnativeBaseUrl)
	resp, err := util.DoRequest(url, http.MethodPost, req)
	if err != nil {
		return models.InvokeFunctionResponse{}, err
	}

	return resp, nil
}
