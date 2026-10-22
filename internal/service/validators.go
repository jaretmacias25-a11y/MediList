package service

import (
	"strings"
	"time"

	"medilist/internal/service/entities"
)

func normalizarPersona(p *entities.Persona) {
	p.TipoDocumento = strings.ToUpper(strings.TrimSpace(p.TipoDocumento))
	p.NumeroDocumento = strings.TrimSpace(p.NumeroDocumento)
	p.Nombres = strings.TrimSpace(p.Nombres)
	p.Apellidos = strings.TrimSpace(p.Apellidos)
	p.Sexo = strings.ToLower(strings.TrimSpace(p.Sexo))
	p.Telefono = strings.TrimSpace(p.Telefono)
	p.Email = strings.ToLower(strings.TrimSpace(p.Email))
	p.Direccion = strings.TrimSpace(p.Direccion)
}

func validarPersona(p entities.Persona) error {
	if p.TipoDocumento == "" {
		return entities.NewValidationError("persona.tipo_documento es obligatorio")
	}
	if p.NumeroDocumento == "" {
		return entities.NewValidationError("persona.numero_documento es obligatorio")
	}
	if p.Nombres == "" {
		return entities.NewValidationError("persona.nombres es obligatorio")
	}
	if p.Apellidos == "" {
		return entities.NewValidationError("persona.apellidos es obligatorio")
	}
	if p.FechaNacimiento.IsZero() {
		return entities.NewValidationError("persona.fecha_nacimiento es obligatoria (formato YYYY-MM-DD)")
	}
	if p.FechaNacimiento.After(time.Now()) {
		return entities.NewValidationError("persona.fecha_nacimiento no puede estar en el futuro")
	}
	switch p.Sexo {
	case "masculino", "femenino", "otro", "no_especificado":
	default:
		return entities.NewValidationError("persona.sexo debe ser masculino, femenino, otro o no_especificado")
	}
	if p.Email != "" && !strings.Contains(p.Email, "@") {
		return entities.NewValidationError("persona.email no tiene un formato válido")
	}
	return nil
}

func normalizarPaciente(p *entities.Paciente) {
	normalizarPersona(&p.Persona)
	p.GrupoSanguineo = strings.ToUpper(strings.TrimSpace(p.GrupoSanguineo))
	p.FactorRH = strings.TrimSpace(p.FactorRH)
	p.ContactoEmergenciaNombre = strings.TrimSpace(p.ContactoEmergenciaNombre)
	p.ContactoEmergenciaTelefono = strings.TrimSpace(p.ContactoEmergenciaTelefono)
}

func validarPaciente(p entities.Paciente) error {
	if err := validarPersona(p.Persona); err != nil {
		return err
	}
	switch p.GrupoSanguineo {
	case "", "A", "B", "AB", "O":
	default:
		return entities.NewValidationError("grupo_sanguineo debe ser A, B, AB, O o vacío")
	}
	switch p.FactorRH {
	case "", "+", "-":
	default:
		return entities.NewValidationError("factor_rh debe ser +, - o vacío")
	}
	return nil
}

func normalizarMedico(m *entities.Medico) {
	normalizarPersona(&m.Persona)
	m.NumeroLicencia = strings.ToUpper(strings.TrimSpace(m.NumeroLicencia))
	m.TipoProfesional = strings.ToLower(strings.TrimSpace(m.TipoProfesional))
	if m.TipoProfesional == "" {
		m.TipoProfesional = "medico"
	}
}

func validarMedico(m entities.Medico) error {
	if err := validarPersona(m.Persona); err != nil {
		return err
	}
	if m.EspecialidadID <= 0 {
		return entities.NewValidationError("especialidad_id es obligatorio")
	}
	if m.NumeroLicencia == "" {
		return entities.NewValidationError("numero_licencia es obligatorio")
	}
	return nil
}

func normalizarCita(c *entities.Cita) {
	c.Motivo = strings.TrimSpace(c.Motivo)
	if c.DuracionMin == 0 {
		c.DuracionMin = 30
	}
	if c.Tipo == "" {
		c.Tipo = entities.TipoCitaPrimeraVez
	}
}

func validarDatosCita(c entities.Cita) error {
	if c.PacienteID <= 0 {
		return entities.NewValidationError("paciente_id es obligatorio")
	}
	if c.MedicoID <= 0 {
		return entities.NewValidationError("medico_id es obligatorio")
	}
	if c.FechaHoraInicio.IsZero() {
		return entities.NewValidationError("fecha_hora_inicio es obligatoria (formato RFC3339)")
	}
	if c.DuracionMin < 5 || c.DuracionMin > 480 {
		return entities.NewValidationError("duracion_min debe estar entre 5 y 480 minutos")
	}
	if !c.Tipo.Valido() {
		return entities.NewValidationError("tipo debe ser primera_vez, control, urgencia o telemedicina")
	}
	if !c.Estado.Valido() {
		return entities.NewValidationError("estado de cita no válido")
	}
	return nil
}
