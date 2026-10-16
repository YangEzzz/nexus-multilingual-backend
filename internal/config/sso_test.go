package config

import "testing"

func TestAutomaticSSORedirect(t *testing.T) {
	for _, tc := range []struct {
		name, frontend, api, override, want string
	}{
		{"local proxy", "http://localhost:5173", "", "", "http://localhost:5173/api/v1/auth/sso/callback"},
		{"different local port", "http://127.0.0.1:6200/", "", "", "http://127.0.0.1:6200/api/v1/auth/sso/callback"},
		{"IPv6", "http://[::1]:5173", "", "", "http://[::1]:5173/api/v1/auth/sso/callback"},
		{"same-origin deployment", "https://nexus.example", "", "", "https://nexus.example/api/v1/auth/sso/callback"},
		{"separate API deployment", "https://nexus.example", "https://api.example/", "", "https://api.example/api/v1/auth/sso/callback"},
		{"existing callback preserved", "https://nexus.example", "https://api.example", "https://legacy.example/custom/callback", "https://legacy.example/custom/callback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveSSORedirect(tc.frontend, tc.api, tc.override)
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestAutomaticSSORedirectRejectsInvalidOrigin(t *testing.T) {
	for _, origin := range []string{"", "//api.example", "http://api.example", "http://localhost.evil.example", "https://user:pass@api.example", "https://api.example?target=evil", "https://api.example#fragment", "https://api.example/api/v1", "https://api.example?"} {
		if _, err := resolveSSORedirect(origin, "", ""); err == nil {
			t.Errorf("accepted invalid origin %q", origin)
		}
	}
}
