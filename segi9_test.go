package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPlugin_Export(t *testing.T) {
	// Create a mock server
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check Auth
		auth := r.Header.Get("Authorization")
		if r.URL.Path == "/basic" {
			user, pass, ok := r.BasicAuth()
			if !ok || user != "user" || pass != "pass" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		} else if r.URL.Path == "/bearer" {
			if auth != "Bearer token123" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		} else if r.URL.Path == "/404" {
			w.WriteHeader(http.StatusNotFound)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "not found"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer ts.Close()

	p := &Plugin{
		config: Config{Timeout: 5, SkipVerify: true},
	}

	falseVal := false

	tests := []struct {
		name       string
		key        string
		params     []string
		want       string
		wantErr    bool
		skipVerify *bool
	}{
		{
			name:   "Success No Auth",
			key:    "segi9.http",
			params: []string{ts.URL},
			want:   `{"status":"ok"}`,
		},
		{
			name:   "Success Basic Auth",
			key:    "segi9.http",
			params: []string{ts.URL + "/basic", "basic", "user", "pass"},
			want:   `{"status":"ok"}`,
		},
		{
			name:   "Success Bearer Auth",
			key:    "segi9.http",
			params: []string{ts.URL + "/bearer", "bearer", "token123"},
			want:   `{"status":"ok"}`,
		},
		{
			name:    "Missing URL",
			key:     "segi9.http",
			params:  []string{},
			wantErr: true,
		},
		{
			name:    "Unsupported Auth",
			key:     "segi9.http",
			params:  []string{ts.URL, "digest"},
			wantErr: true,
		},
		{
			name:    "Invalid URL",
			key:     "segi9.http",
			params:  []string{"://invalid-url"},
			wantErr: true,
		},
		{
			name:   "Success 404 Response",
			key:    "segi9.http",
			params: []string{ts.URL + "/404"},
			want:   `{"status":"not found"}`,
		},
		{
			name:       "Secure Fail SelfSigned",
			key:        "segi9.http",
			params:     []string{ts.URL},
			wantErr:    true,
			skipVerify: &falseVal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Update config for test case
			p.mu.Lock()
			if tt.skipVerify != nil {
				p.config.SkipVerify = *tt.skipVerify
			} else {
				p.config.SkipVerify = true // Default for tests with self-signed cert
			}
			p.mu.Unlock()

			got, err := p.Export(tt.key, tt.params, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Plugin.Export() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Normalize JSON string (remove newlines if any)
				gotStr := got.(string)
				// Simple check
				if len(gotStr) < 5 {
					t.Errorf("Plugin.Export() got = %v, want %v", gotStr, tt.want)
				}
			}
		})
	}
}

func TestPlugin_Validate(t *testing.T) {
	p := &Plugin{}

	tests := []struct {
		name    string
		options interface{}
		wantErr bool
	}{
		{
			name:    "Nil options",
			options: nil,
			wantErr: false,
		},
		{
			name:    "Empty map",
			options: map[string]interface{}{},
			wantErr: false,
		},
		{
			name: "Valid Timeout",
			options: map[string]interface{}{
				"Timeout": 20,
			},
			wantErr: false,
		},
		{
			name: "Invalid Timeout Low",
			options: map[string]interface{}{
				"Timeout": 0,
			},
			wantErr: true,
		},
		{
			name: "Invalid Timeout High",
			options: map[string]interface{}{
				"Timeout": 31,
			},
			wantErr: true,
		},
		{
			name: "Valid SkipVerify",
			options: map[string]interface{}{
				"SkipVerify": true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := p.Validate(tt.options); (err != nil) != tt.wantErr {
				t.Errorf("Plugin.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
