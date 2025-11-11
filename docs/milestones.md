## Milestone Plan

### M0 – Foundations (Week 0-1)

- Finalise requirements, security controls, and compliance expectations.
- Install Kubebuilder, Quasar CLI, and supporting toolchain images in CI.
- Bootstrap `operator/` project with Go modules, `WikiTarget` CRD skeleton, Makefile, helper scripts (`buildme.sh`, `cycleme.sh`).
- Draft OpenShift operator bundle structure (CSV skeleton).

### M1 – Discovery & UI Skeleton (Week 2-3)

- Implement `WikiTarget` reconciler for page discovery and memdb population.
- Build Quasar UI scaffold with authentication stub and page catalogue table fed by mocked API.
- Wire OTEL exporter and structured logging.
- Define Tekton pipeline templates (no execution yet) in `infra/`.

### M2 – Translation Pipeline Integration (Week 4-6)

- Implement `TranslationJob` CRD and controller logic for job lifecycle.
- Integrate Outline client for page fetch/publish; support read-only and read-write modes.
- Add vLLM dispatch abstraction with pluggable backends (`InlineLLM`, `TektonJob`).
- Harden memdb lifecycle to guarantee in-memory operations with optional encrypted swap.
- Extend UI for job submission, status display, and simple filters.

### M3 – Security Hardening & Auditing (Week 7-8)

- Enforce Kubernetes NetworkPolicies, Seccomp, SELinux contexts, and sandboxed runtimeClass.
- Implement admission webhooks validating CRD inputs and enforcing zero-exfil guardrails.
- Integrate OTEL spans and traces with structured audit logs persisted via cluster logging stack.
- Add compliance automation hooks (RHEL security profiles, compliance operator integration).

### M4 – Packaging & Pilot (Week 9-10)

- Finalise Helm/OLM bundles, Tekton pipeline definitions, and operator hub metadata.
- Add end-to-end tests (envtest, Cypress for UI).
- Run pilot against `https://wiki.infra.dasmlab.org` with controlled dataset.
- Document rollout, troubleshooting, and handover guides.

