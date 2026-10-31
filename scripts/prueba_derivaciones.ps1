param(
    [string]$DbHost = "localhost",
    [string]$DbUser = "postgres",
    [string]$Password = $env:PGPASSWORD,
    [string]$Database = "medilist_derivaciones_test",
    [switch]$KeepDatabase
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot

$psql = Get-ChildItem "C:\Program Files\PostgreSQL\*\bin\psql.exe" -ErrorAction SilentlyContinue |
    Sort-Object FullName -Descending |
    Select-Object -First 1 -ExpandProperty FullName

if (-not $psql) {
    throw "No se encontró psql.exe. Instala PostgreSQL o agrega su carpeta bin al PATH."
}
if ([string]::IsNullOrWhiteSpace($Password)) {
    throw "Define la contraseña con -Password o mediante la variable de entorno PGPASSWORD."
}

$env:PGPASSWORD = $Password
$usuarioUrl = [uri]::EscapeDataString($DbUser)
$passwordUrl = [uri]::EscapeDataString($Password)

try {
    $exists = & $psql -U $DbUser -h $DbHost -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '$Database'"
    if ("$exists".Trim() -eq "1") {
        Write-Host "Eliminando base temporal anterior '$Database'..."
        & $psql -U $DbUser -h $DbHost -d postgres -v ON_ERROR_STOP=1 -c "DROP DATABASE $Database"
    }

    Write-Host "Creando base temporal '$Database'..."
    & $psql -U $DbUser -h $DbHost -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE $Database"
    if ($LASTEXITCODE -ne 0) { throw "No se pudo crear la base temporal." }

    & (Join-Path $root "scripts\migrate.ps1") -DbHost $DbHost -DbUser $DbUser -Password $Password -Database $Database
    if ($LASTEXITCODE -ne 0) { throw "Fallaron las migraciones." }

    $env:TEST_DATABASE_URL = "postgres://${usuarioUrl}:${passwordUrl}@${DbHost}:5432/${Database}?sslmode=disable"

    Push-Location $root
    try {
        go test -count=1 -run 'Test(Derivacion|Nota|Citas)' -v ./internal/repository/postgres
        if ($LASTEXITCODE -ne 0) { throw "Fallaron las pruebas de derivaciones." }
    }
    finally {
        Pop-Location
    }

    Write-Host "Pruebas de derivaciones, notas y metadata de citas completadas."
}
finally {
    if (-not $KeepDatabase) {
        Write-Host "Eliminando base temporal '$Database'..."
        & $psql -U $DbUser -h $DbHost -d postgres -v ON_ERROR_STOP=0 -c "DROP DATABASE IF EXISTS $Database"
    }
}
