package observability

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// RotatingFileWriter escribe logs JSON en un archivo y rota por tamaño.
// Es seguro para uso concurrente porque slog puede escribir desde varias
// solicitudes al mismo tiempo.
type RotatingFileWriter struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	backups  int
	file     *os.File
	size     int64
	closed   bool
}

// NewRotatingFileWriter crea el archivo, su directorio y prepara la rotación.
func NewRotatingFileWriter(path string, maxBytes int64, backups int) (*RotatingFileWriter, error) {
	if path == "" {
		return nil, fmt.Errorf("la ruta del archivo de log no puede estar vacía")
	}
	if backups < 1 {
		backups = 1
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("crear directorio de logs: %w", err)
	}
	w := &RotatingFileWriter{path: path, maxBytes: maxBytes, backups: backups}
	if err := w.abrir(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *RotatingFileWriter) abrir() error {
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return fmt.Errorf("abrir archivo de logs: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return fmt.Errorf("inspeccionar archivo de logs: %w", err)
	}
	w.file = file
	w.size = info.Size()
	return nil
}

func (w *RotatingFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, os.ErrClosed
	}
	if w.file == nil {
		if err := w.abrir(); err != nil {
			return 0, err
		}
	}

	var rotateErr error
	if w.maxBytes > 0 && w.size > 0 && w.size+int64(len(p)) > w.maxBytes {
		rotateErr = w.rotar()
		if w.file == nil {
			return 0, rotateErr
		}
	}
	n, writeErr := w.file.Write(p)
	w.size += int64(n)
	return n, errors.Join(rotateErr, writeErr)
}

func (w *RotatingFileWriter) rotar() error {
	closeErr := w.file.Close()
	w.file = nil

	_ = os.Remove(fmt.Sprintf("%s.%d", w.path, w.backups))
	var rotateErr error
	for i := w.backups - 1; i >= 1; i-- {
		origen := fmt.Sprintf("%s.%d", w.path, i)
		destino := fmt.Sprintf("%s.%d", w.path, i+1)
		if err := os.Rename(origen, destino); err != nil && !os.IsNotExist(err) {
			rotateErr = fmt.Errorf("rotar archivo de logs: %w", err)
			break
		}
	}
	if rotateErr == nil {
		if err := os.Rename(w.path, w.path+".1"); err != nil && !os.IsNotExist(err) {
			rotateErr = fmt.Errorf("renombrar archivo de logs actual: %w", err)
		}
	}
	openErr := w.abrir()
	return errors.Join(closeErr, rotateErr, openErr)
}

// Close cierra el archivo. Es seguro llamarlo más de una vez.
func (w *RotatingFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}
