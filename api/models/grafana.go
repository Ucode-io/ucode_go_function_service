package models

type GetGrafanaFunctionLogRequest struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Namespace string `json:"namespace"`
	Function  string `json:"function"`
}

type GetGrafanaFunctionLogResponse struct {
	Results struct {
		A GetGrafanaFunctionLogA `json:"A"`
	} `json:"results"`
}

type GetGrafanaFunctionLogA struct {
	Frames []GetGrafanaFunctionLogFrames `json:"frames"`
	Status int                           `json:"status"`
}

type GetGrafanaFunctionLogFrames struct {
	Data GetGrafanaFunctionLogDatas `json:"data"`
}

type GetGrafanaFunctionLogDatas struct {
	Nanos  []any `json:"nanos"`
	Values []any
}
