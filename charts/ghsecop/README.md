# ghsecop Helm chart

Deploys the ghsecop Kubernetes operator, which syncs 1Password Connect items into GitHub Actions repository secrets.

## Install from the public chart repository

### Helm repo (GitHub Pages)

```bash
helm repo add ghsecop https://heartwilltell.github.io/ghsecop
helm repo update

helm upgrade --install ghsecop ghsecop/ghsecop \
  --namespace ghsecop-system --create-namespace \
  --set credentials.connectHost=http://onepassword-connect.default.svc.cluster.local:8080 \
  --set credentials.connectToken="$OP_CONNECT_TOKEN" \
  --set credentials.githubToken="$GITHUB_TOKEN"
```

### OCI (GHCR)

```bash
helm upgrade --install ghsecop oci://ghcr.io/heartwilltell/charts/ghsecop --version 0.1.0 \
  --namespace ghsecop-system --create-namespace \
  --set credentials.connectHost=http://onepassword-connect.default.svc.cluster.local:8080 \
  --set credentials.connectToken="$OP_CONNECT_TOKEN" \
  --set credentials.githubToken="$GITHUB_TOKEN"
```

## Install from this repository (local)

```bash
# Option A: pass credentials on the CLI
helm upgrade --install ghsecop ./charts/ghsecop \
  --namespace ghsecop-system --create-namespace \
  --set credentials.connectHost=http://onepassword-connect.default.svc.cluster.local:8080 \
  --set credentials.connectToken="$OP_CONNECT_TOKEN" \
  --set credentials.githubToken="$GITHUB_TOKEN"

# Option B: use an existing secret
kubectl -n ghsecop-system create secret generic ghsecop-credentials \
  --from-literal=OP_CONNECT_HOST=http://onepassword-connect.default.svc.cluster.local:8080 \
  --from-literal=OP_CONNECT_TOKEN="$OP_CONNECT_TOKEN" \
  --from-literal=GITHUB_TOKEN="$GITHUB_TOKEN"

helm upgrade --install ghsecop ./charts/ghsecop \
  --namespace ghsecop-system \
  --set credentials.existingSecret=ghsecop-credentials
```

## Optional: create sync CRs with the chart

```yaml
# values-sync.yaml
syncs:
  - name: secure-node
    itemPath: vaults/Infrastructure/items/secure-node
    github:
      organization: my-org
    secretNamePrefix: OP
    syncIntervalSeconds: 300
```

```bash
helm upgrade --install ghsecop ghsecop/ghsecop \
  --namespace ghsecop-system --create-namespace \
  --set credentials.existingSecret=ghsecop-credentials \
  -f values-sync.yaml
```

## Uninstall

```bash
helm uninstall ghsecop --namespace ghsecop-system
```

CRDs installed from `crds/` are **not** removed by `helm uninstall`. Delete them manually if desired:

```bash
kubectl delete crd githubsecretsyncs.ghsecop.io
```

## Values

| Key | Description | Default |
|-----|-------------|---------|
| `image.repository` | Operator image | `ghcr.io/heartwilltell/ghsecop` |
| `image.tag` | Image tag (empty = chart `appVersion`) | `""` |
| `pollingInterval` | Connect poll interval | `10m` |
| `leaderElect` | Enable leader election | `true` |
| `credentials.existingSecret` | Use an existing credentials secret | `""` |
| `credentials.connectHost` | 1Password Connect URL | Connect svc URL |
| `credentials.connectToken` | Connect API token | `""` |
| `credentials.githubToken` | GitHub token | `""` |
| `syncs` | Optional list of `GitHubSecretSync` CRs | `[]` |
| `resources` | Pod resource requests/limits | see `values.yaml` |

See `values.yaml` for the full set of knobs.

## Publishing

Chart releases are automated by `.github/workflows/release-chart.yaml` on pushes to `main` that change `charts/**`:

1. [chart-releaser](https://github.com/helm/chart-releaser-action) creates a GitHub Release and updates the `gh-pages` Helm repo index
2. The packaged chart is pushed to `oci://ghcr.io/heartwilltell/charts/ghsecop`
