package grafana

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"ucode/ucode_go_function_service/api/models"

	"github.com/spf13/cast"
)

func GetFunctionLogs(req models.GetGrafanaFunctionLogRequest, token, url string) ([]string, error) {
	var (
		paylod        map[string]any
		response      models.GetGrafanaFunctionLogResponse
		payloadString string
	)

	if req.Namespace == "knative-fn" {
		payloadString = fmt.Sprintf(`{
			"queries": [
				{
					"refId": "loki-data-samples",
					"expr": "{namespace=\"%s\", app=~\"%s.*\"}",
					"queryType": "range",
					"datasource": {
						"type": "loki",
						"uid": "loki"
					},
					"editorMode": "code",
					"maxLines": 1000,
					"legendFormat": "",
					"datasourceId": 1,
					"intervalMs": 5000,
					"maxDataPoints": 855
				}
			],
			"from": "%s",
			"to": "%s"
		}`, req.Namespace, req.Function, req.From, req.To)
	} else {
		payloadString = fmt.Sprintf(`{
			"queries": [
				{
					"refId": "loki-data-samples",
					"expr": "{namespace=\"%s\", app=\"%s\"}",
					"queryType": "range",
					"datasource": {
						"type": "loki",
						"uid": "loki"
					},
					"editorMode": "code",
					"maxLines": 1000,
					"legendFormat": "",
					"datasourceId": 1,
					"intervalMs": 5000,
					"maxDataPoints": 855
				}
			],
			"from": "%s",
			"to": "%s"
		}`, req.Namespace, req.Function, req.From, req.To)
	}

	if err := json.Unmarshal([]byte(payloadString), &paylod); err != nil {
		return []string{}, err
	}

	respByte, err := DoRequest(http.MethodPost, url, token, paylod)
	if err != nil {
		return []string{}, err
	}

	if err := json.Unmarshal(respByte, &response); err != nil {
		return []string{}, err
	}

	if len(response.Results.A.Frames) == 0 {
		return []string{}, nil
	}

	if len(response.Results.A.Frames[0].Data.Values) < 2 {
		return []string{}, nil
	}

	return cast.ToStringSlice(response.Results.A.Frames[0].Data.Values[2]), nil
}

func GetFunctionList(token, url string) (any, error) {
	var response any

	resp, err := DoRequest(http.MethodGet, url, token, nil)
	if err != nil {
		return response, err
	}

	if err := json.Unmarshal(resp, &response); err != nil {
		return response, err
	}

	return response, nil
}

func DoRequest(method, url, token string, payload map[string]any) ([]byte, error) {
	var (
		reqBody      = new(bytes.Buffer)
		encodedToken = base64.StdEncoding.EncodeToString([]byte(token))
	)

	if payload != nil {
		if err := json.NewEncoder(reqBody).Encode(payload); err != nil {
			return []byte{}, err
		}
	}

	client := http.Client{
		Timeout: time.Duration(time.Second * 30),
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Basic "+encodedToken)

	res, err := client.Do(req)
	if err != nil {
		return []byte{}, err
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return []byte{}, err
	}

	return body, nil
}
