package models

type CreateFunctionRequest struct {
	Path             string `json:"path"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	CommitId         int64  `json:"-"`
	CommitGuid       string `json:"-"`
	VersionId        string `json:"-"`
	FunctionFolderId string `json:"function_folder_id"`
	FrameworkType    string `json:"framework_type"`
}

type Function struct {
	ID               string `json:"id"`
	Path             string `json:"path"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	FuncitonFolderId string `json:"function_folder_id"`
}

type InvokeFunctionResponse struct {
	Status      string                 `json:"status"`
	Data        map[string]interface{} `json:"data"`
	Attributes  map[string]interface{} `json:"attributes"`
	ServerError string                 `json:"server_error"`
}

type NewInvokeFunctionRequest struct {
	Data map[string]interface{} `json:"data"`
}

type InvokeFunctionRequest struct {
	FunctionID string   `json:"function_id"`
	ObjectIDs  []string `json:"object_ids"`
	Attributes map[string]interface{}
	TableSlug  string `json:"table_slug"`
}
