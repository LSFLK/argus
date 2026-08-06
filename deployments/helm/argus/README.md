# Argus Helm Chart

This Helm chart deploys **Argus**, the secure cryptographic Audit Logging service, on Kubernetes/OpenShift.

## Prerequisites

- Kubernetes 1.20+
- Helm 3.0+
- (Optional) External Secrets Operator (ESO) & HashiCorp Vault integration for managing secrets.

## Chart Details

This chart provisions:
- Argus service **Deployment** with health checks (liveness and readiness probes)
- ClusterIP **Service** (defaulting to port `3001`)
- Audit enums **ConfigMap** (`enums.yaml`)
- Credentials **Secret** (or **ExternalSecret** when ESO is enabled)

## Installation & Deployment

### Standalone Deployment

To deploy Argus independently:

```bash
helm upgrade --install argus ./deployments/helm/argus \
  --namespace nsw-infra-staging \
  --create-namespace \
  --values ./deployments/helm/argus/values.yaml
```

### Parent Chart (GitOps Umbrella) Integration

When deployed via `nsw-gitops` under `infra-umbrella`, toggle the Argus component in your environment values file (e.g., `envs/staging/infra-values.yaml`):

```yaml
argus:
  enabled: true
  auth:
    existingSecret: "nsw-db-credentials"
  env:
    DB_HOST: "staging-nsw-db"
    DB_NAME: "nsw_staging"
    S3_COMPLIANCE_BUCKET: "nsw-audit-compliance-logs-staging"
```

## Configuration Parameters

| Parameter | Description | Default |
| --- | --- | --- |
| `replicaCount` | Number of pod replicas | `2` |
| `image.repository` | Container image repository | `ghcr.io/opennsw/argus` |
| `image.tag` | Container image tag | `f21da85558410c19b6a96275b6e0eef2a788fb4b` |
| `service.type` | Kubernetes service type | `ClusterIP` |
| `service.port` | Service port | `3001` |
| `env.ENVIRONMENT` | Deployment environment | `production` |
| `env.DB_TYPE` | Database driver (`postgres` or `sqlite`) | `postgres` |
| `env.DB_HOST` | Database host | `nsw-db` |
| `env.DB_PORT` | Database port | `5432` |
| `env.DB_NAME` | Database name | `audit_db` |
| `env.REQUIRE_SIGNATURES` | Enable signature verification | `"true"` |
| `env.S3_COMPLIANCE_BUCKET` | S3 WORM compliance bucket name | `"nsw-audit-compliance-logs-staging"` |
| `auth.existingSecret` | Existing Kubernetes secret containing `DB_PASSWORD` | `""` |
| `auth.externalSecrets.enabled` | Enable ExternalSecrets Operator (ESO) | `false` |
