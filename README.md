# ghsecop Helm repository

Public Helm chart repository for [ghsecop](https://github.com/heartwilltell/ghsecop).

## Install

```bash
helm repo add ghsecop https://heartwilltell.github.io/ghsecop
helm repo update
helm install ghsecop ghsecop/ghsecop \
  --namespace ghsecop-system --create-namespace \
  --set credentials.connectHost=http://onepassword-connect.default.svc.cluster.local:8080 \
  --set credentials.connectToken="$OP_CONNECT_TOKEN" \
  --set credentials.githubToken="$GITHUB_TOKEN"
```

## OCI (GHCR)

```bash
helm install ghsecop oci://ghcr.io/heartwilltell/charts/ghsecop --version 0.1.0 \
  --namespace ghsecop-system --create-namespace \
  --set credentials.connectHost=http://onepassword-connect.default.svc.cluster.local:8080 \
  --set credentials.connectToken="$OP_CONNECT_TOKEN" \
  --set credentials.githubToken="$GITHUB_TOKEN"
```

Chart packages and `index.yaml` are published here by GitHub Actions (chart-releaser).
