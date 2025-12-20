# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

**Do not open a public issue for security vulnerabilities.**

Please report security vulnerabilities using GitHub's private security advisory feature:

1. Go to the Security tab of this repository
2. Click "Report a vulnerability"
3. Fill in the details

We will respond within 48 hours and work with you to understand and resolve the issue.

## Security Measures

- All dependencies are automatically updated via Dependabot
- Security scanning with gosec and govulncheck on every PR
- Docker images scanned with Trivy before release
- Supply chain attestation on all release artifacts
