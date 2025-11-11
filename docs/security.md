## Security, Compliance, and Data Residency

### Zero-Exfiltration Guardrails

1. **Network Containment:** Every pod involved in the ETL path uses dedicated network namespaces with egress set to `deny` via Kubernetes NetworkPolicies and OpenShift EgressFirewall. The only permitted destinations are the source/destination Outline endpoints and the in-cluster vLLM service.
2. **Compute Sandbox:** Controllers and worker pods execute under a restricted runtimeClass (Kata Containers or gVisor) with SELinux enforcing `container_t` + custom MCS labels. cgroup egress hooks drop outbound packets that do not match allowlisted CIDRs, providing an additional OSI-layer barrier.
3. **Data Diodes for vLLM:** The vLLM integration leverages a sidecar implementing a unidirectional gRPC interface. The worker mounts the sidecar via loopback; the sidecar forwards requests to the vLLM and refuses responses that include URIs or external references. Responses are validated against a schema that prohibits outbound call instructions.

These measures, combined, make it practically impossible for content to leave the cluster once fetched.

### Auditing & Tracing

- **OpenTelemetry Everywhere:** All controllers, services, and the UI emit traces with correlation IDs (`WikiTarget`, `TranslationJob`, request IDs). Spans annotate movement of sensitive payloads.
- **Immutable Event Log:** Translation jobs append to an append-only audit store (OpenShift Logging backed by Loki/Elasticsearch) with hash chaining for tamper evidence.
- **K8s Native Events:** Controllers surface Kubernetes events for lifecycle transitions; aggregated by cluster logging for compliance review.
- **Tekton/Cron Audits:** If Tekton is used, pipeline runs emit CDI-compliant provenance (SLSA) and Tekton Chains to record supply chain metadata.

### Data Residency & At-Rest Strategy

- **MemDB Default:** In-memory map backed by `go-memdb` with optional encrypted swap using tmpfs + LUKS. Snapshots disabled by default. Controllers expose metrics to confirm no disk persistence.
- **Secrets Handling:** Service accounts stored in Kubernetes secrets sealed via SealedSecrets or External Secrets operator; never written to logs.
- **Post-Run Sanitisation:** A background cleaner scrubs transient translation buffers immediately after publish. Tekton workspaces mount `emptyDir` with memory medium wherever possible.

### Compliance Alignment (RHEL / OCP)

- Integrate with **Red Hat Compliance Operator** to apply predefined profiles (e.g., `ocp4-moderate`, `cis`).
- Use **OpenShift Sandbox Containers** runtime for strong isolation.
- Leverage **Cluster Logging** and **Distributed Tracing** operators for OTEL ingestion and log retention policies.
- Document **Standard Operating Procedures** for change control, incident response, and approvals in `docs/runbooks/`.

