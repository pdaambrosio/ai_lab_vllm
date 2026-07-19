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

## Benchmark

`cmd/bench` fires N chat-completion requests at a configurable concurrency level
against any OpenAI-compatible endpoint and reports throughput, aggregate
tokens/sec (from the `usage` field), and latency percentiles.

```bash
go run ./cmd/bench -url http://localhost:11434/v1/chat/completions -model qwen2.5:0.5b -n 40 -c 8
```

Flags: `-url`, `-model`, `-n` (total requests), `-c` (concurrency),
`-max-tokens`, `-prompt`.

Note on interpreting results: raising `-c` only increases aggregate throughput
when the GPU has spare compute. Ollama by default also serves few requests in
parallel (`OLLAMA_NUM_PARALLEL`, limited further on low-VRAM cards), so extra
concurrent requests queue. vLLM's headline feature is continuous batching, which
batches concurrent requests together — but that advantage only materializes on a
GPU with compute headroom. On a small/entry GPU that is already compute-bound,
concurrency stops helping (or even hurts) for **both** engines.

### Measured results (GTX 1650 Mobile, 4 GB — Qwen2.5-0.5B)

Same bench (`-n 24 -max-tokens 64`, identical prompt, warm cache):

| Engine | c=1 req/s | c=1 tok/s | c=1 mean lat | c=8 req/s | c=8 tok/s | c=8 mean lat |
|---|---|---|---|---|---|---|
| Ollama | 0.62 | 25.6 | 1.60s | 0.71 | 28.2 | 9.96s |
| vLLM   | 1.21 | 47.0 | 0.83s | 0.81 | 32.4 | 9.38s |

Takeaways on this hardware:

- **Single request: vLLM wins clearly** (~47 vs ~26 tok/s, ~2x lower latency) —
  optimized kernels + prefix caching, even falling back to the Triton attention
  backend (FlashAttention-2 needs compute capability >= 8.0; Turing is 7.5).
- **Under concurrency the two roughly tie**, and neither scales up: the 4 GB
  entry GPU saturates on compute, so batching adds contention without throughput.
  vLLM's batching win needs a larger GPU to show.

vLLM was run in Docker, tuned for the 4 GB Turing card (fp16, small context/batch):

```bash
docker run --rm --gpus all -p 8000:8000 --ipc=host \
  -e PYTORCH_CUDA_ALLOC_CONF=expandable_segments:True \
  vllm/vllm-openai:latest \
  --model Qwen/Qwen2.5-0.5B-Instruct \
  --dtype half --max-model-len 1024 --max-num-seqs 16 \
  --gpu-memory-utilization 0.75 --enforce-eager
```

(Requires `nvidia-container-toolkit`. On 4 GB, free the GPU first — e.g.
`ollama stop qwen2.5:0.5b` — and keep `--gpu-memory-utilization` conservative to
avoid CUDA OOM.)

## Tests

```bash
go test ./...
```
