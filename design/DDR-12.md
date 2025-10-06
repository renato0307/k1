# Local LLM Architecture for kubectl Command Generation

| Metadata | Value                                           |
|----------|-------------------------------------------------|
| Date     | 2025-10-06                                      |
| Author   | @renato0307                                     |
| Status   | Proposed                                        |
| Tags     | llm, ai, kubectl, ollama, quantization, privacy |
| Updates  | DDR-05                                          |

| Revision | Date       | Author      | Info           |
|----------|------------|-------------|----------------|
| 1        | 2025-10-06 | @renato0307 | Initial design |

## Context and Problem Statement

DDR-05 introduced the `/ai` command for natural language to kubectl
translation, but left the LLM provider and architecture open-ended.
Users want AI-powered command generation with:

1. **Privacy:** No cluster commands sent to external APIs
2. **Cost:** No per-request pricing or rate limits
3. **Speed:** Instant responses (<500ms ideal, <2s acceptable)
4. **Reliability:** High accuracy for kubectl syntax (>90% correct)
5. **Portability:** Works offline without internet dependency

**Key Questions:**
- Local inference vs cloud APIs?
- Which model architecture (size, specialization, training)?
- How to optimize for speed (quantization, hardware utilization)?
- How to maximize reliability (prompt engineering vs fine-tuning)?
- How to integrate with existing command bar UX (DDR-05)?

## Prior Work and Ecosystem Analysis

The natural language to kubectl space is mature, with several
production-grade tools validating the architectural approach:

### Reference Implementations

**Google kubectl-ai** (GKE team, open-source):
- Translates natural language to kubectl commands
- Native support for Ollama and llama.cpp (local models)
- Safety-first: mandatory user confirmation before execution
- Context-aware: sends resource type, namespace, selection to LLM
- Example: `kubectl-ai --llm-provider ollama --model gemma3:12b`

**k8sgpt** (CNCF Sandbox project):
- Kubernetes SRE assistant for diagnostics and troubleshooting
- Supports multiple backends: OpenAI, Anthropic, Ollama
- Shows that local models are production-viable

**Key Insight:** Major projects (Google, CNCF) officially support
Ollama for local inference. This validates Ollama as a stable,
enterprise-ready platform for local LLM serving.

### Architecture Pattern

All successful tools share a common pattern:

1. **User input** (natural language)
2. **Prompt engineering** (system prompt + context)
3. **LLM inference** (generate kubectl command)
4. **Preview + confirmation** (safety gate)
5. **Execution** (subprocess or client-go)

**Critical safety principle:** LLM suggests, human approves. Even
with 99% accuracy, the potential damage from a single bad command
(e.g., `kubectl delete deployment --all -n production`) is too high.
The confirmation step is non-negotiable.

## Design

### Decision: Local-First Architecture

**Use local inference via Ollama with quantized small language models
(SLMs) instead of cloud APIs.**

**Rationale:**

**Privacy:** Kubernetes commands often contain sensitive cluster
information (namespace names, deployment names, internal conventions).
Sending these to external APIs creates data leakage risk. Local
inference keeps all data on user's machine.

**Cost:** Cloud APIs charge per token (~$0.001-0.01 per request).
For heavy users (50+ commands/day), this adds up to $10-50/month.
Local models are free after initial download (~2-7 GB one-time).

**Speed:** Cloud APIs have network latency (50-500ms) + inference
time (200-2000ms) = 250-2500ms total. Local inference eliminates
network latency, achieving 100-800ms on CPU, 50-300ms on GPU.

**Reliability:** Both local and cloud models require prompt
engineering and validation. Cloud models are larger but not
necessarily better for this narrow domain. Domain-specific
fine-tuning (optional) works only with local models.

**Portability:** Works offline, on airplanes, in secure environments.
No internet dependency or API key management.

### Model Selection Strategy

The optimal model balances three constraints:

1. **Small parameter count** (3-7B) for fast CPU inference
2. **Code-focused training** for syntax accuracy
3. **High-quality quantization** (Q4_K_M minimum) for reliability

**Primary Recommendation: Microsoft Phi-3-mini (3.8B)**

- **Size:** 3.8B parameters
- **Quantization:** Q4_K_M GGUF (~2.3 GB, ~5.8 GB RAM)
- **Training:** High-quality "textbook" data, reasoning-optimized
- **Speed:** Excellent (smallest viable model)
- **Reliability:** Outperforms larger models on coding/reasoning tasks
- **Availability:** Pre-packaged in Ollama (`ollama run phi3`)

