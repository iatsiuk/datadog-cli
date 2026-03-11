package config_test

import (
	"testing"

	"github.com/iatsiuk/datadog-cli/internal/config"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    config.Config
		wantErr bool
	}{
		{
			name: "all vars set",
			env: map[string]string{
				"DD_API_KEY": "apikey123",
				"DD_APP_KEY": "appkey456",
				"DD_SITE":    "datadoghq.eu",
			},
			want: config.Config{
				APIKey: "apikey123",
				AppKey: "appkey456",
				Site:   "datadoghq.eu",
			},
		},
		{
			name: "site defaults to datadoghq.com",
			env: map[string]string{
				"DD_API_KEY": "apikey123",
				"DD_APP_KEY": "appkey456",
			},
			want: config.Config{
				APIKey: "apikey123",
				AppKey: "appkey456",
				Site:   "datadoghq.com",
			},
		},
		{
			name:    "missing DD_API_KEY",
			env:     map[string]string{"DD_APP_KEY": "appkey456"},
			wantErr: true,
		},
		{
			name:    "missing DD_APP_KEY",
			env:     map[string]string{"DD_API_KEY": "apikey123"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			got, err := config.LoadConfig()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
