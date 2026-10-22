$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

Write-Host "Iniciando MediList API en http://localhost:8080 ..."
go run ./cmd/api
