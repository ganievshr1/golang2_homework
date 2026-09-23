package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGateway(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		status int
		body   string
	}{
		{"ping", http.MethodGet, "/ping", http.StatusOK, "pong"},
		{"unsupported method", http.MethodPost, "/ping", http.StatusMethodNotAllowed, "method not allowed\n"},
		{"unknown path", http.MethodGet, "/missing", http.StatusNotFound, "404 page not found\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.path, nil)
			response := httptest.NewRecorder()
			newHandler().ServeHTTP(response, request)

			if response.Code != tt.status {
				t.Fatalf("status = %d, want %d", response.Code, tt.status)
			}
			if response.Body.String() != tt.body {
				t.Errorf("body = %q, want %q", response.Body.String(), tt.body)
			}
			if tt.status == http.StatusMethodNotAllowed && response.Header().Get("Allow") != "GET" {
				t.Error("405 response must contain Allow: GET")
			}
		})
	}
}
