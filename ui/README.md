# Glooscap UI
# CI Test Bump 24 - All three test
# CI Test Bump 18 - Login and build in same block

Quasar/Vue 3 single-page console for browsing discovered Outline pages, staging translations, monitoring jobs, and managing defaults. Works alongside the Kubebuilder operator (`../operator`) and surfaces telemetry-friendly interactions only—no design frills.

## Features

- Catalogue view with target selector, search, and job queue actions
- Pinia stores with mock data placeholders (API wiring via `/api/v1`)
- Job timeline visualisation and security posture banner
- Settings form for translation defaults and OTEL endpoints

## Getting Started

```bash
cd ui
npm install
npm run dev
```

The dev server proxies `/api` traffic to `http://localhost:8080` (configurable via `VITE_PROXY_TARGET`).

## Scripts

- `npm run dev` – launch Quasar dev server
- `npm run build` – build static assets
- `npm run lint` – run `vite-plugin-checker`/ESLint
- `npm run format` – Prettier across sources

## Container Image

```bash
# build image
docker build -t quay.io/dasmlab/glooscap-ui:latest .

# push to registry
docker push quay.io/dasmlab/glooscap-ui:latest
```

Apply the OpenShift manifest in `../infra/openshift/glooscap-ui.yaml`, updating the host to `web-glooscap.apps.<cluster-domain>`.
Ensure `VITE_API_BASE_URL` points at the operator API service (e.g. `http://glooscap-operator-api.glooscap-system.svc.cluster.local:3000/api/v1`).

## Configuration

- Update boot files in `src/boot/` to register additional plugins (e.g., auth).
- REST client configuration lives in `src/services/api.js`.
- Quasar/Vite settings in `quasar.config.js` (router mode, proxy, env vars).

See `../docs/ui.md` for the UX blueprint and future enhancements.*** End Patch
