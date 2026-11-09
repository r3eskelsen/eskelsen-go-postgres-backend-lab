# Limitations

- Independent synthetic backend demo; no production deployment.
- No authentication, tenant authorization, TLS termination or rate limiting.
- No payments, financial operations or compliance certification.
- No backup, failover or durable-storage verification.
- PostgreSQL data is deliberately ephemeral.
- Only single-product reservations; no replenishment or expiration.
- Successful creation responses are stored for replay; failed attempts
  roll back and may be retried.
- Replay after cancellation returns the original creation response.
  GET retrieves current reservation state.
- No idempotency retention or cleanup policy is implemented.
- Cross-table stock conservation depends on application transactions.
  Arbitrary manual SQL can violate that invariant.
- Application logic must not commit incomplete idempotency records;
  the schema permits them during a transaction.
- Race detection covers executed Go workloads and schedules.
- Server process shutdown and every network failure are not tested.
- JSON decoding rejects unknown fields but permits duplicate field names.
- The demo uses a database-owner role; separate least-privilege runtime
  and migration roles are not implemented.
- PostgreSQL Compose image uses a mutable major-version tag.
  The tested local image identity is recorded in execution evidence.
- No independent security audit or completed vulnerability review.
- Root license texts for seven go.mod dependencies are retained in THIRD_PARTY_NOTICES.md. Original project code has all rights reserved.
- No commercial production experience or availability guarantee is claimed.
