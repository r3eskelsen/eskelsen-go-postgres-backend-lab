# Release history

## v1.0.1 — 2026-11-09 (GitHub/Upwork handoff)

- Changed `go.mod` module path and the API entry point import to `github.com/r3eskelsen/eskelsen-go-postgres-backend-lab`.
- Clarified status of original 2026-11-09 test logs versus independent archive integrity checks.
- Updated stale archive-verification and publication-status wording.
- Added historical G0 header, source publication checklist, and release provenance.
- Preserved the transactional implementation, migrations, tests, fixtures and original execution logs unchanged.
- Regenerated hashes for this publication package.

**Validation limit:** Go 1.25 with a live PostgreSQL service was unavailable for a fresh integrated run here. Reported passing runs are from the source project delivery, prior to the import-path change. Do not market as production-ready or independently security-certified.
