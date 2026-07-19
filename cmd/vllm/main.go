package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"ai_lab_vllm/internal/devops"
)

// 1. Structs compatible with the OpenAI API (vLLM)
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ResponseFormat struct {
	Type string `json:"type"` // "json_object" to force structured output
}

type VLLMRequest struct {
	Model          string         `json:"model"`
	Messages       []Message      `json:"messages"`
	ResponseFormat ResponseFormat `json:"response_format"`
	Temperature    float64        `json:"temperature"`
}

// VLLMResponse captures the nested OpenAI-format response.
type VLLMResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Defaults point to vLLM (port 8000). But since Ollama also exposes an
// OpenAI-compatible API under /v1, you can point this same client at a local
// Ollama using the flags/env below:
//
//	go run ./cmd/vllm -url http://localhost:11434/v1/chat/completions -model qwen2.5:0.5b
//
// It also honors the VLLM_URL and VLLM_MODEL environment variables.
const (
	defaultURL   = "http://localhost:8000/v1/chat/completions"
	defaultModel = "Qwen/Qwen2.5-0.5B-Instruct"
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	url := flag.String("url", envOr("VLLM_URL", defaultURL), "OpenAI-compatible endpoint (/v1/chat/completions)")
	model := flag.String("model", envOr("VLLM_MODEL", defaultModel), "model name (e.g. qwen2.5:0.5b on Ollama)")
	flag.Parse()

	// The captured error log
	logErro := "ERROR: falha de resolucao de DNS para o endpoint do banco de dados na AWS."

	// 2. Build the payload in OpenAI Chat format
	reqData := VLLMRequest{
		Model: *model,
		Messages: []Message{
			{
				Role:    "system",
				Content: `Você é um engenheiro de infraestrutura. Baseado no erro, sugira um comando Linux de diagnóstico (como ping ou nslookup). Responda APENAS com um JSON contendo: "causa_raiz", "severidade" e "comando_sugerido".`,
			},
			{
				Role:    "user",
				Content: logErro,
			},
		},
		ResponseFormat: ResponseFormat{Type: "json_object"},
		Temperature:    0.1, // Low temperature for more deterministic responses
	}

	// 3. Send to the v1/chat/completions endpoint
	fmt.Printf("[*] Enviando log para análise em %s (modelo: %s)...\n", *url, *model)
	bodyBytes, err := devops.PostJSON(*url, reqData)
	if err != nil {
		fmt.Println("Erro fatal ao contatar o vLLM:", err)
		os.Exit(1)
	}

	// 4. Parse the structured OpenAI/vLLM response
	var vllmResp VLLMResponse
	if err := json.Unmarshal(bodyBytes, &vllmResp); err != nil || len(vllmResp.Choices) == 0 {
		fmt.Println("Erro ao decodificar resposta do vLLM:", err)
		os.Exit(1)
	}

	// The returned text should be the JSON of our contract.
	textoJsonDaIA := vllmResp.Choices[0].Message.Content

	// 5. Parse the inner text into the final automation struct
	var analise devops.Analysis
	if err := json.Unmarshal([]byte(textoJsonDaIA), &analise); err != nil {
		fmt.Println("Erro: O vLLM não retornou o contrato JSON esperado.", err)
		os.Exit(1)
	}

	fmt.Println("\n--- DIAGNÓSTICO DO vLLM ---")
	fmt.Printf("Causa Raiz: %s\n", analise.CausaRaiz)
	fmt.Printf("Severidade: %s\n", analise.Severidade)
	fmt.Printf("Comando Sugerido: %s\n", analise.ComandoSugerido)
	fmt.Println("---------------------------")

	// 6. Safe action: we do NOT execute the command coming from the AI.
	// Running LLM output through bash is command injection (e.g. rm -rf).
	// We only display it for human review.
	fmt.Println("\n[!] Comando NÃO executado automaticamente por segurança.")
	fmt.Println("    Revise e execute manualmente se apropriado:")
	fmt.Printf("    $ %s\n", analise.ComandoSugerido)
}
