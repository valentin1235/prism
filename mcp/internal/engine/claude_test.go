package engine

import (
	"fmt"
	"testing"
)

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"concurrency", fmt.Errorf("API concurrency limit reached"), true},
		{"rate limit", fmt.Errorf("rate limit exceeded"), true},
		{"timeout", fmt.Errorf("request timeout"), true},
		{"overloaded", fmt.Errorf("server overloaded"), true},
		{"temporarily", fmt.Errorf("service temporarily unavailable"), true},
		{"empty response", fmt.Errorf("empty response from CLI"), true},
		{"need retry", fmt.Errorf("need retry"), true},
		{"case insensitive", fmt.Errorf("API RATE LIMIT"), true},
		{"non-retryable", fmt.Errorf("invalid JSON schema"), false},
		{"permission denied", fmt.Errorf("permission denied"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRetryableError(tt.err)
			if got != tt.want {
				t.Errorf("IsRetryableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
