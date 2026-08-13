// Package connect wraps the 1Password Connect SDK for ghsecop.
package connect

import (
	"fmt"
	"strings"

	opconnect "github.com/1Password/connect-sdk-go/connect"
	"github.com/1Password/connect-sdk-go/onepassword"
)

// Client fetches items from a 1Password Connect server.
type Client interface {
	GetItem(vault, item string) (*onepassword.Item, error)
}

// SDKClient adapts the official Connect SDK.
type SDKClient struct {
	inner opconnect.Client
}

// NewFromEnvironment builds a Connect client from OP_CONNECT_HOST and OP_CONNECT_TOKEN.
func NewFromEnvironment() (*SDKClient, error) {
	client, err := opconnect.NewClientFromEnvironment()
	if err != nil {
		return nil, fmt.Errorf("create connect client: %w", err)
	}
	return &SDKClient{inner: client}, nil
}

// New builds a Connect client with an explicit host and token.
func New(host, token string) *SDKClient {
	return &SDKClient{inner: opconnect.NewClient(host, token)}
}

// GetItem retrieves a full item by vault and item id or title.
func (c *SDKClient) GetItem(vault, item string) (*onepassword.Item, error) {
	return c.inner.GetItem(item, vault)
}

// ParsedItemPath is the vault/item pair extracted from an itemPath.
type ParsedItemPath struct {
	Vault string
	Item  string
}

// ParseItemPath parses "vaults/<vault>/items/<item>" paths used by OnePasswordItem.
func ParseItemPath(itemPath string) (ParsedItemPath, error) {
	parts := strings.Split(strings.Trim(itemPath, "/"), "/")
	if len(parts) != 4 || parts[0] != "vaults" || parts[2] != "items" {
		return ParsedItemPath{}, fmt.Errorf("invalid itemPath %q; expected vaults/<vault>/items/<item>", itemPath)
	}
	if parts[1] == "" || parts[3] == "" {
		return ParsedItemPath{}, fmt.Errorf("invalid itemPath %q; vault and item must be non-empty", itemPath)
	}
	return ParsedItemPath{Vault: parts[1], Item: parts[3]}, nil
}

// FieldMap extracts label -> value for syncable fields on an item.
// Empty labels and empty values are skipped. When allowlist is non-empty,
// only labels present in the allowlist are returned.
func FieldMap(item *onepassword.Item, allowlist []string) map[string]string {
	allowed := map[string]struct{}{}
	for _, f := range allowlist {
		allowed[f] = struct{}{}
	}

	out := make(map[string]string)
	for _, field := range item.Fields {
		if field == nil {
			continue
		}
		label := strings.TrimSpace(field.Label)
		if label == "" || field.Value == "" {
			continue
		}
		if len(allowed) > 0 {
			if _, ok := allowed[label]; !ok {
				continue
			}
		}
		out[label] = field.Value
	}
	return out
}