**Secondary Recommendation: Code Llama 7B Instruct**

- **Size:** 7B parameters
- **Quantization:** Q4_K_M GGUF (~4.1 GB, ~6.6 GB RAM)
- **Training:** Extensive code corpus, instruction-tuned
- **Speed:** Good (2x slower than Phi-3, still acceptable)
- **Reliability:** Superior syntactic fluency for structured languages
- **Proof:** LLM Compiler model (Code Llama fine-tuned for compiler
  IR/assembly) demonstrates strong capability for formal grammars
- **Availability:** Ollama (`ollama run codellama:7b-instruct`)

**Tertiary Recommendation: IBM Granite Code 3B**

- **Size:** 3B parameters (Granite-4.0-H-Micro variant)
- **Quantization:** Q4_K_M GGUF (~2.0 GB, ~5.5 GB RAM)
- **Architecture:** Hybrid Mamba-Transformer (efficient long context)
- **Training:** Enterprise-focused, tool-use optimized
- **Speed:** Excellent (comparable to Phi-3)
- **Reliability:** Validated in kubectl-ai PoC for this exact use case
- **Availability:** Hugging Face Hub + Ollama import

### Quantization Strategy

**Use GGUF format with Q4_K_M quantization as default.**

**GGUF (GGML Universal Format):**
- De facto standard for CPU inference (llama.cpp ecosystem)
- Single-file distribution (model + metadata)
- Optimized for fast loading and memory efficiency
- Wide hardware support (CPU, GPU, Apple Silicon)

**Quantization Levels:**

| Level  | Bits | Quality     | Use Case                          |
|--------|------|-------------|-----------------------------------|
| Q8_0   | 8    | Excellent   | Baseline (no compression)         |
| Q6_K   | 6    | Very Good   | High quality, still efficient     |
| Q5_K_M | 5    | Good        | Balanced (recommended alternative)|
| Q4_K_M | 4    | Good        | Recommended (speed/quality sweet) |
| Q3_K_L | 3    | Moderate    | Aggressive (use with caution)     |
| Q2_K   | 2    | Poor        | Too degraded for this task        |

**Recommended:** Q4_K_M provides ~75% size reduction with minimal
quality loss. For kubectl command generation (high syntactic
precision required), this is the optimal trade-off.

**Advanced:** Q5_K_M if quality issues observed, Q3_K_L only for
very resource-constrained environments (test thoroughly).

**Key Principle:** Better to use a smaller base model (3B) at higher
quantization (Q5/Q4) than a larger model (13B) at aggressive
quantization (Q3/Q2). Smaller models preserve their learned patterns
better under moderate compression.

### Inference Stack: Ollama + llama.cpp

**Use Ollama for deployment, backed by llama.cpp for inference.**

