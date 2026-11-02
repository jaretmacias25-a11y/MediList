package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPIgnoraXForwardedForSinProxyConfiable(t *testing.T) {
	if err := SetTrustedProxies(nil); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "198.51.100.10:4567"
	req.Header.Set("X-Forwarded-For", "203.0.113.99")

	if got := clientIP(req); got != "198.51.100.10" {
		t.Fatalf("IP esperada 198.51.100.10, obtenida %s", got)
	}
}

func TestClientIPUsaXForwardedForDeProxyConfiable(t *testing.T) {
	if err := SetTrustedProxies([]string{"127.0.0.1/32", "10.0.0.0/8"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = SetTrustedProxies(nil) })

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:4567"
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.2")

	if got := clientIP(req); got != "203.0.113.7" {
		t.Fatalf("IP esperada 203.0.113.7, obtenida %s", got)
	}
}

func TestClientIPTomaLaEntradaNoConfiableMasCercana(t *testing.T) {
	if err := SetTrustedProxies([]string{"127.0.0.1/32"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = SetTrustedProxies(nil) })

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:4567"
	req.Header.Set("X-Forwarded-For", "203.0.113.99, 198.51.100.7")

	if got := clientIP(req); got != "198.51.100.7" {
		t.Fatalf("IP esperada 198.51.100.7, obtenida %s", got)
	}
}
