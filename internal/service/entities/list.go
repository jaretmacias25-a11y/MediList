package entities

// ListOptions contiene los parámetros comunes de paginación y búsqueda.
type ListOptions struct {
	Limit       int
	Offset      int
	Search      string
	SoloActivos bool
}

// Normalized aplica límites seguros a la paginación.
func (o ListOptions) Normalized() ListOptions {
	if o.Limit <= 0 {
		o.Limit = 50
	}
	if o.Limit > 200 {
		o.Limit = 200
	}
	if o.Offset < 0 {
		o.Offset = 0
	}
	return o
}
