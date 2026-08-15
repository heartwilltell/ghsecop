# ghsecop

Kubernetes operator that syncs **1Password** items (via **1Password Connect**) into **GitHub Actions repository secrets** across an organization.

It is intentionally similar to the official [1Password Kubernetes Operator](https://github.com/1Password/onepassword-operator) `OnePasswordItem` CRD, but the destination is GitHub Actions secrets instead of Kubernetes Secrets.

## How it works

1. You create a `GitHubSecretSync` custom resource that points at a 1Password vault/item (`itemPath`) and a GitHub organization.
2. The operator reads the item from your **1Password Connect** server.
3. Each field on that item (label → value) becomes a GitHub Actions secret name/value.
4. Those secrets are written to **every repository** in the organization (or a filtered subset).
5. Because Connect does not push change notifications, the operator **polls** Connect on an interval (default 10m, same model as the official operator) and re-syncs when the item version changes.

```text
┌─────────────────────┐     poll      ┌──────────────────────┐
│  GitHubSecretSync   │──────────────▶│ 1Password Connect    │
│  (Kubernetes CR)    │               │ vaults/.../items/... │
└─────────┬───────────┘               └──────────────────────┘
          │
          │ upsert Actions secrets
          ▼
┌─────────────────────┐
│ GitHub org repos    │
│ api / web / ...     │
└─────────────────────┘
```

## Example

1Password item `secure-node` in vault `Infrastructure` with fields:

| Field label | Value        |
|-------------|--------------|
| username    | `deploy`     |
| password    | `s3cret`     |
| api_token   | `ghp_...`    |

CR:

```yaml
apiVersion: ghsecop.io/v1
kind: GitHubSecretSync
metadata:
  name: secure-node
spec:
  itemPath: vaults/Infrastructure/items/secure-node
  github:
    organization: my-org
  secretNamePrefix: OP
  syncIntervalSeconds: 300
```

Result: every non-archived, non-fork repo under `my-org` gets Actions secrets:

- `OP_USERNAME`
- `OP_PASSWORD`
- `OP_API_TOKEN`

See `config/samples/ghsecop_v1_githubsecretsync.yaml` for more options (`fields`, `repositories`, `excludeRepositories`).

## Prerequisites

- A running [1Password Connect](https://developer.1password.com/docs/connect/) server and Connect API token
- A GitHub token with permission to:
  - list organization repositories
  - create/update Actions secrets on those repositories  
  (classic PAT: `repo` + `admin:org` as needed, or a fine-grained token / GitHub App with equivalent scopes)
- Kubernetes cluster access

## Install (Helm)

### From the public chart repository

```bash
helm repo add ghsecop https://heartwilltell.github.io/ghsecop
helm repo update

helm upgrade --install ghsecop ghsecop/ghsecop \
  --namespace ghsecop-system --create-namespace \
  --set credentials.connectHost=http://onepassword-connect.default.svc.cluster.local:8080 \
  --set credentials.connectToken="$OP_CONNECT_TOKEN" \
  --set credentials.githubToken="$GITHUB_TOKEN"
```

Or via OCI:

```bash
helm upgrade --install ghsecop oci://ghcr.io/heartwilltell/charts/ghsecop --version 0.1.0 \
  --namespace ghsecop-system --create-namespace \
  --set credentials.connectHost=http://onepassword-connect.default.svc.cluster.local:8080 \
  --set credentials.connectToken="$OP_CONNECT_TOKEN" \
  --set credentials.githubToken="$GITHUB_TOKEN"
```

### From this repository

```bash
helm upgrade --install ghsecop ./charts/ghsecop \
  --namespace ghsecop-system --create-namespace \
  --set credentials.connectHost=http://onepassword-connect.default.svc.cluster.local:8080 \
  --set credentials.connectToken="$OP_CONNECT_TOKEN" \
  --set credentials.githubToken="$GITHUB_TOKEN"
```

Or point at an existing secret with `--set credentials.existingSecret=ghsecop-credentials`.

Apply a sync resource (or set `syncs` in Helm values — see `charts/ghsecop/values-sync-example.yaml`):

```bash
kubectl apply -f config/samples/ghsecop_v1_githubsecretsync.yaml
kubectl get githubsecretsyncs
kubectl describe githubsecretsync secure-node
```

Chart details: [`charts/ghsecop/README.md`](charts/ghsecop/README.md).

### Install (raw manifests)

```bash
# Edit credentials in deploy/all-in-one.yaml first, or create the secret separately.
kubectl apply -f deploy/all-in-one.yaml
```

## Configuration

| Env / flag | Description |
|------------|-------------|
| `OP_CONNECT_HOST` | Connect server URL (required) |
| `OP_CONNECT_TOKEN` | Connect API token (required) |
| `GITHUB_TOKEN` | GitHub token for Actions secrets API (required) |
| `--polling-interval` | Default Connect poll interval (default `10m`) |
| `spec.syncIntervalSeconds` | Per-CR poll override (min 30) |

### CRD fields

| Field | Meaning |
|-------|---------|
| `spec.itemPath` | `vaults/<vault>/items/<item>` (id or title), same as OnePasswordItem |
| `spec.github.organization` | Target GitHub org |
| `spec.github.repositories` | Optional allowlist of repo names; empty = all org repos |
| `spec.github.excludeRepositories` | Repos to skip |
| `spec.fields` | Optional 1Password field-label allowlist; empty = all fields |
| `spec.secretNamePrefix` | Prefix for Actions secret names |

Field labels are sanitized to valid Actions secret names: uppercase `A-Z`, `0-9`, `_`.

## Development

```bash
make test
make build
make docker-build IMG=ghcr.io/you/ghsecop:dev
```

## Status

`GitHubSecretSync.status` reports:

- `phase`: `Synced` / `Error`
- `itemVersion`: last synced 1Password item version
- `lastSyncedFields`: Actions secret names written
- `syncedRepositories`: repo count updated
- `lastSyncTime`
- standard `Ready` / `Synced` conditions

## Security notes

- Prefer least-privilege GitHub credentials scoped to the target org.
- Store Connect and GitHub tokens only in Kubernetes Secrets (or an external secret manager).
- Deleting a `GitHubSecretSync` does **not** delete secrets already written to GitHub (safe default).
