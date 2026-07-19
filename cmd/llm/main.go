package main

import (
	"encoding/json"
	"fmt"
	"os"

	"ai_lab_vllm/internal/devops"
)

// OllamaRequest is the payload sent to Ollama's native endpoint.
type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Format string `json:"format"` // Forces structured output
}

const ollamaURL = "http://localhost:11434/api/generate"

func main() {
	// Simulating the error lines extracted in "State 2"
	logsFiltrados := "[2026-07-19 12:00:01] ERROR: falha na conexao com o banco de dados - connection refused na porta 5432"

	prompt := fmt.Sprintf(`Você é um SRE. Analise o erro abaixo e responda APENAS com um JSON contendo as chaves "causa_raiz", "severidade" (alta/baixa) e "comando_sugerido".
	Log: %s`, logsFiltrados)

	reqData := OllamaRequest{
		Model:  "qwen2.5:0.5b",
		Prompt: prompt,
		Stream: false,
		Format: "json",
	}

	bodyBytes, err := devops.PostJSON(ollamaURL, reqData)
	if err != nil {
		fmt.Println("Erro de comunicação com o Ollama:", err)
		os.Exit(1)
	}

	// Extracting the string from the raw API response
	var ollamaResponse struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(bodyBytes, &ollamaResponse); err != nil {
		fmt.Println("Erro ao decodificar resposta do Ollama:", err)
		os.Exit(1)
	}

	// Safety validation: parse the AI text into the Go struct
	var analise devops.Analysis
	if err := json.Unmarshal([]byte(ollamaResponse.Response), &analise); err != nil {
		fmt.Println("Erro de segurança! IA alucinou o formato JSON:", err)
		os.Exit(1)
	}

	fmt.Println("Causa Raiz:", analise.CausaRaiz)
	fmt.Println("Severidade:", analise.Severidade)
	// Safe action: we only display the suggested command, never execute it.
	fmt.Printf("Comando sugerido (revise antes de executar): %s\n", analise.ComandoSugerido)
}
