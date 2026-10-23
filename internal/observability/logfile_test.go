package observability

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRotatingFileWriterRotaYConservaCopias(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "api.jsonl")
	writer, err := NewRotatingFileWriter(path, 10, 2)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := writer.Write([]byte("1234567890")); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("abcdef")); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("XYZ")); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("123456")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	actual := leerArchivo(t, path)
	if actual != "123456" {
		t.Fatalf("archivo actual inesperado: %q", actual)
	}
	copia1 := leerArchivo(t, path+".1")
	if copia1 != "abcdefXYZ" {
		t.Fatalf("copia 1 inesperada: %q", copia1)
	}
	copia2 := leerArchivo(t, path+".2")
	if copia2 != "1234567890" {
		t.Fatalf("copia 2 inesperada: %q", copia2)
	}
}

func TestRotatingFileWriterCierraDirectorioYEsIdempotente(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "api.jsonl")
	writer, err := NewRotatingFileWriter(path, 1024, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("linea\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("despues\n")); err == nil {
		t.Fatal("escribir después de cerrar debe fallar")
	}
}

func leerArchivo(t *testing.T, path string) string {
	t.Helper()
	contenido, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contenido)
}
