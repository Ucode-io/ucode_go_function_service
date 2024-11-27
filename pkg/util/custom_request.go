package util

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
	"ucode/ucode_go_function_service/api/models"
)

func DoRequest(url string, method string, body interface{}) (responseModel models.InvokeFunctionResponse, err error) {
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

func DoRequestCheckCodeServer(url string, method string, body interface{}) (status int, err error) {
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
