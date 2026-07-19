$ErrorActionPreference = "Stop"
$workspace = Split-Path -Parent $PSScriptRoot
$backend = Join-Path $workspace "backend"
$nodeBin = "C:\Users\Yasen\.cache\codex-runtimes\codex-primary-runtime\dependencies\node\bin"
$pnpm = "C:\Users\Yasen\.cache\codex-runtimes\codex-primary-runtime\dependencies\bin\fallback\pnpm.cmd"

Push-Location $backend
try {
    go vet ./...
    go test ./...
    go build -buildvcs=false ./apps/api ./apps/worker
}
finally {
    Pop-Location
}

if (Test-Path $nodeBin) { $env:Path = "$nodeBin;$env:Path" }
if (-not (Test-Path $pnpm)) { $pnpm = "pnpm" }
& $pnpm --dir (Join-Path $workspace "web") lint
& $pnpm --dir (Join-Path $workspace "web") typecheck
& $pnpm --dir (Join-Path $workspace "web") test
& $pnpm --dir (Join-Path $workspace "web") build
