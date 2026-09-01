# Argus Helm Chart

This Helm chart deploys **Argus**, the secure cryptographic Audit Logging service, on Kubernetes/OpenShift.

## Prerequisites

- Kubernetes 1.20+
- Helm 3.8.0+ (with native OCI support)
- (Optional) External Secrets Operator (ESO) & HashiCorp Vault integration for managing secrets.

## Chart Details

This chart provisions:
- Argus service **Deployment** with health checks (liveness and readiness probes)
- ClusterIP **Service** (defaulting to port `3001`)
- Audit enums **ConfigMap** (`enums.yaml`)
- Credentials **Secret** (or **ExternalSecret** when ESO is enabled)

---

## Installation & Deployment

### 1. Install via OCI Artifact (Recommended)

Argus Helm charts are published as OCI artifacts to the GitHub Container Registry (`ghcr.io`). Chart `0.1.1` defaults to the application image `ghcr.io/lsflk/argus:latest` (the same image is also tagged with the git SHA). Do not use chart `0.1.0` — it pointed at an unpublished `ghcr.io/opennsw/argus` image.

```bash
# Install directly from OCI registry
helm upgrade --install argus oci://ghcr.io/lsflk/charts/argus \
  --version 0.1.1 \
  --namespace <your-namespace> \
  --create-namespace \
  --values ./custom-values.yaml
```

To pull the packaged chart locally:

```bash
helm pull oci://ghcr.io/lsflk/charts/argus --version 0.1.1
```

### 2. Standalone Deployment from Source

To deploy Argus from the local repository directory:

```bash
helm upgrade --install argus ./deployments/helm/argus \
  --namespace <your-namespace> \
  --create-namespace \
  --values ./deployments/helm/argus/values.yaml
```

### 3. Parent Chart (GitOps Umbrella) Integration

When referencing Argus as a dependency in your umbrella chart (`Chart.yaml`):

```yaml
dependencies:
  - name: argus
    version: "0.1.1"
    repository: "oci://ghcr.io/lsflk/charts"
```

In your environment values file (e.g., `envs/staging/infra-values.yaml`):

```yaml
argus:
  enabled: true
  auth:
    existingSecret: "argus-db-credentials"
  env:
    DB_HOST: "staging-db"
    DB_NAME: "argus_staging"
    S3_COMPLIANCE_BUCKET: "audit-compliance-logs-staging"
```

---

## Publishing to OCI Registry

### Automated (CI/CD)

The Helm chart automation follows a standard GitOps setup:
- **Application image (`.github/workflows/build-image.yml`)**: Builds and pushes `ghcr.io/lsflk/argus` (`:<git sha>` and `:latest` on `main`). After a successful image push, also publishes the stable chart version from `Chart.yaml` (currently `0.1.1`) to `oci://ghcr.io/lsflk/charts`.
- **Dev Chart (`.github/workflows/build-dev-chart.yml`)**: On pushes to `main` with chart changes (or manual dispatch), packages and publishes a dev chart (`0.0.0-dev.<run_number>`) to `oci://ghcr.io/lsflk/charts`.
- **Chart CI (`.github/workflows/helm-ci.yml`)**: Lints the chart and verifies template rendering on pull requests.

### Manual Packaging and Push

To manually package and push to OCI registry:

```bash
# 1. Package the chart
helm package deployments/helm/argus -d .cr-release-packages/

# 2. Login to GHCR (requires PAT with write:packages)
echo "$CR_PAT" | helm registry login ghcr.io -u <username> --password-stdin

# 3. Push OCI artifact
helm push .cr-release-packages/argus-0.1.1.tgz oci://ghcr.io/lsflk/charts
```

---

## Configuration Parameters

| Parameter | Description | Default |
| --- | --- | --- |
| `replicaCount` | Number of pod replicas | `2` |
| `image.repository` | Container image repository | `ghcr.io/lsflk/argus` |
| `image.tag` | Container image tag (`:<git sha>` also published) | `latest` |
| `service.type` | Kubernetes service type | `ClusterIP` |
| `service.port` | Service port | `3001` |
| `env.ENVIRONMENT` | Deployment environment | `production` |
| `env.DB_TYPE` | Database driver (`postgres` or `sqlite`) | `postgres` |
| `env.DB_HOST` | Database host | `audit-db` |
| `env.DB_PORT` | Database port | `5432` |
| `env.DB_NAME` | Database name | `audit_db` |
| `env.REQUIRE_SIGNATURES` | Enable signature verification | `"true"` |
| `env.S3_COMPLIANCE_BUCKET` | S3 WORM compliance bucket name | `"audit-compliance-logs-staging"` |
| `auth.existingSecret` | Existing Kubernetes secret containing `DB_PASSWORD` | `""` |
| `auth.externalSecrets.enabled` | Enable ExternalSecrets Operator (ESO) | `false` |
