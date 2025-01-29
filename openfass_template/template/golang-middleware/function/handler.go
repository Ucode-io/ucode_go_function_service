package function

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	cache "github.com/golanguzb70/redis-cache"
	sdk "github.com/golanguzb70/ucode-sdk"
	"github.com/rs/zerolog"
)

var (
	baseUrl        = "https://api.admin.u-code.io"
	functionName   = ""
	appId          = ""
	requestTimeout = 30 * time.Second
)

/*
Answer below questions before starting the function.

When the function invoked?
  - table_slug -> AFTER | BEFORE | HTTP -> CREATE | UPDATE | MULTIPLE_UPDATE | DELETE | APPEND_MANY2MANY | DELETE_MANY2MANY

What does it do?
- Explain the purpose of the function.(O'zbekcha yozilsa ham bo'ladi.)
*/

type Params struct {
	CacheClient    cache.RedisCache
	CacheAvailable bool
	Log            zerolog.Logger
}

func NewHadler(in Params) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			request       sdk.Request
			response      sdk.Response
			errorResponse sdk.ResponseError
			ucodeApi      = sdk.New(&sdk.Config{BaseURL: baseUrl, FunctionName: functionName, RequestTimeout: requestTimeout})
		)

		ucodeApi.Config().AppId = appId

		// Parse request body
		{
			requestByte, err := io.ReadAll(r.Body)
			if err != nil {
				errorResponse.ClientErrorMessage = "Error on getting request body"
				errorResponse.ErrorMessage = err.Error()
				errorResponse.StatusCode = http.StatusInternalServerError
				in.Log.Err(err).Msg("Error on getting request body")
				handleResponse(w, returnError(errorResponse), http.StatusBadRequest)
				return
			}

			err = json.Unmarshal(requestByte, &request)
			if err != nil {
				in.Log.Err(err).Msg("Error on unmarshal request")
				errorResponse.ClientErrorMessage = "Error on unmarshal request"
				errorResponse.ErrorMessage = err.Error()
				errorResponse.StatusCode = http.StatusInternalServerError
				handleResponse(w, returnError(errorResponse), http.StatusInternalServerError)
				return
			}
		}

		response.Status = "done"
		handleResponse(w, response, http.StatusOK)
	}
}

func returnError(errorResponse sdk.ResponseError) interface{} {
	return sdk.Response{
		Status: "error",
		Data:   map[string]interface{}{"message": errorResponse.ClientErrorMessage, "error": errorResponse.ErrorMessage},
	}
}

func handleResponse(w http.ResponseWriter, body interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")

	bodyByte, err := json.Marshal(body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`
			{
				"error": "Error marshalling response"
			}
		`))
		return
	}

	w.WriteHeader(statusCode)
	w.Write(bodyByte)
}
