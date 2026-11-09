# Portfolio Publication Guidance

## Title
Go + PostgreSQL API: Concurrency and Idempotency Demo

## Project description

This sample describes the operator-verified original execution logs;
post-publication-module-path Go 1.25/PostgreSQL rerun remains pending.

Built an independent inventory reservation API using Go, PostgreSQL
and Docker Compose to demonstrate transaction correctness under
concurrent requests.

Implemented row-level locking, idempotent creation, repeat-safe
cancellation, request validation and bounded database operations.
Verified rollback through an injected failure and tested deadline
behavior while a product row was locked.

In the recorded workload, 100 requests competed for 25 units:
25 succeeded, 75 received stock conflicts and no unexpected errors
occurred. Twenty same-key requests produced one reservation, and
twenty concurrent cancellations returned stock once.

The test suite passed three local runs with the Go race detector,
including database reconstruction and a reproducible Compose workflow.
SQL checks reported zero inventory conservation violations.

Deliverables include source code, SQL migration, synthetic fixtures,
tests, setup instructions, limitations and execution evidence.

Independent synthetic demo; not a client deployment or performance
benchmark. No production security or availability certification claimed.

## Skills
Go; PostgreSQL; REST API; Docker; Backend Development.

## Publication status
Intended GitHub URL after the owner creates/publishes the repository:
`https://github.com/r3eskelsen/eskelsen-go-postgres-backend-lab`.
This is **not yet verified as live**. Upwork entry has not been posted by this package.
Use the actual URL only after opening and confirming it.

## Suggested visual
Show the measured 25 successes / 75 stock conflicts beside an authentic
test-output screenshot. Label any decorative illustration as illustrative.
