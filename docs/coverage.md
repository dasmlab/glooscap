# Code Coverage

This document describes the code coverage setup and reporting for the Glooscap project.

## Overview

We track code coverage for the operator (Go) to ensure quality and identify untested code paths. Coverage reports are generated automatically in CI/CD and can be run locally.

## Local Coverage Reports

### Quick Start

```bash
cd operator
make test-coverage
```

This will:
1. Run all unit tests (excluding e2e)
2. Generate `coverage.out` (raw coverage data)
3. Generate `coverage.html` (interactive HTML report)
4. Generate `coverage.txt` (function-level breakdown)

### Using the Coverage Script

```bash
cd operator
./scripts/coverage.sh
```

This script:
- Automatically sets up envtest if needed
- Generates all coverage reports
- Opens the HTML report in your browser (if available)

### Viewing Reports

- **HTML Report**: Open `coverage.html` in your browser for an interactive view
- **Text Report**: View `coverage.txt` for a function-by-function breakdown
- **Summary**: The last line of `coverage.txt` shows overall coverage percentage

## CI/CD Coverage

### GitHub Actions

Coverage is automatically generated on:
- Push to `main` or `develop` branches
- Pull requests targeting `main` or `develop`
- Manual workflow dispatch

### Coverage Artifacts

Each CI run produces:
- `coverage.out` - Raw coverage profile
- `coverage.html` - Interactive HTML report
- `coverage.txt` - Function-level breakdown

Download these from the GitHub Actions workflow artifacts.

### Codecov Integration

If configured with a `CODECOV_TOKEN` secret, coverage is automatically uploaded to [Codecov](https://codecov.io).

**Setup Codecov:**
1. Sign up at https://codecov.io
2. Add your repository
3. Copy the repository token
4. Add it as `CODECOV_TOKEN` in GitHub repository secrets

### Coverage Badge

Add this to your README to show current coverage:

```markdown
[![codecov](https://codecov.io/gh/dasmlab/glooscap/branch/main/graph/badge.svg)](https://codecov.io/gh/dasmlab/glooscap)
```

## Coverage Goals

### Current Targets

- **Minimum**: 60% overall coverage
- **Target**: 80% overall coverage
- **Critical Paths**: 90%+ coverage (controllers, API handlers)

### What We Track

- **Included**: All unit tests, controller tests, API tests
- **Excluded**: E2E tests (run separately), generated code, vendor code

### Improving Coverage

1. **Identify gaps**: Review `coverage.html` to find untested code
2. **Add tests**: Focus on critical paths first (controllers, API handlers)
3. **Test edge cases**: Error paths, boundary conditions, validation logic
4. **Mock dependencies**: Use Ginkgo/Gomega for isolated unit tests

## Coverage by Package

View detailed package-level coverage in `coverage.txt`:

```bash
grep "^github.com/dasmlab/glooscap-operator" coverage.txt
```

## Troubleshooting

### envtest Not Found

```bash
make setup-envtest
export KUBEBUILDER_ASSETS=$(make -s setup-envtest | grep KUBEBUILDER_ASSETS | cut -d'=' -f2 | tr -d '"')
```

### Coverage Report Empty

Ensure tests are actually running:
```bash
go test -v ./...
```

### HTML Report Won't Open

Manually open `coverage.html` in your browser, or use:
```bash
# macOS
open coverage.html

# Linux
xdg-open coverage.html

# Windows
start coverage.html
```

## Best Practices

1. **Run coverage before PR**: `make test-coverage` locally
2. **Aim for incremental improvement**: Don't let coverage decrease
3. **Focus on critical paths**: Controllers and API handlers first
4. **Test error cases**: Many bugs hide in error paths
5. **Keep tests fast**: Use mocks and avoid slow I/O in unit tests

## References

- [Go Coverage Documentation](https://go.dev/blog/cover)
- [Codecov Documentation](https://docs.codecov.com)
- [Ginkgo Testing Framework](https://onsi.github.io/ginkgo/)

