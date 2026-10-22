package entities

import "testing"

func TestEstadoCitaPuedePasarA(t *testing.T) {
	tests := []struct {
		nombre   string
		desde    EstadoCita
		hacia    EstadoCita
		esperado bool
	}{
		{"programada a confirmada", EstadoCitaProgramada, EstadoCitaConfirmada, true},
		{"programada a cancelada", EstadoCitaProgramada, EstadoCitaCancelada, true},
		{"programada a reprogramada", EstadoCitaProgramada, EstadoCitaReprogramada, true},
		{"programada a atendida", EstadoCitaProgramada, EstadoCitaAtendida, false},
		{"confirmada a atendida", EstadoCitaConfirmada, EstadoCitaAtendida, true},
		{"confirmada a no asistio", EstadoCitaConfirmada, EstadoCitaNoAsistio, true},
		{"confirmada a cancelada", EstadoCitaConfirmada, EstadoCitaCancelada, true},
		{"confirmada a reprogramada", EstadoCitaConfirmada, EstadoCitaReprogramada, true},
		{"confirmada a programada", EstadoCitaConfirmada, EstadoCitaProgramada, false},
		{"atendida a confirmada", EstadoCitaAtendida, EstadoCitaConfirmada, false},
		{"cancelada a programada", EstadoCitaCancelada, EstadoCitaProgramada, false},
		{"reprogramada a confirmada", EstadoCitaReprogramada, EstadoCitaConfirmada, false},
	}

	for _, tt := range tests {
		t.Run(tt.nombre, func(t *testing.T) {
			if got := tt.desde.PuedePasarA(tt.hacia); got != tt.esperado {
				t.Fatalf("%s → %s: se esperaba %v y se obtuvo %v",
					tt.desde, tt.hacia, tt.esperado, got)
			}
		})
	}
}

func TestEstadoCitaEsTerminal(t *testing.T) {
	terminales := []EstadoCita{
		EstadoCitaAtendida,
		EstadoCitaNoAsistio,
		EstadoCitaCancelada,
		EstadoCitaReprogramada,
	}
	for _, estado := range terminales {
		if !estado.EsTerminal() {
			t.Errorf("%s debería ser terminal", estado)
		}
	}

	activos := []EstadoCita{EstadoCitaProgramada, EstadoCitaConfirmada}
	for _, estado := range activos {
		if estado.EsTerminal() {
			t.Errorf("%s no debería ser terminal", estado)
		}
	}
}

func TestEstadoCitaValido(t *testing.T) {
	if (EstadoCita("inventado")).Valido() {
		t.Fatal("un estado desconocido no debería ser válido")
	}
	if !EstadoCitaProgramada.Valido() {
		t.Fatal("programada debería ser válido")
	}
}
