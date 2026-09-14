# provider-hsdp

A native (non-upjet) [Crossplane](https://crossplane.io/) 2.0 provider for the Philips DIP/HSDP platform, enabling declarative management of IAM, MDM, and Provisioning resources. API groups and kinds are compatible with the legacy upjet-based [provider-hsdp](https://github.com/philips-software/provider-hsdp).

## Overview

This provider uses the [go-dip-api](https://github.com/philips-software/go-dip-api) library directly to interact with DIP services, providing full lifecycle management of resources through Kubernetes custom resources.

## Supported Resources

### IAM (Identity and Access Management)

| Resource | API Group | Kind |
|----------|-----------|------|
| Organization | iam.hsdp.m.crossplane.io | Organization |
| Proposition | iam.hsdp.m.crossplane.io | Proposition |
| Application | iam.hsdp.m.crossplane.io | Application |
| Group | iam.hsdp.m.crossplane.io | Group |
| Role | iam.hsdp.m.crossplane.io | Role |
| Service | iam.hsdp.m.crossplane.io | Service |
| Client | iam.hsdp.m.crossplane.io | Client |
| User | iam.hsdp.m.crossplane.io | User |
| EmailTemplate | iam.hsdp.m.crossplane.io | EmailTemplate |
| PasswordPolicy | iam.hsdp.m.crossplane.io | PasswordPolicy |

### MDM (Master Data Management)

| Resource | API Group | Kind |
|----------|-----------|------|
| Proposition | mdm.hsdp.m.crossplane.io | Proposition |
| Application | mdm.hsdp.m.crossplane.io | Application |
| StandardService | mdm.hsdp.m.crossplane.io | StandardService |
| DeviceGroup | mdm.hsdp.m.crossplane.io | DeviceGroup |
| DeviceType | mdm.hsdp.m.crossplane.io | DeviceType |
| AuthenticationMethod | mdm.hsdp.m.crossplane.io | AuthenticationMethod |

### Provisioning

| Resource | API Group | Kind |
|----------|-----------|------|
| OrgConfiguration | provisioning.hsdp.m.crossplane.io | OrgConfiguration |

## Installation

Install the provider using crossplane CLI or a DeploymentRuntimeConfig:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-hsdp
spec:
  package: ghcr.io/loafoe/provider-hsdp:v0.3.0
```

## Configuration

### ProviderConfig

Most resources should reference a cluster-scoped `ClusterProviderConfig` (the
default per the Crossplane v2 convention):

```yaml
apiVersion: hsdp.crossplane.io/v1
kind: ClusterProviderConfig
metadata:
  name: default
spec:
  region: us-east
  environment: client-test
  credentials:
    source: Secret
    secretRef:
      name: dip-credentials
      namespace: crossplane-system
      key: credentials
```

A namespace-scoped `ProviderConfig` is also available to override credentials
for a specific tenant/namespace; reference it explicitly via
`providerConfigRef: {kind: ProviderConfig, name: <name>}`:

```yaml
apiVersion: hsdp.m.crossplane.io/v1
kind: ProviderConfig
metadata:
  name: default
  namespace: crossplane-system
spec:
  region: us-east
  environment: client-test
  credentials:
    source: Secret
    secretRef:
      name: dip-credentials
      namespace: crossplane-system
      key: credentials
```

### Credentials Secret

The credentials secret should contain a JSON object with your service identity:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: dip-credentials
  namespace: crossplane-system
type: Opaque
stringData:
  credentials: |
    {
      "service_id": "your-service-id",
      "service_private_key": "-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----"
    }
```

The secret can optionally override `region` and `environment` from the ProviderConfig spec.

## Example Resources

### Organization

```yaml
apiVersion: iam.hsdp.m.crossplane.io/v1
kind: Organization
metadata:
  name: my-org
  namespace: crossplane-system
spec:
  forProvider:
    name: my-organization
    description: My organization
    parentOrganizationId: <parent-org-uuid>
  providerConfigRef:
    name: default
```

### Application

```yaml
apiVersion: iam.hsdp.m.crossplane.io/v1
kind: Application
metadata:
  name: my-app
  namespace: crossplane-system
spec:
  forProvider:
    name: my-application
    description: My application
    propositionId: <proposition-uuid>
    globalReferenceId: my-app-ref
  providerConfigRef:
    name: default
```

### Client

```yaml
apiVersion: iam.hsdp.m.crossplane.io/v1
kind: Client
metadata:
  name: my-client
  namespace: crossplane-system
spec:
  forProvider:
    name: my-client
    description: My OAuth2 client
    applicationId: <application-uuid>
    clientId: my-client-id
    type: Public
    globalReferenceId: my-client-ref
    redirectionURIs:
      - https://example.com/callback
    responseTypes:
      - code
    passwordSecretRef:
      name: client-password
      namespace: crossplane-system
      key: password
  providerConfigRef:
    name: default
```

## Development

### Prerequisites

- Go 1.23+
- Docker
- kubectl
- A Kubernetes cluster with Crossplane installed

### Build

```shell
make build
```

### Run locally

```shell
make run
```

### Build and push image

```shell
make docker-build docker-push IMG=ghcr.io/loafoe/provider-hsdp:v0.3.0
```

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.
