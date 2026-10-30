param([switch]$Integration)

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot

Push-Location $projectRoot

try {
    foreach ($module in @('ledger', 'gateway')) {
        Push-Location $module

        try {
            go test -count=1 -cover ./...
            if ($LASTEXITCODE -ne 0) {
                throw "Tests failed: $module"
            }

            go vet ./...
            if ($LASTEXITCODE -ne 0) {
                throw "go vet failed: $module"
            }

            go build ./...
            if ($LASTEXITCODE -ne 0) {
                throw "Build failed: $module"
            }
        }
        finally {
            Pop-Location
        }
    }

    if ($Integration) {
        if (-not $env:TEST_DATABASE_URL) {
            throw 'Set TEST_DATABASE_URL first; see ledger/README.md.'
        }

        Push-Location ledger

        try {
            go test -tags=integration -run TestPostgresRedisIntegration -count=1 -v ./...

            if ($LASTEXITCODE -ne 0) {
                throw 'PostgreSQL + Redis integration test failed'
            }
        }
        finally {
            Pop-Location
        }
    }

    Write-Host '[OK] All selected checks passed.' -ForegroundColor Green
}
finally {
    Pop-Location
}