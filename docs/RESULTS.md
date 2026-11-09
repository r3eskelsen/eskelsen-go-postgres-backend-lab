# Verification Results

Date: 2026-11-09. Evidence: operator-supplied local terminal logs.
These logs were produced before the publication-only module/import path change.
Their results are preserved unchanged, but are not a fresh execution of
this GitHub handoff revision.
Environment: Go 1.25.14 linux/amd64, CGO enabled, PostgreSQL 16.13.

## Observed outcomes

| Scenario | Result |
|---|---|
| Input validation | 10 table-driven subtests passed |
| HTTP validation | 11 subtests passed |
| HTTP lifecycle | Create, read, cancel and historical replay passed |
| Concurrent stock requests | 25 successes, 75 stock conflicts, 0 unexpected errors |
| Same-key concurrency | 20 requests produced 1 reservation |
| Concurrent cancellation | 20 requests returned stock once |
| Injected failure | Rollback and same-key retry passed |
| Row-lock deadline | Returned after approximately 200 ms; retry passed |
| SQL conservation check | 0 violations |
| Incomplete idempotency records | 0 |
| Race detector | No Go races reported in executed workloads |
| Static analysis | go vet passed in initial and Compose verification |

## Recorded runs

1. Initial isolated database: complete suite passed with -race.
2. New database created from template0: migrations, fixtures, complete
   suite with -race and independent SQL verification passed.
3. Compose: automatic database initialization, independent module/build
   caches, build, complete suite with -race, vet and SQL check passed.

Reconstruction reused the local Docker installation and existing images.
It does not establish installation from a clean operating system.

## Original evidence filenames

- G2_P4_BUILD_20261109_130830_925.txt
- G2_P5_DATABASE_20261109_131043_978.txt
- G3_P1_RACE_20261109_131259_914.txt
- G3_P2_REPRO_20261109_132334_078.txt
- G4_PACKAGE_DEPS_20261109_132908_313.txt
- G4_PACKAGE_TESTS_20261109_133038_625.txt

Original logs remain in the project `evidence/` directory.
Six public log copies are available there. The original delivery ZIP and
all 29 source hashes were verified by independent offline inspection.
The updated publication archive has a separate regenerated SHA-256 manifest.
Post-rewrite Go 1.25/PostgreSQL execution remains pending.
Small local test durations are observations, not performance benchmarks.
