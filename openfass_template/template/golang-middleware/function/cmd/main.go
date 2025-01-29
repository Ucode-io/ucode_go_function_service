package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"handler/function"
	"net/http"
	"net/http/httptest"

	"github.com/rs/zerolog"
)

// this part is to test the function locally
func main() {
	// Open and read the JSON file
	requestBody := map[string]interface{}{
		"data": map[string]interface{}{
			"key":  "value",
			"key2": "value2",
		},
	}

	byteValue, err := json.Marshal(requestBody)
	if err != nil {
		return
	}

	// Parse the JSON into a struct
	var requestData struct {
		Data json.RawMessage `json:"data"`
	}
	err = json.Unmarshal(byteValue, &requestData)
	if err != nil {
		return
	}

	// Create a new request
	req, err := http.NewRequest("", "", bytes.NewBuffer(requestData.Data))
	if err != nil {
		return
	}

	// Create a response recorder
	rr := httptest.NewRecorder()

	params := function.Params{
		Log:            zerolog.New(zerolog.ConsoleWriter{Out: rr.Body}).With().Timestamp().Logger(),
		CacheAvailable: false,
	}

	// Call the handler
	handler := function.NewHadler(params)

	// Invoke the handler
	handler.ServeHTTP(rr, req)

	// Print the response
	fmt.Printf("Status: %d\n", rr.Code)
	fmt.Printf("Body: %s\n", rr.Body.String())
}
