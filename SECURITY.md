# Security Policy

## Supported versions

DDAG is currently pre-1.0. Security fixes are applied to the latest commit on the `main` branch. Pin production deployments to a reviewed release or commit and follow repository updates.

## Reporting a vulnerability

**Do not open a public issue for a suspected vulnerability.** Use [GitHub private vulnerability reporting](https://github.com/RiprLutuk/DDAG/security/advisories/new).

Include the affected component and commit, minimal reproduction steps, security impact, suggested mitigation, and whether exploitation is known. Never include real access tokens, database credentials, private keys, customer data, or unredacted production logs.

We aim to acknowledge a complete report within 5 business days, validate severity and scope, coordinate a fix, and publish an advisory after affected users have a reasonable remediation path. Please allow coordinated disclosure before publishing details.

## Deployment responsibility

DDAG provides authentication, authorization, encrypted secret storage, bound query parameters, rate limiting, audit records, and structured errors. Operators remain responsible for TLS, network segmentation, least-privilege database roles, secret rotation, backups, dependency updates, and restricting dashboard and metrics access.
