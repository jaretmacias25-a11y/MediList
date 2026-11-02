param(
    [string]$DbHost = "localhost",
    [string]$DbUser = "postgres",
    [string]$Password = $env:PGPASSWORD,
    [string]$Database = "medilist_e2e_fase10",
    [int]$Port = 8096,
    [switch]$KeepDatabase
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$base = "http://localhost:$Port"
$outbox = Join-Path $root "outbox\e2e-fase10"
$pidFile = Join-Path $env:TEMP "medilist-e2e-fase10.pid"

$psql = Get-ChildItem "C:\Program Files\PostgreSQL\*\bin\psql.exe" -ErrorAction SilentlyContinue |
    Sort-Object FullName -Descending |
    Select-Object -First 1 -ExpandProperty FullName
if (-not $psql) { throw "No se encontró psql.exe." }
if ([string]::IsNullOrWhiteSpace($Password)) { throw "Define -Password o PGPASSWORD." }

$env:PGPASSWORD = $Password
$usuarioUrl = [uri]::EscapeDataString($DbUser)
$passwordUrl = [uri]::EscapeDataString($Password)
$apiProc = $null

function Stop-Api {
    if ($script:apiProc -and -not $script:apiProc.HasExited) {
        Stop-Process -Id $script:apiProc.Id -Force -ErrorAction SilentlyContinue
        Start-Sleep -Milliseconds 300
    }
    $script:apiProc = $null
}

function Start-Api {
    param([bool]$Portal, [bool]$Derivaciones, [bool]$Transferencias)
    Stop-Api
    $env:DATABASE_URL = "postgres://${usuarioUrl}:${passwordUrl}@${DbHost}:5432/${Database}?sslmode=disable"
    $env:HTTP_PORT = "$Port"
    $env:APP_ENV = "development"
    $env:LOG_FILE = ""
    $env:EMAIL_PROVIDER = "outbox"
    $env:EMAIL_OUTBOX_DIR = $outbox
    $env:PORTAL_PACIENTE_ENABLED = if ($Portal) { "true" } else { "false" }
    $env:DERIVACIONES_ENABLED = if ($Derivaciones) { "true" } else { "false" }
    $env:TRANSFERENCIAS_PACIENTE_ENABLED = if ($Transferencias) { "true" } else { "false" }
    $env:NOTIFICATION_INTERVAL_SECONDS = "1"
    $env:NOTIFICATION_BATCH_SIZE = "20"
    $env:NOTIFICATION_REMINDER_HOURS = "24"
    $env:RATE_LIMIT_ENABLED = "true"

    $script:apiProc = Start-Process -FilePath (Join-Path $root "bin\medilist-e2e.exe") `
        -WorkingDirectory $root -PassThru -WindowStyle Hidden
    Set-Content -Path $pidFile -Value $script:apiProc.Id
    $ok = $false
    for ($i = 0; $i -lt 60; $i++) {
        Start-Sleep -Milliseconds 250
        try {
            $health = Invoke-RestMethod -Uri "$base/health" -TimeoutSec 2
            if ($health.status -eq "ok") { $ok = $true; break }
        } catch {}
    }
    if (-not $ok) { throw "La API no arrancó en $base." }
}

function Invoke-Api {
    param(
        [string]$Method,
        [string]$Path,
        [string]$Token,
        $Body,
        [int]$Expected = 200
    )
    $headers = @{}
    if ($Token) { $headers.Authorization = "Bearer $Token" }
    $params = @{
        Method = $Method
        Uri = "$base$Path"
        Headers = $headers
        UseBasicParsing = $true
        TimeoutSec = 15
    }
    if ($null -ne $Body) {
        $params.ContentType = "application/json"
        $params.Body = ($Body | ConvertTo-Json -Depth 8 -Compress)
    }
    $status = 0
    $content = ""
    try {
        $resp = Invoke-WebRequest @params
        $status = [int]$resp.StatusCode
        $content = $resp.Content
    } catch {
        if ($_.Exception.Response) { $status = [int]$_.Exception.Response.StatusCode }
        $content = $_.ErrorDetails.Message
        if (-not $content) { $content = $_.Exception.Message }
    }
    if ($status -ne $Expected) {
        throw "$Method $Path devolvió $status, esperado $Expected. Cuerpo: $content"
    }
    if ([string]::IsNullOrWhiteSpace($content)) { return $null }
    return $content | ConvertFrom-Json
}

function Get-VerificationToken {
    for ($i = 0; $i -lt 40; $i++) {
        $mail = Get-ChildItem $outbox -Filter "*verificacion_email*.eml" -ErrorAction SilentlyContinue |
            Sort-Object LastWriteTime -Descending |
            Select-Object -First 1
        if ($mail) {
            $texto = Get-Content -Raw -Path $mail.FullName
            if ($texto -match 'token=([^\s&<"]+)') {
                return [uri]::UnescapeDataString($Matches[1])
            }
        }
        Start-Sleep -Milliseconds 250
    }
    throw "No se encontró el token de verificación en $outbox."
}

function Get-FirstSlot {
    param([long]$MedicoID)
    $desde = (Get-Date).AddDays(1).ToString("yyyy-MM-dd")
    $hasta = (Get-Date).AddDays(7).ToString("yyyy-MM-dd")
    $disp = Invoke-Api GET "/api/v1/portal/disponibilidad?medico_id=$MedicoID&desde=$desde&hasta=$hasta" $patientToken $null 200
    if (-not $disp.data.slots -or $disp.data.slots.Count -eq 0) {
        throw "No hay slots para el médico $MedicoID."
    }
    return $disp.data.slots[0].inicio
}

try {
    if (Test-Path $pidFile) { Stop-Api }

    $exists = & $psql -U $DbUser -h $DbHost -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '$Database'"
    if ("$exists".Trim() -eq "1") {
        & $psql -U $DbUser -h $DbHost -d postgres -v ON_ERROR_STOP=1 -c "DROP DATABASE $Database"
    }
    & $psql -U $DbUser -h $DbHost -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE $Database"
    if ($LASTEXITCODE -ne 0) { throw "No se pudo crear la base E2E." }

    & (Join-Path $root "scripts\migrate.ps1") -DbHost $DbHost -DbUser $DbUser -Password $Password -Database $Database
    if ($LASTEXITCODE -ne 0) { throw "Fallaron las migraciones." }

    Remove-Item -Recurse -Force $outbox -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force $outbox | Out-Null

    Push-Location $root
    try {
        go build -o .\bin\medilist-e2e.exe .\cmd\api
        if ($LASTEXITCODE -ne 0) { throw "No se pudo compilar la API." }
    } finally {
        Pop-Location
    }

    Write-Host "1/7 Validando feature flags apagados..."
    Start-Api -Portal $false -Derivaciones $false -Transferencias $false
    $null = Invoke-Api GET "/api/v1/portal/perfil" $null $null 503
    $null = Invoke-Api GET "/api/v1/medico/derivaciones" $null $null 503
    $null = Invoke-Api POST "/api/v1/pacientes/1/transferir" $null @{medico_destino_id=1;motivo="prueba";version=1} 503

    Write-Host "2/7 Activando feature flags..."
    Start-Api -Portal $true -Derivaciones $true -Transferencias $true

    $admin = Invoke-Api POST "/api/v1/auth/login" $null @{username="admin";password="Admin12345!2026"} 200
    $adminToken = $admin.data.access_token
    $especialidades = Invoke-Api GET "/api/v1/especialidades?limit=50" $adminToken $null 200
    $medgen = $especialidades.data | Where-Object { $_.codigo -eq "MED-GEN" } | Select-Object -First 1
    $otorrino = $especialidades.data | Where-Object { $_.codigo -eq "OTORRINO" } | Select-Object -First 1
    if (-not $medgen -or -not $otorrino) { throw "No se encontraron MED-GEN/OTORRINO." }

    Write-Host "3/7 Creando médicos y horarios..."
    $doctorBase = @{
        persona = @{
            tipo_documento = "CEDULA"; numero_documento = "1700000101"
            nombres = "Médico"; apellidos = "E2E"; fecha_nacimiento = "1980-02-20"
            sexo = "masculino"; telefono = "0977777777"
            email = "medgen.e2e@example.test"; direccion = "Quito"
        }
        especialidad_id = [int64]$medgen.id; numero_licencia = "LIC-E2E-MG"
        tipo_profesional = "medico"; duracion_cita_min = 30; acepta_portal = $true
    }
    $medgenDoctor = Invoke-Api POST "/api/v1/medicos" $adminToken $doctorBase 201
    $medgenDoctorID = [int64]$medgenDoctor.data.id

    $specialistBase = @{
        persona = @{
            tipo_documento = "CEDULA"; numero_documento = "1700000102"
            nombres = "Especialista"; apellidos = "E2E"; fecha_nacimiento = "1980-02-20"
            sexo = "masculino"; telefono = "0977777778"
            email = "otorrino.e2e@example.test"; direccion = "Quito"
        }
        especialidad_id = [int64]$otorrino.id; numero_licencia = "LIC-E2E-OT"
        tipo_profesional = "medico"; duracion_cita_min = 30; acepta_portal = $true
    }
    $specialist = Invoke-Api POST "/api/v1/medicos" $adminToken $specialistBase 201
    $specialistID = [int64]$specialist.data.id

    $dow = [int](Get-Date).AddDays(1).DayOfWeek
    if ($dow -eq 0) { $dow = 7 }
    $schedule = @{dia_semana=$dow;hora_inicio="09:00";hora_fin="17:00";duracion_slot_min=30;activo=$true}
    $null = Invoke-Api POST "/api/v1/medicos/$medgenDoctorID/horarios" $adminToken $schedule 201
    $null = Invoke-Api POST "/api/v1/medicos/$specialistID/horarios" $adminToken $schedule 201

    Write-Host "4/7 Registrando y verificando paciente..."
    $registro = @{
        username = "paciente.e2e"; email = "paciente.e2e@example.test"
        password = "Paciente12345!2026"; nombres = "Paciente"; apellidos = "E2E"
        tipo_documento = "CEDULA"; numero_documento = "1700000103"
        fecha_nacimiento = "1990-05-10"; sexo = "femenino"; telefono = "0999999999"
        direccion = "Quito"; acepta_terminos = $true; version_terminos = "2026-10"
    }
    $reg = Invoke-Api POST "/api/v1/auth/register" $null $registro 201
    $pacienteID = [int64]$reg.data.paciente_id
    $tokenVerificacion = Get-VerificationToken
    $null = Invoke-Api POST "/api/v1/auth/verify-email" $null @{token=$tokenVerificacion} 200
    $loginPaciente = Invoke-Api POST "/api/v1/auth/login" $null @{username="paciente.e2e";password="Paciente12345!2026"} 200
    $patientToken = $loginPaciente.data.access_token

    Write-Host "5/7 Reserva directa y atención médica..."
    $slotMG = Get-FirstSlot $medgenDoctorID
    $cita = Invoke-Api POST "/api/v1/portal/citas" $patientToken @{
        medico_id = $medgenDoctorID; fecha_hora_inicio = $slotMG
        tipo = "primera_vez"; motivo = "E2E Fase 10"
    } 201
    $citaID = [int64]$cita.data.id

    $doctorUser = Invoke-Api POST "/api/v1/usuarios" $adminToken @{
        username = "medgen.e2e"; email = "medgen.e2e@example.test"
        nombres = "Médico"; apellidos = "E2E"; password = "Medico12345!2026"; roles = @("medico")
    } 201
    $doctorUserID = [int64]$doctorUser.data.id
    $null = Invoke-Api POST "/api/v1/cuentas/vincular-medico" $adminToken @{
        medico_id = $medgenDoctorID; usuario_id = $doctorUserID
    } 204
    & $psql -U $DbUser -h $DbHost -d $Database -v ON_ERROR_STOP=1 -c "UPDATE usuarios SET email_verificado_en = now() WHERE username = 'medgen.e2e';" | Out-Null
    $loginDoctor = Invoke-Api POST "/api/v1/auth/login" $null @{username="medgen.e2e";password="Medico12345!2026"} 200
    $doctorToken = $loginDoctor.data.access_token
    $null = Invoke-Api PATCH "/api/v1/medico/citas/$citaID/estado" $doctorToken @{estado="atendida"} 200
    $null = Invoke-Api POST "/api/v1/medico/citas/$citaID/nota" $doctorToken @{
        motivo = "Consulta E2E"; evaluacion = "Paciente estable"; plan = "Control"
    } 200

    Write-Host "6/7 Derivación y reserva con especialista..."
    $derivacion = Invoke-Api POST "/api/v1/medico/citas/$citaID/derivaciones" $doctorToken @{
        especialidad_destino_id = [int64]$otorrino.id
        medico_destino_id = $specialistID
        motivo = "Evaluación E2E"; prioridad = "normal"
    } 201
    $derivacionID = [int64]$derivacion.data.id
    $listaDerivaciones = Invoke-Api GET "/api/v1/portal/derivaciones" $patientToken $null 200
    if (-not ($listaDerivaciones.data | Where-Object { [int64]$_.id -eq $derivacionID })) {
        throw "La derivación no aparece en el portal del paciente."
    }
    $slotEspecialista = Get-FirstSlot $specialistID
    $citaEspecialista = Invoke-Api POST "/api/v1/portal/citas" $patientToken @{
        medico_id = $specialistID; fecha_hora_inicio = $slotEspecialista
        tipo = "primera_vez"; motivo = "Reserva con derivación"; derivacion_id = $derivacionID
    } 201
    if ([int64]$citaEspecialista.data.derivacion_id -ne $derivacionID) {
        throw "La cita del especialista no conserva la derivación."
    }

    Write-Host "7/7 E2E completado."
    Write-Output "paciente_id=$pacienteID cita_directa=$citaID derivacion_id=$derivacionID cita_especialista=$($citaEspecialista.data.id)"
} finally {
    Stop-Api
    Remove-Item $pidFile -Force -ErrorAction SilentlyContinue
    Remove-Item -Recurse -Force $outbox -ErrorAction SilentlyContinue
    if (-not $KeepDatabase) {
        & $psql -U $DbUser -h $DbHost -d postgres -v ON_ERROR_STOP=0 -c "DROP DATABASE IF EXISTS $Database" | Out-Null
    }
}
