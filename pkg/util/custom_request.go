package util

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
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
		log.Printf("Failed to marshal request body: %v", err)
		return nil, http.StatusBadRequest, err
	}

	client := &http.Client{}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		return nil, http.StatusBadRequest, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	log.Printf("Final Request:\nMETHOD: %s\nURL: %s\nHEADERS: %v\nBODY: %s\n", req.Method, req.URL.String(), req.Header, string(data))

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Request failed: %v", err)
		return nil, http.StatusBadRequest, err
	}
	defer resp.Body.Close()

	log.Printf("Response Status: %s %s", resp.Status, url)
	log.Printf("Response Headers: %v %v", resp.Header, url)

	respHeader := resp.Header.Get("Content-Encoding")
	var respBody []byte
	switch respHeader {
	case "gzip":
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			log.Printf("Failed to create gzip reader: %v", err)
			return nil, http.StatusInternalServerError, err
		}
		defer reader.Close()

		respBody, err = io.ReadAll(reader)
		if err != nil {
			log.Printf("Failed to read gzipped response body: %v", err)
			return nil, http.StatusInternalServerError, err
		}
	default:
		respBody, err = io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Failed to read response body: %v", err)
			return nil, http.StatusInternalServerError, err
		}
	}

	log.Printf("Response Body: %s %v", string(respBody), url)

	responseModel := make(map[string]any)
	err = json.Unmarshal(respBody, &responseModel)
	if err != nil {
		log.Printf("Failed to unmarshal response body: %v %v", err, url)
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
