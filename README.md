# ai_lab_vllm

A lab for DevOps log analysis with a local LLM. Two Go clients run the same task
(extract `causa_raiz`, `severidade`, and `comando_sugerido` from an error log)
against two inference backends.

## Ollama vs. vLLM

| | Ollama | vLLM |
|---|---|---|
| Role | Run an LLM locally, easy (`ollama pull/run`) | High-performance inference engine (GPU, production) |
| Default port | `11434` | `8000` |
| Native API | `/api/generate` | — |
| OpenAI-compatible API | `/v1/chat/completions` | `/v1/chat/completions` |
| Model name | `qwen2.5:0.5b` | `Qwen/Qwen2.5-0.5B-Instruct` |

**Key point:** both speak the OpenAI format at `/v1/chat/completions`. That is
why the `cmd/vllm` client works against a local Ollama — you do not need to
install vLLM. Only the **port** and the **model name** differ.

## Layout

```
cmd/llm/            client for Ollama's NATIVE API (/api/generate)
cmd/vllm/           client for the OpenAI-compatible API (/v1/chat/completions)
internal/devops/    Analysis type (contract) + PostJSON (timeout + HTTP status)
```

## Running

Prerequisite: Ollama running with the model pulled.

```bash
ollama pull qwen2.5:0.5b
```

Ollama native client:

```bash
go run ./cmd/llm
```

OpenAI-format client pointed at a **local Ollama** (port 11434):

```bash
go run ./cmd/vllm -url http://localhost:11434/v1/chat/completions -model qwen2.5:0.5b
```

Against a real vLLM (defaults, port 8000):

```bash
go run ./cmd/vllm
```

It also honors the `VLLM_URL` and `VLLM_MODEL` environment variables.

## Security

The `comando_sugerido` returned by the AI is **only displayed, never executed**.
Running LLM output through `bash -c` would be command injection (e.g. `rm -rf`).
Review and run it manually if appropriate.

## Tests

```bash
go test ./...
```
