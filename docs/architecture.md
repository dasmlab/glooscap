## Architecture Overview

### High-Level Flow

1. **Discovery:** `WikiTarget` custom resources describe Outline instances, authentication, and desired behaviour (read-only, read-write, push-only). The controller spawns a discovery worker that traverses the Outline REST API and populates an in-memory catalogue of pages plus metadata (language, parents, assets).
2. **Selection:** Operators use the Quasar UI to review discovered pages, select translation operations, and designate destinations. The UI speaks to a lightweight gRPC/REST service exposed by the operator (running inside the controller manager pod).
3. **Translation Jobs:** Selected pages create `TranslationJob` custom resources that model the ETL lifecycle. Reconciliation will:
   - Fetch source content (and assets) through the wiki client.
   - Package payloads and metadata for inference.
   - Dispatch the task to the vLLM platform (Kubernetes Job via Tekton or API request, depending on deployment mode).
   - Receive translated artifacts and stage them for publication.
4. **Publication:** Depending on the `WikiTarget` mode:
   - **Read-only:** No publish; job marked completed once translation stored for user download.
   - **Read-write:** Publish as sibling page (e.g., `/fr/<slug>`).
   - **Push-only:** Push translation to external target using dedicated credentials.

### Custom Resource Definitions

#### `WikiTarget`

- `spec.uri`: Outline base URL.
- `spec.serviceAccountSecretRef`: Kubernetes secret for API credentials.
- `spec.mode`: `ReadOnly`, `ReadWrite`, `PushOnly`.
- `spec.sync.interval`: Page discovery schedule.
- `spec.translationDefaults`: Default destination wiki, namespace, language tags.
- `status.lastSync`, `status.catalogRevision`, `status.conditions`.

#### `TranslationJob`

- `spec.sourceTargetRef`: Target wiki reference.
- `spec.pageId` and `spec.revision`.
- `spec.destination`: wiki identifier + publication rules.
- `spec.pipeline`: `InlineLLM` or `TaskJob`.
- `status.state`: `Queued`, `Dispatching`, `Running`, `Publishing`, `Completed`, `Failed`.
- `status.auditTrail`: lightweight pointer to immutable event stream.

### Components

- **Controller Manager:** Hosts reconcilers for all CRDs, exposes metrics, health probes, OTEL exporter, and the UI API.
- **Wiki Client:** Go package wrapping Outline REST API (discovery, page fetch, asset fetch, publish).
- **MemDB Runtime:** In-memory index of discovered pages (hash keyed by wiki + page ID). Receives periodic checkpointing to avoid data at rest; uses in-memory only by default, with optional encrypted snapshots stored in tmpfs.
- **ETL Service:** gRPC/REST façade to the memdb and job queue. Validates user actions, includes RBAC using Kubernetes ServiceAccounts / OIDC.
- **UI (Quasar):** SPA served via controller sidecar or `ui/` static container. Auth via OAuth2/OIDC against cluster IdP.
- **Telemetry Stack:** OpenTelemetry SDK in Go and front-end instrumentation forwarding to OTEL collector (already available in-cluster).
- **Security Hooks:** Admission webhooks ensuring targets configured with secrets, network policies, and translation pipelines comply with data-handling rules.

### Key Data Structures

```startLine:endLine:operator/api/v1alpha1/wikitarget_types.go
// ... existing code ...
```

The Go structs will follow Kubebuilder scaffolding conventions with validation markers for `mode`, `uri`, and secret references.

### Runtime Interactions

- **Discovery Worker:** Schedules via controller runtime worker pools, respects per-target rate limits, pushes results into memdb, updates `WikiTarget.status`.
- **Translation Queue:** Backed by controller-managed queue (workqueue) with job deduplication and concurrency controls driven by CRD annotations.
- **Publication Handler:** Applies Outline API updates through service account tokens; supports idempotent updates by tracking last published revision.

### Deployment Topology

- Default deployment: single controller manager pod with sidecar containers:
  - `controller`: Go binary.
  - `ui`: Container serving compiled Quasar assets.
  - `otel-collector` or remote exporter sidecar.
- Optional Tekton integration: `TranslationJob` reconciliation submits `PipelineRun` objects to Tekton, enabling pipeline visualization and audit.
- Configured through Helm chart or OLM bundle for OpenShift.

