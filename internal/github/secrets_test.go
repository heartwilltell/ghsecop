package github_test

import (
	"testing"

	ghsec "github.com/heartwilltell/ghsecop/internal/github"
)

func TestSanitizeSecretName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		prefix string
		label  string
		want   string
	}{
		{label: "password", want: "PASSWORD"},
		{label: "db-password", want: "DB_PASSWORD"},
		{label: "API Key!", want: "API_KEY"},
		{prefix: "op", label: "username", want: "OP_USERNAME"},
		{prefix: "OP_", label: "token", want: "OP_TOKEN"},
		{label: "---", want: "SECRET"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := ghsec.SanitizeSecretName(tt.prefix, tt.label)
			if got != tt.want {
				t.Fatalf("SanitizeSecretName(%q, %q) = %q, want %q", tt.prefix, tt.label, got, tt.want)
			}
		})
	}
}

func TestBuildSecretMap(t *testing.T) {
	t.Parallel()

	got := ghsec.BuildSecretMap(map[string]string{
		"username": "alice",
		"password": "s3cret",
	}, "OP")

	if got["OP_USERNAME"] != "alice" || got["OP_PASSWORD"] != "s3cret" {
		t.Fatalf("unexpected map: %#v", got)
	}
}
