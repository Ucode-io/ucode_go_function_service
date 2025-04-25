package util

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"time"
	"ucode/ucode_go_function_service/api/models"
)

func DoRequest(url string, method string, body any) (responseModel models.InvokeFunctionResponse, err error) {
	data, err := json.Marshal(&body)
	if err != nil {
		return
	}
	client := &http.Client{}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	respByte, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(respByte, &responseModel)
	if err != nil {
		return
	}

	return
}

func DoDynamicRequest(url string, headers map[string]string, method string, body any) (map[string]any, int, error) {
	data, err := json.Marshal(&body)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	respBody := make([]byte, 0)

	client := &http.Client{}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	defer resp.Body.Close()

	respHeader := resp.Header.Get("Content-Encoding")
	switch respHeader {
	case "gzip":
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		defer reader.Close()

		respBody, err = io.ReadAll(reader)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
	default:
		respBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}

	}

	responseModel := make(map[string]any)

	err = json.Unmarshal(respBody, &responseModel)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return responseModel, resp.StatusCode, nil
}

func DoRequestCheckCodeServer(url string, method string, body any) (status int, err error) {
	data, err := json.Marshal(&body)
	if err != nil {
		return
	}
	client := &http.Client{
		Timeout: time.Duration(5 * time.Second),
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	status = resp.StatusCode
	defer resp.Body.Close()

	return
}
