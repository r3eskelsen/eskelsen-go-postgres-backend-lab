# GitHub publication checklist

1. Create a **Public** repository under `r3eskelsen` named `eskelsen-go-postgres-backend-lab`.
2. Do not auto-generate GitHub README, license, or `.gitignore`; these files exist in this project.
3. Upload the **contents** of `01_GITHUB_UPLOAD/` from the parent publication package to the **repository root**. Do not upload the outer ZIP or Upwork marketing folder as the application source.
4. Include dotfiles `.gitignore` and `.gitattributes`.
5. Review the public repository for secrets and confirm the project-specific license notice.
6. Confirm `go.mod` module path and import path match the exact repository URL.
7. If possible, rerun the isolated Compose verification with Go 1.25 and PostgreSQL, preserve new results, then update `docs/RESULTS.md` with the execution environment and outcomes.
8. Add the actual repository URL to Upwork only after verifying it opens successfully.

The prior original-package test logs are historical evidence, not a live CI badge.

## GitHub description

Go + PostgreSQL transactional inventory API demo: concurrent reservations, row locking, idempotency, rollback tests and Docker-based verification.

## Suggested topics

`golang` `postgresql` `backend` `rest-api` `concurrency` `transactions` `idempotency` `docker` `pgx` `testing`

## Source-code scope

Original demo source code is **all rights reserved**; do not relicense as MIT.
No application production deployment or security certification is claimed.
