# Security policy

## Supported versions

Only the latest release gets security fixes: the newest
[GitHub release](https://github.com/swatto/promtotwilio/releases) and the
`ghcr.io/swatto/promtotwilio:latest` image (also tagged with its version,
for example `1`, `1.10` and `1.10.5`). Fixes ship as a new patch release;
older versions are not patched, so upgrade to get them.

## Reporting a vulnerability

Please report vulnerabilities privately through GitHub:
[Report a vulnerability](https://github.com/swatto/promtotwilio/security/advisories/new)
(Security tab, "Report a vulnerability"). Do not open a public issue or
pull request for it.

Include what you can of:

- the version (release, image tag or commit) you tested
- how to reproduce it, and what an attacker gains (for example sending
  SMS through someone else's Twilio account, or reading the Twilio
  credentials)
- any configuration it depends on, such as `WEBHOOK_SECRET` being unset

This is a project maintained in spare time, so there is no fixed response
time, but reports are read and acknowledged as soon as possible. Once a fix
is released, the advisory is published with credit to the reporter, unless
you would rather stay anonymous.

## Scope

In scope: the promtotwilio code in this repository, the released binaries
and the published image.

Out of scope: vulnerabilities in Twilio, Prometheus or Alertmanager
themselves, and setups that expose `POST /send` to untrusted networks
without setting `WEBHOOK_SECRET`.

## What is already checked

Every pull request and push to `main`, and a daily scheduled run, check
the code with govulncheck. The daily run also checks the published image
and release binary. Every release image is scanned with Grype before it is
pushed. Dependabot keeps the Go modules, tools, GitHub Actions and Docker
images up to date.