**Ollama:**
- Bundles model management, serving, and API into single tool
- One-command setup: `ollama run phi3`
- Automatic model download and caching
- OpenAI-compatible HTTP API (http://localhost:11434/api/chat)
- Zero configuration required for basic use

**llama.cpp (under the hood):**
- High-performance C++ inference engine
- Optimized for GGUF models on CPU and GPU
- Automatic hardware detection (AVX2, Metal, CUDA, ROCm)
- Hybrid execution: offload N layers to GPU with `-ngl` flag
- Can be used directly for advanced tuning (optional)

**Hardware Acceleration:**

Modern laptops have GPUs (integrated or discrete). Even modest GPUs
significantly accelerate inference:

- **CPU-only:** 5-20 tokens/second (acceptable)
- **Hybrid (GPU layers):** 20-60 tokens/second (excellent)
- **GPU-only:** 60-150 tokens/second (overkill for this task)

Ollama automatically uses available GPU. Users with Apple Silicon
(M1/M2/M3) or NVIDIA GPUs get 2-5x speedup with zero config.

### Integration with k1 Architecture

**Connection to DDR-05 and DDR-08:**

The `/ai` command flow (from DDR-05):

```
User types: /ai delete all failing pods in namespace foo
  ↓
CommandBar: Enters StateLLMPreview
  ↓
LLM Client: Sends prompt to Ollama API
  ↓
Prompt Engineering: Wraps user input in system prompt:
  "You are a Kubernetes expert. Generate a single kubectl command.
   Output only the command, no explanation.
   Context: resource=pods, namespace=foo, selection=nginx-abc123
   User request: delete all failing pods in namespace foo"
  ↓
Ollama: Runs inference with phi3:Q4_K_M (~300-800ms)
  ↓
LLM Client: Receives generated command:
  "kubectl delete pods --field-selector=status.phase!=Running -n foo"
  ↓
CommandBar: Displays preview in StateLLMPreview:
  ┌────────────────────────────────────────────────────┐
  │ Generated Command:                                 │
  │ kubectl delete pods --field-selector=...           │
  │                                                    │
  │ [Enter] Execute  [e] Edit  [ESC] Cancel           │
  └────────────────────────────────────────────────────┘
  ↓
User: Presses Enter to confirm
  ↓
Executor: Runs kubectl via subprocess (DDR-08 strategy)
  ↓
Result: "✓ Deleted 3 pods"
```

**Cache integration (DDR-05):**

```go
// ~/.config/k1/llm_cache.json
{
  "delete all failing pods": {
    "command": "kubectl delete pods --field-selector=...",
    "timestamp": "2025-10-06T10:23:15Z",
    "uses": 3
  }
}
```

Cache key: normalized prompt (lowercase, trimmed). Cache hit shows
⚡ indicator and skips LLM call entirely (~10ms vs ~500ms).

**Context awareness:**

The system prompt includes current state from DDR-05's CommandContext:

- **Resource type:** "pods", "deployments", etc.
- **Namespace:** Current namespace filter
- **Selection:** Selected resource name (if any)

This context dramatically improves LLM accuracy. Instead of:

```
User: "scale to 5"
LLM: (confused, generates invalid command)
```

The prompt becomes:

```
System: resource=deployments, namespace=prod, selected=nginx-deployment
User: "scale to 5"
LLM: kubectl scale deployment nginx-deployment -n prod --replicas 5
```

### Prompt Engineering

**The quality of the system prompt is critical for reliability.**

**Template structure:**

```
You are an expert Kubernetes administrator. Your task is to translate
the user's request into a single, valid kubectl command.

RULES:
- Output ONLY the kubectl command, nothing else
- No explanations, apologies, or conversational text
- Use proper kubectl syntax and flags
- Prefer --namespace over -n for clarity
- Use --field-selector or --label-selector when appropriate
- Do not use deprecated flags

CONTEXT:
- Resource type: {resource_type}
- Namespace: {namespace}
- Selected resource: {selected_name}

USER REQUEST:
{user_input}

KUBECTL COMMAND:
```

**Key principles:**

1. **Clear instructions:** "Output ONLY the command" prevents
   conversational output
2. **Explicit constraints:** List forbidden patterns (deprecated
   flags, dangerous operations)
3. **Context injection:** Provide resource type, namespace, selection
4. **Output format:** Prime the model with "KUBECTL COMMAND:" prefix

**Iteration strategy:**

The prompt will be refined based on empirical testing. Version it:

```go
const PromptVersion = "v1.0.0"

// Store version in cache for invalidation when prompt changes
```

### Benchmarking Strategy

**Create a domain-specific benchmark to measure reliability.**

Standard coding benchmarks (HumanEval, SWE-Bench) do not correlate
with kubectl command generation. The task is not algorithmic
problem-solving but precise syntactic recall and formatting.

**Custom benchmark structure:**

```json
[
  {
    "prompt": "list all pods in namespace kube-system",
    "expected": "kubectl get pods -n kube-system",
    "category": "simple"
  },
  {
    "prompt": "scale deployment nginx to 5 replicas in prod namespace",
    "expected": "kubectl scale deployment nginx -n prod --replicas 5",
    "category": "moderate"
  },
  {
    "prompt": "delete all pods not running in foo namespace",
    "expected": "kubectl delete pods --field-selector=status.phase!=Running -n foo",
    "category": "complex"
  }
]
```

**Benchmark size:** 50-100 examples covering:
- Simple queries (get, describe)
- Moderate operations (scale, delete, restart)
- Complex operations (field selectors, multiple resources)
- Edge cases (dry-run, output formats, force flags)

**Scoring:**
- **Exact match:** Command is byte-for-byte identical (strict)
- **Semantic match:** Command achieves same result (relaxed)
- **Syntax error:** Command is invalid kubectl (failure)

**Usage:**

```bash
# Test model against benchmark
go run cmd/benchmark-llm/main.go \
  --model phi3 \
  --quantization Q4_K_M \
  --benchmark data/kubectl_benchmark.json

# Output:
# Model: phi3:Q4_K_M
# Pass rate: 87/100 (87%)
# Avg latency: 420ms
# Categories:
#   simple: 48/50 (96%)
#   moderate: 28/35 (80%)
#   complex: 11/15 (73%)
```

This data-driven approach identifies which model provides best
reliability for this specific task.

### Fine-Tuning Path (Optional)

**For users requiring >95% reliability, domain-specific fine-tuning
is the ultimate solution.**

Fine-tuning adapts a general-purpose model into a kubectl specialist.
This is a significant investment (data collection, training time) but
yields dramatic reliability improvements.

**QLoRA (Quantized Low-Rank Adaptation):**

Modern PEFT (Parameter-Efficient Fine-Tuning) makes fine-tuning
accessible on consumer hardware:

1. Load base model in 4-bit quantization (~3 GB VRAM)
2. Add small adapter layers (~1% of total parameters)
3. Train only adapters, freeze base model
4. Result: 7B model fine-tunable on 24 GB GPU

**Dataset requirements:**

- **Size:** 1000-5000 (natural_language, kubectl_command) pairs
- **Generation:** Use GPT-4 to synthesize diverse examples
- **Validation:** Manual review for correctness
- **Format:** JSONL with input/output pairs

**Training:**

```python
# Using Hugging Face ecosystem (transformers, peft, trl)
from transformers import AutoModelForCausalLM, BitsAndBytesConfig
from peft import LoraConfig, get_peft_model
from trl import SFTTrainer

# Load base model in 4-bit
model = AutoModelForCausalLM.from_pretrained(
    "codellama/CodeLlama-7b-hf",
    quantization_config=BitsAndBytesConfig(load_in_4bit=True)
)

# Add LoRA adapters
lora_config = LoraConfig(r=16, lora_alpha=32, target_modules=["q_proj", "v_proj"])
model = get_peft_model(model, lora_config)

# Train
trainer = SFTTrainer(model=model, train_dataset=dataset, ...)
trainer.train()

# Export to GGUF for Ollama
model.save_pretrained("kubectl-specialist-7b")
# Convert to GGUF with llama.cpp tools
# Import to Ollama: ollama create kubectl-specialist -f Modelfile
```

**Expected results:**

- Base model: 75-85% accuracy
- Fine-tuned model: 90-98% accuracy
- Latency: Unchanged (same architecture)

**Trade-off:** Significant upfront cost (days of work) for 10-20%
reliability improvement. Only justified for power users or commercial
deployment.

## Decision Summary

**Architecture: Local-first with Ollama + quantized SLMs**

1. **Inference stack:** Ollama (frontend) + llama.cpp (backend)
2. **Primary model:** Phi-3-mini Q4_K_M (3.8B, ~2.3 GB)
3. **Secondary model:** Code Llama 7B Instruct Q4_K_M (~4.1 GB)
4. **Tertiary model:** Granite Code 3B Q4_K_M (~2.0 GB)
5. **Quantization:** Q4_K_M (4-bit) as default, Q5_K_M for quality-sensitive
6. **Prompt engineering:** Versioned system prompts with context injection
7. **Safety:** Mandatory preview + confirmation (DDR-05 StateLLMPreview)
8. **Execution:** kubectl subprocess (DDR-08 strategy)
9. **Caching:** Normalized prompt keys, 30-day TTL (DDR-05 design)
10. **Benchmarking:** Custom kubectl domain benchmark (50-100 cases)
11. **Fine-tuning:** Optional QLoRA path for 95%+ reliability

**Deployment:**

```bash
# One-time setup
ollama pull phi3

# Test inference
curl http://localhost:11434/api/chat -d '{
  "model": "phi3",
  "messages": [{"role": "user", "content": "list all pods"}]
}'

# k1 integration: HTTP client talks to localhost:11434
```

## Consequences

### Positive

1. **Privacy:** No data leaves user's machine. Cluster information
   stays local.
2. **Cost:** Zero per-request cost after initial model download
   (2-7 GB one-time).
3. **Speed:** 300-800ms CPU inference, 100-300ms with GPU. No network
   latency.
4. **Offline:** Works without internet. Useful for secure
   environments, travel.
5. **Control:** User owns the model. No API key management, rate
   limits, or service outages.
6. **Proven architecture:** kubectl-ai and k8sgpt validate Ollama as
   production-ready.
7. **Easy setup:** `ollama pull phi3` is the entire installation.
8. **Hardware-agnostic:** Works on CPU. GPU acceleration automatic if
   available.
9. **Gradual improvement:** Can upgrade from base → fine-tuned model
   without architecture change.
10. **Transparent:** User sees exact command before execution. LLM is
    a suggester, not executor.

### Negative

1. **Disk space:** Models are 2-7 GB per variant. Users need ~10 GB
   for experimentation.
2. **Memory:** 4-8 GB RAM required during inference. May compete with
   other apps.
3. **First-run latency:** Model download takes 2-10 minutes on slow
   connections.
4. **Accuracy ceiling:** 3-7B models less accurate than GPT-4 or
   Claude (75-85% vs 90-95%).
5. **Maintenance burden:** Ollama is an external dependency (separate
   installation).
6. **Hardware variance:** Performance depends on CPU/GPU. Older
   hardware may be slow.
7. **No streaming:** Current design waits for full completion. Future
   enhancement.
8. **Prompt engineering required:** Reliability depends on prompt
   quality. Needs iteration.

### Mitigations

1. **Disk space:** Document storage requirements. Provide model
   cleanup commands.
2. **Memory:** Use smaller models (Phi-3-mini at 3.8B, not 13B
   variants).
3. **First-run latency:** Show progress bar during download. Cache
   models.
4. **Accuracy ceiling:** Domain-specific benchmark guides model
   selection. Fine-tuning path exists.
5. **Maintenance burden:** Check Ollama availability at startup. Show
   clear error with install link.
6. **Hardware variance:** Benchmark on target hardware. Recommend
   minimum specs.
7. **No streaming:** Preview + confirm UX works well without
   streaming. Acceptable trade-off.
8. **Prompt engineering:** Version prompts. Iterate based on benchmark
   results.

## Implementation Notes

### Phased Rollout

**Phase 1: Basic inference (MVP)**
- Ollama HTTP client
- Phi-3-mini Q4_K_M only
- Simple system prompt (v1.0.0)
- Preview + confirm UX (DDR-05)
- Cache with 30-day TTL (DDR-05)
- ~2-3 days development

**Phase 2: Multi-model support**
- Model selection in config: phi3, codellama, granite
- Benchmark harness (50-100 test cases)
- Compare models, recommend winner
- ~3-5 days development

**Phase 3: Advanced features**
- Context-aware prompts (resource type, namespace, selection)
- Prompt versioning and cache invalidation
- GPU detection and layer offloading hints
- ~2-3 days development

**Phase 4: Fine-tuning (optional)**
- Dataset generation (GPT-4 synthetic + manual curation)
- QLoRA training scripts (Hugging Face + Ollama export)
- Benchmark validation (target: 95%+ accuracy)
- ~1-3 weeks (mostly data work)

### Configuration

```yaml
# ~/.config/k1/config.yaml
llm:
  enabled: true
  provider: ollama                  # Future: openai, anthropic
  model: phi3                       # or codellama, granite
  url: http://localhost:11434       # Ollama API endpoint
  timeout: 10s                      # Max inference time
  prompt_version: "v1.0.0"          # Cache invalidation key
  cache:
    enabled: true
    ttl: 720h                       # 30 days
    path: ~/.config/k1/llm_cache.json
```

### Ollama Availability Check

```go
// internal/llm/client.go

func CheckOllama(url string) error {
    resp, err := http.Get(url + "/api/tags")
    if err != nil {
        return fmt.Errorf(
            "Ollama not available at %s.\n"+
            "Install: https://ollama.ai/download\n"+
            "Start: ollama serve",
            url)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return fmt.Errorf("Ollama API returned %d", resp.StatusCode)
    }

    return nil
}
```

Show error at startup if Ollama unavailable. Disable `/ai` command
gracefully.

### Model Pull Helper

```go
// internal/llm/setup.go

func EnsureModel(model string) error {
    // Check if model exists locally
    resp, _ := http.Post("http://localhost:11434/api/show",
        "application/json",
        strings.NewReader(fmt.Sprintf(`{"name":"%s"}`, model)))

    if resp != nil && resp.StatusCode == 200 {
        return nil // Model exists
    }

    // Model not found, offer to pull
    fmt.Printf("Model %s not found. Pull now? [Y/n] ", model)
    // ... handle user input ...

    // Pull with progress bar
    fmt.Printf("Pulling %s... (this may take 2-10 minutes)\n", model)
    cmd := exec.Command("ollama", "pull", model)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    return cmd.Run()
}
```

Run on first `/ai` command invocation.

## Alternative Approaches Considered

### Alternative 1: Cloud API (OpenAI, Anthropic)

**Approach:** Use GPT-4 or Claude via API for command generation.

**Pros:**
- Highest accuracy (90-95% on benchmarks)
- Zero local installation
- Minimal resource usage (just HTTP client)
- Always up-to-date (models improve automatically)

**Cons:**
- **Privacy risk:** Cluster data sent to external servers
- **Cost:** $0.001-0.01 per request, $10-50/month for heavy users
- **Latency:** Network round-trip adds 50-500ms
- **Internet dependency:** Requires active connection
- **Rate limits:** May hit usage caps during bursts

**Why rejected:** Privacy is paramount for Kubernetes tooling. Users
manage production clusters with sensitive data. Sending commands like
"scale deployment payment-processor-prod to 50" to external APIs is
a non-starter for most enterprises. Cost and latency are secondary
concerns.

### Alternative 2: Hybrid (Local first, cloud fallback)

**Approach:** Try local model first, fall back to cloud API if
confidence is low.

**Pros:**
- Best of both worlds (privacy + accuracy)
- Graceful degradation
- User choice

**Cons:**
- Complex architecture (two inference paths)
- How to measure confidence? (non-trivial)
- Still requires API key and internet
- Cost unpredictable (hard to estimate fallback rate)

**Why rejected:** Adds significant complexity for marginal benefit.
If local model is unreliable enough to need fallback, it shouldn't
be the default. Better to invest in improving local model (prompt
engineering, fine-tuning) than building complex hybrid system.

### Alternative 3: Embedded model (ONNX, TensorFlow Lite)

**Approach:** Embed quantized model directly in k1 binary using
ONNX Runtime or TF Lite.

**Pros:**
- Zero external dependencies
- Single binary distribution
- Fast startup (no model loading)

**Cons:**
- **Binary size:** Model adds 2-7 GB to binary (unacceptable)
- **Inflexible:** Users can't swap models or update easily
- **Maintenance:** Need to recompile k1 for every model update
- **Runtime complexity:** Need to embed ONNX Runtime or TF Lite
  (C++ dependencies, platform-specific builds)

**Why rejected:** Kubernetes tools should be lightweight (~50 MB
max). A 5 GB binary is a non-starter. Ollama provides better UX with
separate model management.

### Alternative 4: Server-side model (self-hosted)

**Approach:** User deploys LLM server (vLLM, TGI) in cluster or
cloud, k1 connects to it.

**Pros:**
- More powerful hardware (cluster GPUs)
- Shared model across team
- Centralized updates

**Cons:**
- Complex deployment (Kubernetes manifests, GPUs, ingress)
- Operational burden (monitoring, scaling, costs)
- Latency if cluster is remote
- Overkill for single-user tool

**Why rejected:** k1 is a single-user CLI tool, not a team product.
Requiring cluster deployment would drastically reduce adoption.
Ollama's local model is simpler and sufficient for this use case.

## Future Enhancements

1. **Multi-model support:** Allow users to choose model in config or
   per-command.
2. **Streaming responses:** Show tokens as they generate (better
   perceived speed).
3. **Explain mode:** `/ai explain <resource>` generates natural
   language explanations.
4. **Learning from edits:** Track when user modifies generated
   command, improve prompts.
5. **Confidence scoring:** Show LLM's confidence in generated command.
6. **Multi-step commands:** Break complex tasks into multiple kubectl
   commands.
7. **YAML generation:** `/ai create deployment with 3 replicas`
   generates YAML instead of kubectl apply.
8. **Custom models:** Support user-provided fine-tuned models.
9. **Cloud optional:** Add OpenAI/Anthropic backend for users who
   prefer it (opt-in).

## References

- DDR-05: Command-Enhanced List Browser (command bar, `/ai` command,
  cache design)
- DDR-08: Pragmatic Command Implementation (kubectl subprocess
  strategy)
- [Google kubectl-ai](https://github.com/GoogleCloudPlatform/kubectl-ai) -
  Validates Ollama for production use
- [k8sgpt](https://github.com/k8sgpt-ai/k8sgpt) - CNCF project with
  Ollama support
- [Ollama documentation](https://ollama.ai/docs)
- [llama.cpp](https://github.com/ggerganov/llama.cpp) - Inference
  engine
- [QLoRA paper](https://arxiv.org/abs/2305.14314) - Efficient
  fine-tuning
- [Phi-3 technical report](https://arxiv.org/abs/2404.14219) -
  Microsoft's SLM
- [Code Llama paper](https://arxiv.org/abs/2308.12950) - Meta's code
  model
