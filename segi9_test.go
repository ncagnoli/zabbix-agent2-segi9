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
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer ts.Close()

	p := &Plugin{
		config: Config{Timeout: 5},
	}

	tests := []struct {
		name    string
		key     string
		params  []string
		want    string
		wantErr bool
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
