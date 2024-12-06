package gitlab

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
	"ucode/ucode_go_function_service/api/status_http"
)

func CreateProjectFork(projectName string, data IntegrationData) (response GitlabIntegrationResponse, err error) {
	var (
		projectId    = data.GitlabProjectId
		strProjectId = strconv.Itoa(projectId)
	)

	resp, err := DoRequest(data.GitlabIntegrationUrl+"/api/v4/projects/"+strProjectId+"/fork", data.GitlabIntegrationToken, "POST", CreateProject{
		NamespaceID:          data.GitlabGroupId,
		Name:                 projectName,
		Path:                 projectName,
		InitializeWithReadme: true,
		DefaultBranch:        "master",
		Visibility:           "private",
	})

	if resp.Code >= 400 {
		return GitlabIntegrationResponse{}, errors.New(status_http.BadRequest.Description)
	} else if resp.Code >= 500 {
		return GitlabIntegrationResponse{}, errors.New(status_http.InternalServerError.Description)
	}

	return resp, err
}

func DoRequest(url, token string, method string, body interface{}) (responseModel GitlabIntegrationResponse, err error) {
	data, err := json.Marshal(&body)
	if err != nil {
		return
	}

	client := &http.Client{
		Timeout: time.Duration(5 * time.Second),
	}

	url += "?access_token=" + token

	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	respByte, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var emptyMap = make(map[string]interface{})

	if err = json.Unmarshal(respByte, &emptyMap); err != nil {
		return GitlabIntegrationResponse{}, err
	}

	responseModel.Message = emptyMap
	responseModel.Code = resp.StatusCode

	return
}
