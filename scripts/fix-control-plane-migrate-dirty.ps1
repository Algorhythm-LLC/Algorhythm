# Dev-only: clear golang-migrate "dirty" flag after a failed migration (e.g. BOM in SQL).
# Resets recorded version to 2 so migration 000003 runs again (ALTER ... IF NOT EXISTS is safe).
#
# Usage: .\scripts\fix-control-plane-migrate-dirty.ps1
#
# Requires: container algorhythm-postgres (ops/full-stack).

$ErrorActionPreference = "Stop"

docker exec algorhythm-postgres psql -U algorhythm -d control_plane -c "SELECT version, dirty FROM schema_migrations;"
docker exec algorhythm-postgres psql -U algorhythm -d control_plane -c "UPDATE schema_migrations SET version = 2, dirty = false WHERE dirty = true;"
Write-Host "Done. Restart control-plane API (e.g. start-desktop-stack.ps1)." -ForegroundColor Green
