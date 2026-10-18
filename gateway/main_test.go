package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGateway(t *testing.T) {
	tests := []struct {
		method string
		path   string
		body   string
		allow  string
		status int
	}{
		{
			"GET", "/ping",
			`{"message":"pong"}`, "", 200,
		},
		{
			"POST", "/ping",
			`{"error":"method not allowed"}`, "GET", 405,
		},
		{
			"GET", "/missing",
			`{"error":"not found"}`, "", 404,
		},
		{
			"GET", "/api/transactions",
			`[]`, "", 200,
		},
		{
			"GET", "/api/budgets",
			`[]`, "", 200,
		},
		{
			"DELETE", "/api/transactions",
			`{"error":"method not allowed"}`, "GET, POST", 405,
		},
		{
			"PUT", "/api/budgets",
			`{"error":"method not allowed"}`, "GET, POST", 405,
		},
	}

	for _, tt := range tests {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(tt.method, tt.path, nil)

			newHandler().ServeHTTP(w, r)

			if w.Code != tt.status ||
				strings.TrimSpace(w.Body.String()) != tt.body {
				t.Fatalf(
					"status=%d body=%s",
					w.Code,
					w.Body.String(),
				)
			}

			if w.Header().Get("Content-Type") !=
				"application/json; charset=utf-8" ||
				!json.Valid(w.Body.Bytes()) {
				t.Fatal("response is not JSON")
			}

			if w.Header().Get("Allow") != tt.allow {
				t.Fatal("wrong Allow header")
			}
		})
	}
}
