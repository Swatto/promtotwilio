# Contributing to promtotwilio

## Development Setup

1. Clone the repository
2. Ensure Go 1.27+ is installed
3. Run `make check` to verify everything works

## Code Style

- Run `make lint` before submitting PRs
- All tests must pass: `make test`
- E2E tests should pass: `make e2e` (requires Docker)

## Security

The Security workflow runs govulncheck on every pull request, on pushes to
`main` and daily. It scans the source and, in binary mode, the binary inside
the Docker image, because the Dockerfile pins its own Go toolchain and the
image can lag behind the Go version CI uses. The daily run scans the
published `latest` image and the latest release binary, and opens a
"Security audit failing" issue when it fails.

When it fails on a standard library advisory, bump the `golang` image in
the Dockerfiles to the fixed Go patch release and cut a new release.

Run it locally with `make vuln` (source) or `make vuln-image` (Docker
image binary).

## Pull Requests

1. Fork the repo and create your branch from `main`
2. Add tests for any new functionality
3. Ensure all checks pass
4. Submit a PR with a clear description

---

## Releasing

Nobody tags by hand. Commits follow [Conventional Commits](https://www.conventionalcommits.org/)
(`feat:`, `fix:`, `deps:`, `feat!:` ...). On every push to `main`,
release-please looks at the commits since the last release and keeps a
pull request named "chore(main): release x.y.z" up to date: it picks the
next version, prepends the generated entry to `CHANGELOG.md`, and lists
what changed. CI (unit, lint, E2E) runs on that pull request like any
other.

Merging it creates the tag and the GitHub release, and the Release
workflow then:

- builds the binaries and attaches them to the release
- builds the image, scans it with Grype (a high or critical finding with
  a fix stops the release), and pushes it for `amd64` and `arm64`

| Artifact | Examples |
|----------|----------|
| **Git Tag** | `v1.2.3` |
| **Binaries** | `promtotwilio-linux-amd64`, `promtotwilio-darwin-arm64`, etc. |
| **Docker Tags** | `1.2.3`, `1.2`, `1`, `latest` |

`feat:` bumps the minor version, `fix:`, `perf:` and `deps:` the patch,
and a `!` or a `BREAKING CHANGE:` footer the major. `chore:`, `ci:`,
`refactor:` and `test:` alone do not cut a release. To force a specific
version, add a `Release-As: 2.0.0` footer to a commit.

To rebuild the binaries and image for an existing tag, run the Release
workflow by hand from the Actions tab and give it the tag.
