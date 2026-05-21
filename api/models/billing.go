package models

type PaymentRequiredData struct {
	Type string `json:"type"`          // always "payment_required"
	Code string `json:"code"`          // "function_limit" | "microfrontend_limit"
	Unit string `json:"unit,omitempty"` // "functions" | "microfrontends"
}