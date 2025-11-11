## vLLM Integration Strategy

### Deployment Targets

- **OpenShift AI / RHOAI:** Preferred managed service for hosting vLLM with GPU-backed nodes.
- **Custom OCP Namespace (`nanabush`):** Dedicated project with GPU machine sets, ConfigMaps, and Secrets for model weights.

### Interaction Models

1. **Job-Oriented (Default)**
   - `TranslationJob` controller submits Tekton `PipelineRun` or Kubernetes `Job` to OCP AI namespace.
   - Job image contains lightweight client that pulls model weights from persistent volume, invokes vLLM inference, and writes results to a Secure Service Bus (e.g., Kafka inside cluster) monitored by the operator.
   - Pros: strong isolation, easy audit; Cons: higher latency per request.
2. **Service-Oriented (Optional)**
   - Always-on vLLM deployment exposes gRPC/REST endpoint inside cluster.
   - Operator uses service mesh (Istio/Service Mesh operator) for mTLS, rate limiting, and zero-trust policies.
   - Requires hardened egress guard rails (see security doc).

### Telemetry & Feedback Loop

- Every inference request carries trace context (`traceparent`) propagated into vLLM service logs.
- Post-run sanitizer strips PII/artifacts and pushes sanitized prompt/response pairs into a retraining queue (Kafka topic `vllm-training`).
- Scheduled retraining pipeline (Tekton) consumes sanitized data to fine-tune the model; results stored in OCI registry as new model versions.

### Configuration Artifacts

- `infra/nanabush/tekton/translation-pipeline.yaml`: Defines pipeline tasks (fetch content, run inference, publish result, sanitize output).
- `infra/nanabush/kustomization.yaml`: Assembles namespace-scoped resources (service accounts, network policies, configmaps).
- `infra/nanabush/helm/vllm/`: Helm chart for service-oriented deployment.

### Open Questions

- Which model variants (size, specialization) are available on the OCP cluster?
- Will RHOAI provide built-in audit logging, or do we inject OTEL sidecars?
- Preferred retraining cadence and approval workflow?

