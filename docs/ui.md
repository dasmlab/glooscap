## UI Plan (Quasar/Vue)

### Goals

- Provide operators with a fast overview of discovered pages per wiki target.
- Enable selection, batching, and submission of translation jobs.
- Show real-time job status, audit trail snippets, and error handling.
- Offer configuration panels for target defaults without exposing sensitive creds.

### Structure

1. **Authentication Shell**
   - OIDC integration via Keycloak/openshift IdP.
   - Token exchanged for short-lived session hitting controller API.
2. **Page Catalogue View**
   - Quasar `QTable` with pagination, filters (status, language, updated date).
   - Inline actions: preview, select, bulk select, schedule translation.
   - Source from `/api/v1/catalogue/{wikitarget}`.
3. **Job Queue View**
   - Visual timeline of `TranslationJob` resources.
   - Status chips reflecting `status.state`.
   - Button to cancel/retry (respecting RBAC).
4. **Settings Drawer**
   - Default destination wiki, translation mode, language tags.
   - Read-only for users without `config` role.
5. **Audit Console**
   - Stream OTEL spans & log summaries via WebSocket.
   - Provide download links for job-level audit bundles.

### Tech Stack

- Quasar CLI with TypeScript, Pinia for state management, Axios for API calls.
- Component library: Quasar core + minimal custom CSS (align with `dasmlab.org` palette).
- Unit tests via Vitest; e2e via Cypress (headless in CI).
- Build pipeline generates static assets served via Nginx sidecar or Quasar SSR depending on preference.

### API Contract Highlights

- `GET /api/v1/targets`: List configured `WikiTarget` CR summaries.
- `GET /api/v1/catalogue/{target}`: Cursor-paginated list of pages with metadata.
- `POST /api/v1/jobs`: Queue translation (payload: target, page IDs, destination options).
- `GET /api/v1/jobs/{jobId}`: Detailed status and audit info.
- `WS /api/v1/telemetry`: Stream of trace events scoped to user session.

### UX Notes

- Keep interactions stateless; rely on in-memory store for short-lived selections.
- Apply optimistic UI updates with fallback to server state.
- Provide prominent indicators of security posture (e.g., “Data contained on-cluster” badge).

