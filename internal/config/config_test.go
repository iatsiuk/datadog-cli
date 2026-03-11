package config_test

import (
	"errors"
	"testing"

	"github.com/iatsiuk/datadog-cli/internal/config"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    config.Config
		wantErr error
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
			wantErr: config.ErrMissingAPIKey,
		},
		{
			name:    "missing DD_APP_KEY",
			env:     map[string]string{"DD_API_KEY": "apikey123"},
			wantErr: config.ErrMissingAppKey,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// clear all keys to ensure a clean environment regardless of real env
			t.Setenv("DD_API_KEY", "")
			t.Setenv("DD_APP_KEY", "")
			t.Setenv("DD_SITE", "")

			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			got, err := config.LoadConfig()
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
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
