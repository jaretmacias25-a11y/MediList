param(
    [string]$DbHost = "localhost",
    [string]$DbUser = "postgres",
    [string]$Password = $env:PGPASSWORD,
    [string]$Database = "medilist"
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

$exists = & $psql -U $DbUser -h $DbHost -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '$Database'"
if ("$exists".Trim() -ne "1") {
    Write-Host "Creando base de datos '$Database'..."
    & $psql -U $DbUser -h $DbHost -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE $Database"
    if ($LASTEXITCODE -ne 0) { throw "No se pudo crear la base de datos." }
}

Get-ChildItem (Join-Path $root "migrations\*.sql") | Sort-Object Name | ForEach-Object {
    Write-Host "Aplicando $($_.Name)..."
    & $psql -U $DbUser -h $DbHost -d $Database -v ON_ERROR_STOP=1 -f $_.FullName
    if ($LASTEXITCODE -ne 0) { throw "Falló la migración $($_.Name)." }
}

Write-Host "Migraciones aplicadas correctamente en '$Database'."
