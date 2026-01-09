# Contributing to promtotwilio

## Development Setup

1. Clone the repository
2. Ensure Go 1.23+ is installed
3. Run `make check` to verify everything works

## Code Style

- Run `make lint` before submitting PRs
- All tests must pass: `make test`
- E2E tests should pass: `make e2e` (requires Docker)

## Pull Requests

1. Fork the repo and create your branch from `main`
2. Add tests for any new functionality
3. Ensure all checks pass
4. Submit a PR with a clear description

---

## Releasing

Releases are fully automated via GitHub Actions. **Do not create tags or releases manually.**

### How to Create a Release

1. Go to **Actions** → **CI** workflow
2. Click **Run workflow**
3. Enter version in format `v1.2.3`
4. Click **Run workflow**

The workflow will:
- ✅ Run all tests (unit, lint, E2E)
- ✅ Validate version format
- ✅ Create and push git tag
- ✅ Build binaries for all platforms
- ✅ Create GitHub Release with binaries
- ✅ Push Docker images with proper tags

### What Gets Created

| Artifact | Examples |
|----------|----------|
| **Git Tag** | `v1.2.3` |
| **Binaries** | `promtotwilio-linux-amd64`, `promtotwilio-darwin-arm64`, etc. |
| **Docker Tags** | `1.2.3`, `1.2`, `1`, `latest` |

### Why Automation?

The automated process prevents common mistakes:

- **Version validation** — Rejects malformed versions like `v1.0` or `1.0.0`
- **Pre-release testing** — Broken code cannot be released
- **Consistent builds** — Multi-platform binaries built the same way every time
- **Proper tagging** — Docker images get all semantic version tags automatically

### Troubleshooting Releases

If a release fails:

1. Check workflow logs in GitHub Actions
2. Fix the issue (failing tests, lint errors, etc.)
3. Re-run the workflow with the same version

The workflow handles existing tags gracefully.
