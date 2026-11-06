package service

import "testing"

func TestUpstreamModelsURL(t *testing.T) {
	tests := []struct {
		name string
		base string
		want string
	}{
		{name: "lm studio base includes v1", base: "http://127.0.0.1:1234/v1", want: "http://127.0.0.1:1234/v1/models"},
		{name: "root base", base: "https://api.example.com", want: "https://api.example.com/v1/models"},
		{name: "trailing slash", base: "https://api.example.com/v1/", want: "https://api.example.com/v1/models"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := upstreamModelsURL(tt.base); got != tt.want {
				t.Fatalf("upstreamModelsURL(%q) = %q, want %q", tt.base, got, tt.want)
			}
		})
	}
}
