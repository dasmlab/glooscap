# Glooscap Translation Runner
# CI Test Bump 26 - All three final test

A Kubernetes Job container that processes TranslationJob CRs by:
1. Reading the TranslationJob CR from Kubernetes
2. Fetching source page content from the wiki
3. Calling the translation service (iskoces/nanabush)
4. Publishing translated pages to the destination wiki

## Building

```bash
./build.sh [tag]
```

Default tag is `latest`. Image will be built as:
`ghcr.io/dasmlab/glooscap-translation-runner:latest`

## Usage

The runner is invoked by the operator's dispatcher with:
```
--translation-job namespace/name
```

The runner will:
- Read the TranslationJob CR
- Get WikiTarget secrets for source and destination
- Fetch page content
- Call translation service
- Publish translated page

## Diagnostic Jobs

For diagnostic jobs (marked with `glooscap.dasmlab.org/diagnostic: "true"`):
- Pages are created at the top level of the wiki
- Title prefix: `AUTODIAG--> <source-title>`
- Pages can overwrite each other (OK for diagnostics)
- UUID or timestamp can be added at bottom for tracking

