param(
    [string]$BaseUrl = "http://localhost:8080",
    [Parameter(Mandatory = $true)][string]$Usuario,
    [Parameter(Mandatory = $true)][string]$Password,
    [int]$Iteraciones = 10,
    [int]$Concurrencia = 5,
    [long]$MedicoID = 0,
    [switch]$Reservar
)

$ErrorActionPreference = "Stop"
$base = $BaseUrl.TrimEnd("/")

function Invoke-Login {
    param([string]$Usuario, [string]$Password)
    return Invoke-RestMethod -Method Post `
        -Uri "$base/api/v1/auth/login" `
        -ContentType "application/json" `
        -Body (@{ username = $Usuario; password = $Password } | ConvertTo-Json)
}

function Get-HttpStatus {
    param($ErrorRecord)
    try {
        return [int]$ErrorRecord.Exception.Response.StatusCode
    } catch {
        return 0
    }
}

Write-Host "Obteniendo un médico y un slot de referencia..."
$login = Invoke-Login -Usuario $Usuario -Password $Password
$headers = @{ Authorization = "Bearer $($login.data.access_token)" }

if ($MedicoID -le 0) {
    $medicos = Invoke-RestMethod -Method Get `
        -Uri "$base/api/v1/portal/medicos?limit=1" `
        -Headers $headers
    if (-not $medicos.data -or $medicos.data.Count -eq 0) {
        throw "No hay médicos disponibles para la prueba. Crea datos de staging o pasa -MedicoID."
    }
    $MedicoID = [long]$medicos.data[0].id
}

$desde = (Get-Date).AddDays(1).ToString("yyyy-MM-dd")
$hasta = (Get-Date).AddDays(7).ToString("yyyy-MM-dd")
$disponibilidad = Invoke-RestMethod -Method Get `
    -Uri "$base/api/v1/portal/disponibilidad?medico_id=$MedicoID&desde=$desde&hasta=$hasta" `
    -Headers $headers
if (-not $disponibilidad.data.slots -or $disponibilidad.data.slots.Count -eq 0) {
    throw "No hay slots en los próximos 7 días para el médico $MedicoID. Ajusta la agenda de staging."
}
$slot = $disponibilidad.data.slots[0].inicio
Write-Host "Médico=$MedicoID Slot=$slot Reservar=$($Reservar.IsPresent)"

$jobs = 1..$Concurrencia | ForEach-Object {
    Start-Job -ScriptBlock {
        param($base, $usuario, $password, $medicoID, $slot, $reservar, $iteraciones)

        function Invoke-LoginLocal {
            return Invoke-RestMethod -Method Post `
                -Uri "$base/api/v1/auth/login" `
                -ContentType "application/json" `
                -Body (@{ username = $usuario; password = $password } | ConvertTo-Json)
        }
        function Get-StatusLocal {
            param($ErrorRecord)
            try { return [int]$ErrorRecord.Exception.Response.StatusCode } catch { return 0 }
        }

        $r = [ordered]@{
            login_ok = 0; login_429 = 0; login_error = 0
            disponibilidad_ok = 0; disponibilidad_error = 0
            reserva_ok = 0; reserva_409 = 0; reserva_error = 0
            latencia_login_ms = @(); latencia_disponibilidad_ms = @()
        }

        for ($i = 0; $i -lt $iteraciones; $i++) {
            $token = $null
            $sw = [Diagnostics.Stopwatch]::StartNew()
            try {
                $login = Invoke-LoginLocal
                $token = $login.data.access_token
                $r.login_ok++
            } catch {
                $status = Get-StatusLocal $_
                if ($status -eq 429) { $r.login_429++ } else { $r.login_error++ }
            } finally {
                $sw.Stop()
                $r.latencia_login_ms += $sw.ElapsedMilliseconds
            }
            if (-not $token) { continue }

            $headers = @{ Authorization = "Bearer $token" }
            $sw = [Diagnostics.Stopwatch]::StartNew()
            try {
                $desde = (Get-Date).AddDays(1).ToString("yyyy-MM-dd")
                $hasta = (Get-Date).AddDays(7).ToString("yyyy-MM-dd")
                $null = Invoke-RestMethod -Method Get `
                    -Uri "$base/api/v1/portal/disponibilidad?medico_id=$medicoID&desde=$desde&hasta=$hasta" `
                    -Headers $headers
                $r.disponibilidad_ok++
            } catch {
                $r.disponibilidad_error++
            } finally {
                $sw.Stop()
                $r.latencia_disponibilidad_ms += $sw.ElapsedMilliseconds
            }

            if ($reservar) {
                try {
                    $body = @{
                        medico_id = $medicoID
                        fecha_hora_inicio = $slot
                        tipo = "primera_vez"
                        motivo = "prueba de carga Fase 9"
                    } | ConvertTo-Json
                    $null = Invoke-RestMethod -Method Post `
                        -Uri "$base/api/v1/portal/citas" `
                        -Headers $headers `
                        -ContentType "application/json" `
                        -Body $body
                    $r.reserva_ok++
                } catch {
                    if ((Get-StatusLocal $_) -eq 409) { $r.reserva_409++ } else { $r.reserva_error++ }
                }
            }
        }

        [pscustomobject]$r
    } -ArgumentList $base, $Usuario, $Password, $MedicoID, $slot, $Reservar.IsPresent, $Iteraciones
}

$resultados = $jobs | Wait-Job | Receive-Job
$jobs | Remove-Job -Force

$resumen = [ordered]@{
    workers = $Concurrencia
    iteraciones_por_worker = $Iteraciones
    login_ok = ($resultados | Measure-Object -Property login_ok -Sum).Sum
    login_429 = ($resultados | Measure-Object -Property login_429 -Sum).Sum
    login_error = ($resultados | Measure-Object -Property login_error -Sum).Sum
    disponibilidad_ok = ($resultados | Measure-Object -Property disponibilidad_ok -Sum).Sum
    disponibilidad_error = ($resultados | Measure-Object -Property disponibilidad_error -Sum).Sum
    reserva_ok = ($resultados | Measure-Object -Property reserva_ok -Sum).Sum
    reserva_409 = ($resultados | Measure-Object -Property reserva_409 -Sum).Sum
    reserva_error = ($resultados | Measure-Object -Property reserva_error -Sum).Sum
}
$resumen.GetEnumerator() | ForEach-Object { "{0}={1}" -f $_.Key, $_.Value }

function Get-Percentil {
    param([int[]]$Valores, [double]$Percentil)
    if (-not $Valores -or $Valores.Count -eq 0) { return 0 }
    $ordenados = @($Valores | Sort-Object)
    $indice = [Math]::Ceiling(($Percentil / 100) * $ordenados.Count) - 1
    if ($indice -lt 0) { $indice = 0 }
    return $ordenados[$indice]
}

$latenciaLogin = @($resultados | ForEach-Object { $_.latencia_login_ms } | ForEach-Object { [int]$_ })
$latenciaDisponibilidad = @($resultados | ForEach-Object { $_.latencia_disponibilidad_ms } | ForEach-Object { [int]$_ })
Write-Output ("login_p50_ms={0} login_p95_ms={1}" -f (Get-Percentil $latenciaLogin 50), (Get-Percentil $latenciaLogin 95))
Write-Output ("disponibilidad_p50_ms={0} disponibilidad_p95_ms={1}" -f (Get-Percentil $latenciaDisponibilidad 50), (Get-Percentil $latenciaDisponibilidad 95))
Write-Host "Prueba terminada. Revisa /metrics, logs y outbox antes de cerrar la validación."
