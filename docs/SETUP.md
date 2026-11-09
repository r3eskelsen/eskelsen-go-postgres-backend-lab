# Local Setup and Verification

Requirements: Docker Desktop with Linux containers, Docker Compose,
PowerShell, internet for initial downloads and sufficient free memory.
Use a local Docker context. No host Go installation is required.

Run from the repository directory containing compose.yaml.
Ensure esk-lab08-package is not used by unrelated Docker resources.
Keep all commands in the same PowerShell window.

## 1. Generate a disposable password

```powershell
$bytes = New-Object byte[] 32
$rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
try { $rng.GetBytes($bytes) } finally { $rng.Dispose() }
$env:LAB08_DB_PASSWORD = -join ($bytes | ForEach-Object { $_.ToString('x2') })
```

## 2. Download dependencies

```powershell
docker compose run --rm --no-deps deps
if ($LASTEXITCODE -ne 0) { throw 'Dependency download failed' }
```

## 3. Initialize the database and run verification

```powershell
docker compose up -d --wait --wait-timeout 90 db
if ($LASTEXITCODE -ne 0) { throw 'Database startup failed' }
docker compose run --rm test
if ($LASTEXITCODE -ne 0) { throw 'Verification failed' }
```

The test service verifies modules, builds the code, executes
go test -race and runs go vet. It uses GOPROXY=off.

## 4. Independently verify SQL invariants

```powershell
Get-Content -Raw .\scripts\verify_invariants.sql | docker compose exec -T db psql -X -v ON_ERROR_STOP=1 -U lab08 -d lab08
if ($LASTEXITCODE -ne 0) { throw 'SQL verification failed' }
```

Expected: zero violating rows and successful completion of the DO block.

## Data and resource boundaries

PostgreSQL publishes no host ports and uses an internal network.
Its data is ephemeral: stopping or removing the container can discard it.
Schema and synthetic fixtures initialize automatically on an empty database.
Do not manually reapply the schema to an initialized database.
Tests add unique synthetic records without truncating tables.

The toolchain container is limited to 2 CPUs and 1536 MiB RAM.
PostgreSQL is limited to 1 CPU and 512 MiB RAM.
Docker and the operating system require additional memory.
Caches are stored in ../work relative to this repository.

The Compose workflow tests HTTP using an integration test server.
It does not publish a persistent API service.

## Teardown

Save evidence and verify project ownership before removing resources.

```powershell
docker compose down
if ($LASTEXITCODE -ne 0) { throw 'Teardown failed' }
Remove-Item Env:LAB08_DB_PASSWORD -ErrorAction SilentlyContinue
```

This removes only this Compose project containers and networks.
Caches remain. Never use global Docker prune for this demo.
For a new database lifecycle, generate a new disposable password.
