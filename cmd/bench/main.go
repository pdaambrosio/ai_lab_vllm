// Command bench issues N chat-completion requests with a configurable
// concurrency level against any OpenAI-compatible endpoint (Ollama or vLLM)
// and reports throughput, tokens/sec, and latency percentiles.
//
//	go run ./cmd/bench -url http://localhost:11434/v1/chat/completions -model qwen2.5:0.5b -n 40 -c 8
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"sync"
	"time"

	"ai_lab_vllm/internal/devops"
)

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatRequest struct {
	Model          string         `json:"model"`
	Messages       []message      `json:"messages"`
	ResponseFormat responseFormat `json:"response_format"`
	Temperature    float64        `json:"temperature"`
	MaxTokens      int            `json:"max_tokens,omitempty"`
}

// chatResponse only needs the usage block to compute tokens/sec.
type chatResponse struct {
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type result struct {
	latency          time.Duration
	completionTokens int
	err              error
}

func main() {
	url := flag.String("url", "http://localhost:11434/v1/chat/completions", "OpenAI-compatible endpoint")
	model := flag.String("model", "qwen2.5:0.5b", "model name")
	n := flag.Int("n", 20, "total number of requests")
	c := flag.Int("c", 4, "concurrency (number of parallel workers)")
	maxTokens := flag.Int("max-tokens", 128, "max_tokens per request (0 = server default)")
	promptText := flag.String("prompt", "ERROR: DNS resolution failure for the database endpoint on AWS.", "user prompt / log line")
	flag.Parse()

	if *n <= 0 || *c <= 0 {
		fmt.Println("n and c must be > 0")
		os.Exit(2)
	}
	if *c > *n {
		*c = *n
	}

	reqData := chatRequest{
		Model: *model,
		Messages: []message{
			{Role: "system", Content: `You are an infrastructure engineer. Based on the error, suggest a Linux diagnostic command. Respond ONLY with a JSON containing: "causa_raiz", "severidade", "comando_sugerido".`},
			{Role: "user", Content: *promptText},
		},
		ResponseFormat: responseFormat{Type: "json_object"},
		Temperature:    0.1,
		MaxTokens:      *maxTokens,
	}

	fmt.Printf("[*] Benchmark: %d requests, concurrency %d\n", *n, *c)
	fmt.Printf("    target: %s (model: %s)\n\n", *url, *model)

	jobs := make(chan int, *n)
	results := make(chan result, *n)
	var wg sync.WaitGroup

	// Worker pool: exactly *c requests are in flight at any moment.
	for w := 0; w < *c; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				results <- doRequest(*url, reqData)
			}
		}()
	}

	start := time.Now()
	for i := 0; i < *n; i++ {
		jobs <- i
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	var latencies []time.Duration
	var totalCompletion, ok, failed int
	var firstErr error
	for r := range results {
		if r.err != nil {
			failed++
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		ok++
		latencies = append(latencies, r.latency)
		totalCompletion += r.completionTokens
	}
	wall := time.Since(start)

	report(wall, ok, failed, totalCompletion, latencies, firstErr)
}

func doRequest(url string, reqData chatRequest) result {
	start := time.Now()
	body, err := devops.PostJSON(url, reqData)
	elapsed := time.Since(start)
	if err != nil {
		return result{latency: elapsed, err: err}
	}
	var resp chatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return result{latency: elapsed, err: err}
	}
	return result{latency: elapsed, completionTokens: resp.Usage.CompletionTokens}
}

func report(wall time.Duration, ok, failed, totalCompletion int, latencies []time.Duration, firstErr error) {
	fmt.Println("--- RESULTS ---")
	fmt.Printf("Completed:        %d ok, %d failed\n", ok, failed)
	if failed > 0 && firstErr != nil {
		fmt.Printf("First error:      %v\n", firstErr)
	}
	fmt.Printf("Wall time:        %s\n", wall.Round(time.Millisecond))

	if ok == 0 {
		fmt.Println("No successful requests; nothing to summarize.")
		return
	}

	secs := wall.Seconds()
	fmt.Printf("Throughput:       %.2f req/s\n", float64(ok)/secs)
	fmt.Printf("Completion tokens:%d (%.1f tok/s aggregate)\n", totalCompletion, float64(totalCompletion)/secs)

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	var sum time.Duration
	for _, l := range latencies {
		sum += l
	}
	fmt.Println("Latency:")
	fmt.Printf("  min  %s\n", latencies[0].Round(time.Millisecond))
	fmt.Printf("  mean %s\n", (sum / time.Duration(len(latencies))).Round(time.Millisecond))
	fmt.Printf("  p50  %s\n", percentile(latencies, 50).Round(time.Millisecond))
	fmt.Printf("  p95  %s\n", percentile(latencies, 95).Round(time.Millisecond))
	fmt.Printf("  p99  %s\n", percentile(latencies, 99).Round(time.Millisecond))
	fmt.Printf("  max  %s\n", latencies[len(latencies)-1].Round(time.Millisecond))
}

// percentile uses the nearest-rank method on an ascending-sorted slice.
func percentile(sorted []time.Duration, p float64) time.Duration {
	idx := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
