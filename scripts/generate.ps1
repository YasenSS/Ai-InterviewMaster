param(
    [switch]$SkipSqlc
)

$ErrorActionPreference = "Stop"
$workspace = Split-Path -Parent $PSScriptRoot
$backend = Join-Path $workspace "backend"

Push-Location $backend
try {
    go run github.com/zeromicro/go-zero/tools/goctl@v1.10.1 api validate --api api/interviewmaster.api
    go run github.com/zeromicro/go-zero/tools/goctl@v1.10.1 api go --api api/interviewmaster.api --dir apps/api
    go run github.com/zeromicro/go-zero/tools/goctl@v1.10.1 api swagger --api api/interviewmaster.api --dir api/openapi --yaml
    go run github.com/zeromicro/go-zero/tools/goctl@v1.10.1 api ts --api api/interviewmaster.api --dir ../web/src/shared/api/generated
    Copy-Item ../web/src/shared/api/gocliRequest.template.ts ../web/src/shared/api/generated/gocliRequest.ts -Force

    if (-not $SkipSqlc) {
        go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
    }
}
finally {
    Pop-Location
}
