package connect_test

import (
	"testing"

	"github.com/1Password/connect-sdk-go/onepassword"

	"github.com/heartwilltell/ghsecop/internal/connect"
)

func TestParseItemPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		wantV   string
		wantI   string
		wantErr bool
	}{
		{name: "valid", path: "vaults/Infra/items/secure-node", wantV: "Infra", wantI: "secure-node"},
		{name: "uuid", path: "vaults/abc123/items/def456", wantV: "abc123", wantI: "def456"},
		{name: "trim", path: "/vaults/V/items/I/", wantV: "V", wantI: "I"},
		{name: "bad", path: "vaults/only", wantErr: true},
		{name: "empty item", path: "vaults/V/items/", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := connect.ParseItemPath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Vault != tt.wantV || got.Item != tt.wantI {
				t.Fatalf("got %+v, want vault=%s item=%s", got, tt.wantV, tt.wantI)
			}
		})
	}
}

func TestFieldMap(t *testing.T) {
	t.Parallel()

	item := &onepassword.Item{
		Fields: []*onepassword.ItemField{
			{Label: "username", Value: "admin"},
			{Label: "password", Value: "s3cret"},
			{Label: "empty", Value: ""},
			{Label: "", Value: "no-label"},
			{Label: "api_key", Value: "key-1"},
		},
	}

	all := connect.FieldMap(item, nil)
	if len(all) != 3 {
		t.Fatalf("expected 3 fields, got %d (%v)", len(all), all)
	}

	filtered := connect.FieldMap(item, []string{"password", "api_key"})
	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered fields, got %d", len(filtered))
	}
	if filtered["password"] != "s3cret" || filtered["api_key"] != "key-1" {
		t.Fatalf("unexpected filtered map: %#v", filtered)
	}
}
