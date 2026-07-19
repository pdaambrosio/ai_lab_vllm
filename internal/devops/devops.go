// Package devops defines the data contract shared between the backends
// (Ollama and vLLM) along with common HTTP helpers for log analysis.
package devops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Analysis is the contract we expect the AI to honor when analyzing a log.
type Analysis struct {
	CausaRaiz       string `json:"causa_raiz"`
	Severidade      string `json:"severidade"`
	ComandoSugerido string `json:"comando_sugerido"`
}

// httpClient with a timeout so the program does not hang if the server stalls.
var httpClient = &http.Client{Timeout: 60 * time.Second}

// PostJSON marshals reqData, issues a POST, and returns the response body.
// It returns an error if the HTTP status is not 2xx, including the body for diagnostics.
func PostJSON(url string, reqData any) ([]byte, error) {
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return nil, fmt.Errorf("falha ao serializar requisição: %w", err)
	}

	resp, err := httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("falha de comunicação com %s: %w", url, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler resposta: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return bodyBytes, nil
}
