// Package problem writes RFC 9457 problem details (`application/problem+json`),
// the error format of every VaxBook API (SAD §8).
package problem

import (
	"encoding/json"
	"net/http"
)

// ContentType is the media type of a problem response.
const ContentType = "application/problem+json"

// Problem is an RFC 9457 problem details object.
type Problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// New returns a problem of type about:blank whose title is the status text.
func New(status int, detail string) Problem {
	return Problem{Type: "about:blank", Title: http.StatusText(status), Status: status, Detail: detail}
}

// Write sends a problem of type about:blank with the given status and detail.
func Write(w http.ResponseWriter, status int, detail string) {
	WriteProblem(w, New(status, detail))
}

// WriteProblem sends p as the response.
func WriteProblem(w http.ResponseWriter, p Problem) {
	w.Header().Set("Content-Type", ContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}
