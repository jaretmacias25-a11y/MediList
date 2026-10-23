param(
    [ValidateSet("up", "down", "restart", "status", "logs")]
    [string]$Action = "up"
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$compose = Join-Path $root "deploy\observability\docker-compose.yml"
$logs = Join-Path $root "logs"

New-Item -ItemType Directory -Force $logs | Out-Null

switch ($Action) {
    "up" {
        docker compose -f $compose up -d
        docker compose -f $compose ps
    }
    "down" {
        docker compose -f $compose down
    }
    "restart" {
        docker compose -f $compose restart
        docker compose -f $compose ps
    }
    "status" {
        docker compose -f $compose ps
    }
    "logs" {
        docker compose -f $compose logs -f --tail=100
    }
}
