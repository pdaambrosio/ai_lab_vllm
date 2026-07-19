package devops

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostJSON_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Confirm the sent body is the expected JSON.
		body, _ := io.ReadAll(r.Body)
		var got map[string]string
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("corpo enviado não é JSON válido: %v", err)
		}
		if got["model"] != "teste" {
			t.Errorf("model = %q, esperado %q", got["model"], "teste")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	resp, err := PostJSON(srv.URL, map[string]string{"model": "teste"})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if string(resp) != `{"ok":true}` {
		t.Errorf("resposta = %q", string(resp))
	}
}

func TestPostJSON_Non2xxRetornaErroComCorpo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("modelo não carregado"))
	}))
	defer srv.Close()

	_, err := PostJSON(srv.URL, map[string]string{})
	if err == nil {
		t.Fatal("esperava erro para status 500, veio nil")
	}
	if !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "modelo não carregado") {
		t.Errorf("erro deveria conter status e corpo, veio: %v", err)
	}
}

func TestPostJSON_HostInvalido(t *testing.T) {
	_, err := PostJSON("http://127.0.0.1:1/nope", map[string]string{})
	if err == nil {
		t.Fatal("esperava erro de comunicação, veio nil")
	}
}
