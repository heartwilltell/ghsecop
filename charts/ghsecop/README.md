# ghsecop Helm chart

Deploys the ghsecop Kubernetes operator, which syncs 1Password Connect items into GitHub Actions repository secrets.

## Install

```bash
# Create namespace
kubectl create namespace ghsecop-system

# Option A: pass credentials on the CLI
helm upgrade --install ghsecop ./charts/ghsecop \
  --namespace ghsecop-system \
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
helm upgrade --install ghsecop ./charts/ghsecop \
  --namespace ghsecop-system \
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
