// Package github syncs encrypted Actions secrets to organization repositories.
package github

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"github.com/google/go-github/v66/github"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/oauth2"
)

// RepoRef identifies a repository within an organization.
type RepoRef struct {
	Owner string
	Name  string
	ID    int64
}

// ListOptions controls which organization repositories are included.
type ListOptions struct {
	Organization        string
	Repositories        []string
	ExcludeRepositories []string
	IncludeArchived     bool
	IncludeForks        bool
}

// SecretSyncer writes Actions repository secrets.
type SecretSyncer interface {
	ListTargetRepos(ctx context.Context, opts ListOptions) ([]RepoRef, error)
	UpsertRepoSecrets(ctx context.Context, repo RepoRef, secrets map[string]string) error
}

// Client talks to the GitHub REST API.
type Client struct {
	inner *github.Client
}

// NewClient builds a GitHub API client from a personal access token or GitHub App token.
func NewClient(token string) *Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(context.Background(), ts)
	return &Client{inner: github.NewClient(httpClient)}
}

// NewClientWithHTTP is useful for tests.
func NewClientWithHTTP(httpClient *http.Client) *Client {
	return &Client{inner: github.NewClient(httpClient)}
}

// ListTargetRepos returns repositories that should receive secrets.
func (c *Client) ListTargetRepos(ctx context.Context, opts ListOptions) ([]RepoRef, error) {
	if opts.Organization == "" {
		return nil, fmt.Errorf("organization is required")
	}

	exclude := toSet(opts.ExcludeRepositories)

	if len(opts.Repositories) > 0 {
		out := make([]RepoRef, 0, len(opts.Repositories))
		for _, name := range opts.Repositories {
			if _, skip := exclude[name]; skip {
				continue
			}
			repo, _, err := c.inner.Repositories.Get(ctx, opts.Organization, name)
			if err != nil {
				return nil, fmt.Errorf("get repository %s/%s: %w", opts.Organization, name, err)
			}
			out = append(out, RepoRef{
				Owner: opts.Organization,
				Name:  repo.GetName(),
				ID:    repo.GetID(),
			})
		}
		return out, nil
	}

	var out []RepoRef
	listOpts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
		Type:        "all",
	}
	for {
		repos, resp, err := c.inner.Repositories.ListByOrg(ctx, opts.Organization, listOpts)
		if err != nil {
			return nil, fmt.Errorf("list repositories for org %s: %w", opts.Organization, err)
		}
		for _, repo := range repos {
			name := repo.GetName()
			if _, skip := exclude[name]; skip {
				continue
			}
			if repo.GetArchived() && !opts.IncludeArchived {
				continue
			}
			if repo.GetFork() && !opts.IncludeForks {
				continue
			}
			out = append(out, RepoRef{
				Owner: opts.Organization,
				Name:  name,
				ID:    repo.GetID(),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		listOpts.Page = resp.NextPage
	}
	return out, nil
}

// UpsertRepoSecrets encrypts and creates/updates repository Actions secrets.
func (c *Client) UpsertRepoSecrets(ctx context.Context, repo RepoRef, secrets map[string]string) error {
	if len(secrets) == 0 {
		return nil
	}

	pub, _, err := c.inner.Actions.GetRepoPublicKey(ctx, repo.Owner, repo.Name)
	if err != nil {
		return fmt.Errorf("get public key for %s/%s: %w", repo.Owner, repo.Name, err)
	}

	for name, value := range secrets {
		encrypted, err := encryptSecret(pub.GetKey(), value)
		if err != nil {
			return fmt.Errorf("encrypt secret %s for %s/%s: %w", name, repo.Owner, repo.Name, err)
		}
		es := &github.EncryptedSecret{
			Name:           name,
			KeyID:          pub.GetKeyID(),
			EncryptedValue: encrypted,
		}
		if _, err := c.inner.Actions.CreateOrUpdateRepoSecret(ctx, repo.Owner, repo.Name, es); err != nil {
			return fmt.Errorf("upsert secret %s on %s/%s: %w", name, repo.Owner, repo.Name, err)
		}
	}
	return nil
}

// SanitizeSecretName turns a 1Password field label into a valid GitHub Actions
// secret name: uppercase letters, digits, and underscores only.
func SanitizeSecretName(prefix, label string) string {
	var b strings.Builder
	if prefix != "" {
		b.WriteString(SanitizeSecretName("", prefix))
		if !strings.HasSuffix(b.String(), "_") {
			b.WriteByte('_')
		}
	}

	lastWasUnderscore := b.Len() == 0
	for _, r := range label {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToUpper(r))
			lastWasUnderscore = false
		default:
			if !lastWasUnderscore {
				b.WriteByte('_')
				lastWasUnderscore = true
			}
		}
	}

	name := strings.Trim(b.String(), "_")
	if name == "" {
		return "SECRET"
	}
	return name
}

// BuildSecretMap maps 1Password field labels to sanitized GitHub secret names.
func BuildSecretMap(fields map[string]string, prefix string) map[string]string {
	out := make(map[string]string, len(fields))
	for label, value := range fields {
		out[SanitizeSecretName(prefix, label)] = value
	}
	return out
}

func encryptSecret(publicKeyB64, secretValue string) (string, error) {
	rawKey, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		return "", fmt.Errorf("decode public key: %w", err)
	}
	if len(rawKey) != 32 {
		return "", fmt.Errorf("unexpected public key length %d", len(rawKey))
	}
	var publicKey [32]byte
	copy(publicKey[:], rawKey)

	encrypted, err := box.SealAnonymous(nil, []byte(secretValue), &publicKey, rand.Reader)
	if err != nil {
		return "", fmt.Errorf("seal secret: %w", err)
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func toSet(items []string) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		out[item] = struct{}{}
	}
	return out
}
