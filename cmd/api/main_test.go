package main

import "testing"

func TestServerPort(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want int
	}{
		{name: "empty uses default", env: "", want: 3000},
		{name: "valid port", env: "8080", want: 8080},
		{name: "invalid value uses default", env: "not-a-port", want: 3000},
		{name: "out of range uses default", env: "70000", want: 3000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PORT", tt.env)
			if got := serverPort(); got != tt.want {
				t.Fatalf("serverPort() = %d, want %d", got, tt.want)
			}
		})
	}
}
