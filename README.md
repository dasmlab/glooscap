## Glooscap Wiki Translation Operator

This repository tracks the design and scaffolding plan for the Wiki translation operator that synchronises Outline wikis, translates English content to French through an on-cluster vLLM, and exposes a Quasar/Vue front-end for operator-driven ETL workflows.

- **Cluster Runtime:** Kubernetes/OpenShift, scaffolding with Kubebuilder.
- **Languages:** Go (operator + services), TypeScript (Quasar/Vue UI), Bash helpers.
- **Primary Sources:** Outline wiki instances (REST APIs); authentication by service account per target.
- **Primary Sinks:** Outline wikis (side-by-side publication or push-only targets).

### Deliverables

1. Kubebuilder-based operator skeleton with CRDs, controllers, and supporting services.
2. In-cluster ephemeral cache for discovered wiki pages (memdb backed by badger/boltdb or pure in-memory).
3. Quasar/Vue single-page UI for page discovery, translation queueing, and job tracking.
4. Secure task dispatch to the OCP-hosted vLLM along with comprehensive OTEL tracing.
5. Hardened execution and audit trail that guarantees zero outbound data exfiltration.
6. Documentation and helper scripts (`buildme.sh`, `cycleme.sh`, etc.) aligned with org standards.

### Repository Layout (planned)

- `operator/` – Kubebuilder project (`main.go`, `api/`, `controllers/`, `config/`).
- `ui/` – Quasar CLI project for the ETL table interface.
- `pkg/` – Shared go modules (wiki client, memdb abstraction, security hooks, OTEL helpers).
- `infra/` – Helm/OLM packaging, Tekton pipelines, and OCP operator integration.
- `docs/` – Architecture, milestones, security, and runbooks.
- `scripts/` – Helper shell scripts (`buildme.sh`, `cycleme.sh`, `lintme.sh`, etc.).

Refer to the documents under `docs/` for detailed plans.

