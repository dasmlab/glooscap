# Glooscap Operator

Kubebuilder-based controller that discovers Outline wiki content, stages it for user selection, and orchestrates vLLM-backed translation jobs. It manages:

- `WikiTarget` resources defining Outline endpoints, credentials, modes, and discovery cadence.
- `TranslationJob` resources representing ETL executions and publication targets.

Refer to `../docs/architecture.md` for the system blueprint.

## Getting Started

### Prerequisites

- Go `1.24.x`
- Docker/Podman for image builds
- Access to a Kubernetes/OpenShift `1.27+` cluster
- Kubebuilder envtest binaries (`make setup-envtest`) when running controller tests locally

### Local Development

Install CRDs and run the manager against your kubeconfig:

```sh
make install
make run
```

Regenerate code and manifests after editing API definitions:

```sh
make generate manifests
```

### Build & Deploy

Build, push, and deploy an image:

```sh
make docker-build docker-push IMG=registry.example.com/glooscap/operator:dev
make deploy IMG=registry.example.com/glooscap/operator:dev
```

Apply sample resources:

```sh
kubectl apply -k config/samples
```

### Cleanup

```sh
kubectl delete -k config/samples
make undeploy
make uninstall
```

## CRD Highlights

### `WikiTarget`

- `spec.uri`: Outline base URL (`Format: uri`)
- `spec.serviceAccountSecretRef`: Secret containing API credentials
- `spec.mode`: `ReadOnly | ReadWrite | PushOnly`
- `spec.sync`: Interval and full refresh cadence
- `spec.translationDefaults`: Default destination configuration

### `TranslationJob`

- `spec.source.targetRef`, `spec.source.pageId`, optional `revision`
- `spec.destination`: overrides for target, path prefix, language tag
- `spec.pipeline`: `InlineLLM | TektonJob` (default `TektonJob`)
- `status`: lifecycle state, timestamps, audit reference, readiness conditions

## Testing

- `make test` – runs API/controller/unit suites (requires envtest assets)
- Fetch envtest binaries via `make setup-envtest` or export `KUBEBUILDER_ASSETS` before running controller tests
- `SKIP_E2E=1 make test` – skip Docker-backed e2e for constrained environments
- `make test-e2e` – builds the manager image and exercises end-to-end flow

## Next Steps

- Implement discovery workers, memdb integration, and translation dispatch wiring.
- Add OTEL exporters and admission webhooks for security guardrails.
- Connect the forthcoming Quasar UI (`../ui/`) to the controller APIs.

Run `make help` for the full list of helper targets.

