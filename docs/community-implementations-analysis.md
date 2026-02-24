# Community Attractor Implementations: Improvements & Ideas for Kilroy

> Comparative analysis of 5 community Attractor implementations vs. Kilroy.
> Generated 2026-02-12 from https://factory.strongdm.ai/products/attractor

---

## Repos Analyzed

| # | Repo | Author | Language | Lines (approx) |
|---|------|--------|----------|-----------------|
| 1 | [smartcomputer-ai/forge](https://github.com/smartcomputer-ai/forge) | Luke Buehler | Rust | ~15K+ |
| 2 | [brynary/attractor](https://github.com/brynary/attractor) | Bryan Helmkamp | TypeScript/Bun | ~8K+ |
| 3 | [anishkny/attractor](https://github.com/anishkny/attractor) | Anish Karandikar | Python | ~1,650 |
| 4 | [bborn/attractor-ruby](https://github.com/bborn/attractor-ruby) | Bruno Bornsztein | Ruby | ~4K+ |
| 5 | [samueljklee/attractor](https://github.com/samueljklee/attractor) | Samuel Lee | Python | ~4-5K |

---

## Executive Summary

Five community implementations collectively surface **12 high-value improvement areas** for Kilroy. The strongest signal comes from features that appear independently in 3+ implementations -- these represent community consensus on what the Attractor pattern needs beyond what Kilroy currently provides.

**Consensus features (3+ implementations):**
- HTTP server mode with SSE event streaming (all 5)
- Subagent spawning with depth control (4 of 5)
- Manager loop / nested pipeline handler (4 of 5)
- Model capability catalog with structured metadata (4 of 5)
- LLM client middleware chain (3 of 5)
- Simulation/test backend (3 of 5)
- Composable execution environment wrappers (3 of 5)
- Tool call hooks (2 of 5, but high impact)

**Where Kilroy already leads (unanimous):**
- Git-first execution model (no other implementation has this)
- Failure signature normalization + cycle breakers
- Triple resume sources
- Provider breadth (6+ vs 3-4)
- Stall watchdog
- English-to-DOT ingestion

---

## Improvements Ranked by Priority

### TIER 1: High Priority (strong community consensus, high impact)

#### 1. HTTP Server Mode with SSE Event Streaming
**Seen in:** All 5 implementations (Forge via CXDB query API, brynary/anishkny/samueljklee/attractor-ruby via REST+SSE)

**What:** Add `kilroy attractor serve` exposing REST endpoints:
- `POST /pipelines` -- submit DOT, start run, return ID
- `GET /pipelines/{id}` -- poll status
- `GET /pipelines/{id}/events` -- Server-Sent Events real-time stream
- `POST /pipelines/{id}/cancel` -- cancel a running pipeline
- `GET /pipelines/{id}/context` -- inspect current context
- `GET /pipelines/{id}/questions` -- pending human gates
- `POST /pipelines/{id}/questions/{qid}/answer` -- answer human gate

**Why:** Kilroy is CLI-only. An HTTP server enables:
- Web dashboards for pipeline visualization
- CI/CD integration (trigger via webhook, poll for completion)
- Remote human-in-the-loop approval workflows
- Multi-pipeline orchestration from a single endpoint
- Browser-based monitoring without terminal access

**How:** Kilroy already writes `progress.ndjson` with structured events -- the perfect data source for SSE. Go's `net/http` makes this straightforward. Create `internal/server/` wrapping the existing engine.

**Effort:** Medium

---

#### 2. Simulation Backend for Testing
**Seen in:** attractor-ruby, brynary (stubs), samueljklee (mock patterns)

**What:** A first-class `SimulationBackend` that returns canned/pattern-matched/dynamic responses without calling real LLMs:
```go
sim := NewSimulationBackend(map[string]interface{}{
    "impl_code":    Response{Status: "success", Notes: "implemented"},
    "verify_build": Response{Status: "fail", Reason: "compilation error"},
})
```
Support node-specific responses, regex pattern matching, and callback-based dynamic responses.

**Why:** Pipeline integration tests currently need mock LLM behavior. A simulation backend enables:
- Fast, cheap, deterministic test scenarios (no API calls)
- Testing edge selection, retry policies, failure paths in isolation
- CI pipeline validation without LLM credentials

**How:** Implement a backend satisfying the same interface as real backends, accepting a map of node_id/pattern -> response.

**Effort:** Small

---

#### 3. Subagent Spawning with Depth Control
**Seen in:** brynary, attractor-ruby, samueljklee, Forge

**What:** Add `spawn_agent`, `send_input`, `wait`, and `close_agent` tools to the coding agent's tool registry. Child agents share the filesystem but have isolated conversation history. Depth-limited (configurable `max_subagent_depth`, default 3).

**Why:** Complex coding tasks benefit from decomposition. A parent agent can spawn children for independent subtasks ("implement the API" + "write the tests" + "update the docs") within a single pipeline node, without requiring the graph to encode that parallelism statically.

**How:** In `internal/agent/`, add subagent tool definitions. Track active subagents with depth counters. Each subagent gets its own session with decremented depth limit.

**Effort:** Medium

---

#### 4. Manager Loop / Nested Pipeline Handler
**Seen in:** Forge (`stack.manager_loop`), brynary (polling-based), attractor-ruby (`doubleoctagon`), anishkny (`ManagerLoopHandler`)

**What:** A node type (e.g., `doubleoctagon` shape) that runs a nested/child pipeline in a loop with iteration count and completion conditions:
```dot
supervisor [shape=doubleoctagon, pipeline="child.dot", max_iterations="10", completion_condition="done=true"]
```

**Why:** Enables hierarchical orchestration patterns:
- Iterative refinement (generate -> test -> fix -> test until passing)
- A "code review" pipeline that supervises a "code generation" pipeline
- Pipelines that spawn sub-tasks dynamically
- Polling-based observation with intervention/steering

Kilroy currently handles loops via loop restart policies and edge routing, but a first-class nested pipeline node would be more expressive and self-contained.

**How:** Add a `ManagerLoopHandler` that parses a nested DOT graph from a node attribute, creates a child engine, and iterates until a completion condition is met.

**Effort:** Medium-Large

---

#### 5. Tool Call Hooks (Pre/Post)
**Seen in:** Forge, brynary

**What:** Configurable shell script hooks that run before/after each agent tool call, with structured JSON context on stdin:
```dot
graph [tool_hooks.pre="scripts/security-scan.sh", tool_hooks.post="scripts/audit-log.sh"]
```
Pre-hook exit code 0 = continue, non-zero = skip/veto the tool call.

**Why:** Enables security scanning, custom logging, compliance checks, tool call vetoing, and custom post-processing without modifying the agent loop. Critical for enterprise/factory use cases.

**How:** Add `tool_hooks.pre` and `tool_hooks.post` as DOT attributes (node or graph level). Execute via `os/exec` with JSON payload containing run_id, node_id, tool_name, arguments.

**Effort:** Small-Medium

---

### TIER 2: Medium Priority (multiple implementations, solid value)

#### 6. Model Capability Catalog
**Seen in:** Forge (`catalog_models.json`), brynary, attractor-ruby, samueljklee

**What:** Extend the model catalog with structured metadata per model:
```json
{
  "claude-opus-4-6": {
    "context_window": 200000,
    "max_output_tokens": 32000,
    "supports_tools": true,
    "supports_vision": true,
    "supports_thinking": true
  }
}
```

**Why:** Enables:
- Automatic provider routing from model ID alone
- Capability checking before sending unsupported features (vision, tools, thinking)
- Context window-aware truncation and token budgeting
- Better error messages ("model X doesn't support tool use")

**How:** Extend `internal/modelmeta/` with structured capability fields. Use in codergen routing to validate model choices.

**Effort:** Small

---

#### 7. LLM Client Middleware Chain
**Seen in:** Forge (onion model), brynary, attractor-ruby

**What:** Composable middleware pipeline on LLM requests:
```go
type Middleware interface {
    HandleComplete(req *Request, next func(*Request) (*Response, error)) (*Response, error)
    HandleStream(req *Request, next func(*Request) (Stream, error)) (Stream, error)
}
```

**Why:** Enables cross-cutting concerns without modifying core code:
- Request/response logging and auditing
- Cost tracking (estimate tokens before sending)
- Automatic temperature/token overrides per environment
- Prompt injection detection
- Request/response caching layer
- `Retry-After` header honoring (seen in samueljklee, Forge)

**How:** Define middleware interface, chain in the client constructor. Kilroy's current retry is hardcoded -- this would make it pluggable.

**Effort:** Medium

---

#### 8. Composable Execution Environment Wrappers
**Seen in:** brynary (EnvFilter/LoggingEnv/ReadonlyEnv), samueljklee (env filtering), attractor-ruby

**What:** Three decorator layers around the execution environment:
- **EnvFilter**: Strip sensitive environment variables (`*_KEY`, `*_SECRET`, `*_TOKEN`, `*_PASSWORD`) from child processes
- **LoggingEnv**: Audit trail of every file operation and command
- **ReadonlyEnv**: Prevent writes for safe evaluation/review stages

**Why:**
- `EnvFilter` prevents accidental secret leakage to LLM-controlled shell commands (security)
- `LoggingEnv` creates audit trails for debugging and compliance
- `ReadonlyEnv` enables safe review nodes where the agent should analyze but not modify

**How:** Implement as Go interfaces wrapping `ExecutionEnvironment` with the decorator pattern.

**Effort:** Small-Medium

---

#### 9. Pluggable Validation Rules
**Seen in:** Forge (13 rules + `LintRule` trait), anishkny (7 rules + `extra_rules`), brynary (13 rules)

**What:** Refactor validation to accept a slice of rule interfaces:
```go
type LintRule interface {
    Name() string
    Apply(g *model.Graph) []Diagnostic
}
```
Allow external consumers to inject custom validation rules.

**Why:** Kilroy has hardcoded lint rules. Custom rules (organization naming conventions, provider-specific constraints, DOT complexity limits) currently require modifying core code. The `ingest` skill could also use custom rules.

**How:** Each existing rule becomes a `LintRule` implementation. `validate.Graph()` accepts `...LintRule` for extras.

**Effort:** Small

---

#### 10. Typed Event System
**Seen in:** Forge (full typed hierarchy), brynary (18 typed events), anishkny (15 typed events), samueljklee (13 event kinds)

**What:** Replace `map[string]any`-based progress events with typed event structs:
```go
type Event interface {
    Type() EventType
    Timestamp() time.Time
}
type StageStarted struct {
    NodeID  string
    Attempt int
    Max     int
}
```

**Why:** Kilroy's `appendProgress(map[string]any{...})` is flexible but untyped. No compile-time enforcement that `"stage_attempt_start"` includes the right fields. Typed events catch structural errors at compile time.

**How:** Define types in `internal/attractor/engine/events.go`. The `progress.ndjson` serialization stays the same but Go code gains type safety.

**Effort:** Medium

---

### TIER 3: Lower Priority (good ideas, less urgent)

#### 11. Parallel Join Policies (Beyond Wait-All)
**Seen in:** Forge (all_success/any_success/quorum/ignore), brynary (FIRST_SUCCESS/K_OF_N/WAIT_ALL)

**What:** Add `join_policy` attribute to parallel nodes:
- `wait_all` (current behavior)
- `first_success` -- proceed when any branch succeeds, cancel others
- `quorum(n)` -- proceed when N branches succeed
- `ignore` -- always proceed regardless of branch outcomes

**Why:** Some workflows only need one successful branch (trying multiple approaches). Others need a quorum (2 of 3 reviewers approve).

**Effort:** Medium

---

#### 12. Named Retry Presets
**Seen in:** brynary (5 named presets: none/standard/aggressive/linear/patient)

**What:** Named retry policies with curated parameters:
```dot
verify_build [retry_policy="aggressive"]  // 5 retries, 500ms initial, 1.5x factor
review_code  [retry_policy="patient"]     // 3 retries, 2s initial, 2x factor
```

**Why:** More intuitive than raw numeric configuration. Users say "patient" instead of computing backoff parameters.

**Effort:** Small

---

#### 13. Graph Composition / Merge Transform
**Seen in:** brynary (`GraphMergeTransform`)

**What:** Import/merge sub-graphs from separate DOT files with namespace-prefixed node IDs:
```dot
graph [import="review-subgraph.dot"]
```

**Why:** Modular pipeline design. Reusable sub-graphs (standard "code-review", "test-and-deploy" fragments) shared across projects.

**Effort:** Medium

---

#### 14. Typed DOT Attributes
**Seen in:** Forge (String/Integer/Float/Boolean/Duration `AttrValue` enum)

**What:** Parse DOT attribute values into typed variants instead of treating everything as strings.

**Why:** Eliminates scattered `strconv.Atoi` / `time.ParseDuration` calls. Duration values like `"30s"` get parsed once. Enables better validation at parse time.

**Effort:** Medium

---

#### 15. Steering Injection for Running Agent Sessions
**Seen in:** brynary, attractor-ruby, samueljklee

**What:** `session.Steer("Focus on error handling next")` to inject guidance messages between tool rounds.

**Why:** For long-running codergen nodes, operators may want to redirect the agent mid-execution without killing it. Pairs naturally with the HTTP server mode.

**Effort:** Small-Medium

---

#### 16. Recording & Queue Interviewers for Testing
**Seen in:** Forge (5 interviewer impls), brynary (6 impls), attractor-ruby (5 impls)

**What:**
- `QueueInterviewer` -- pre-loaded answers for deterministic testing
- `RecordingInterviewer` -- decorator that captures all Q&A for replay

**Why:** Makes `wait.human` nodes testable in CI without human interaction. Record once interactively, replay in CI.

**Effort:** Small

---

#### 17. Pipeline-as-Library API
**Seen in:** anishkny (Python API), attractor-ruby (gem), brynary (TS packages), samueljklee (Python SDK)

**What:** Expose a stable Go API for programmatic pipeline execution (e.g., `pkg/attractor` package).

**Why:** Kilroy's engine is in `internal/` -- inaccessible to external Go consumers. A public API enables embedding in larger Go applications, custom CLIs, and integration testing without subprocess spawning.

**Effort:** Medium-Large

---

## Where Kilroy Already Wins (Unanimous)

These are areas where **no community implementation matches Kilroy**, confirming these are genuine architectural advantages:

| Capability | Kilroy | Community Implementations |
|---|---|---|
| **Git-first execution** | Branch + worktree + checkpoint commits per node | Zero git integration (all 5) |
| **Triple resume** | Logs dir + CXDB + git branch | Single checkpoint file (all 5) |
| **Failure signature normalization** | Hash-based dedup across restarts | Basic retry only (all 5) |
| **Deterministic failure cycle breaker** | Kills runs on repeated identical failures | None (all 5) |
| **Stuck-cycle breaker** | Max node visits per iteration | None (all 5) |
| **Stall watchdog** | Kills runs with no progress | None (all 5) |
| **Provider breadth** | 6+ (OpenAI, Anthropic, Google, Kimi, ZAI, Cerebras) | 3-4 providers (all 5) |
| **CLI + API dual backends** | Shell to `claude`/`codex` + direct API | API-only (all 5) |
| **English-to-DOT ingestion** | Skill-based natural language pipeline generation | None (all 5) |
| **Detached execution** | `--detach` + `setsid` + PID tracking | None or HTTP-only (all 5) |
| **CXDB observability** | Append-only event log with binary protocol | In-memory events only (all 5) |
| **Model stylesheet as file** | Separate `.css`-like file or inline | Inline only (all 5) |
| **Run configuration YAML** | Full `run.yaml` schema | CLI args only (all 5) |
| **Codergen failover routing** | Multi-provider failover chain | None (all 5) |

---

## Cross-Implementation Feature Matrix

Shows which repo implements each feature:

| Feature | Forge (Rust) | brynary (TS) | anishkny (Py) | attractor-ruby | samueljklee (Py) | Kilroy |
|---|:---:|:---:|:---:|:---:|:---:|:---:|
| HTTP Server + SSE | via CXDB | Y | Y | Y | Y | - |
| Subagent spawning | Y | Y | - | Y | Y | - |
| Manager loop handler | Y | Y | Y | Y | - | - |
| Model capability catalog | Y | Y | - | Y | Y | partial |
| LLM middleware chain | Y | Y | - | Y | - | - |
| Simulation backend | - | stubs | - | Y | - | - |
| Exec env wrappers | - | Y | - | - | Y | - |
| Tool call hooks | Y | Y | - | - | - | - |
| Pluggable validation | Y | Y | Y | Y | Y | hardcoded |
| Typed event system | Y | Y | Y | Y | Y | untyped |
| Parallel join policies | Y | Y | - | - | - | wait-all |
| Named retry presets | - | Y | - | - | - | - |
| Graph merge/compose | - | Y | - | - | - | - |
| Typed DOT attributes | Y | - | - | - | - | - |
| Steering injection | - | Y | - | Y | Y | - |
| Recording interviewer | Y | Y | - | Y | - | - |
| Structured LLM output | - | Y | - | - | - | - |
| Git-first execution | - | - | - | - | - | Y |
| Triple resume | - | - | - | - | - | Y |
| Failure cycle breakers | - | - | - | - | - | Y |
| Stall watchdog | - | - | - | - | - | Y |
| English-to-DOT | - | - | - | - | - | Y |
| CXDB integration | Y | - | - | - | - | Y |
| 6+ providers | - | - | - | - | - | Y |
| CLI + API backends | - | - | - | - | - | Y |

---

## Recommended Roadmap

**Phase 1 (Quick wins):**
1. Simulation backend for testing
2. Pluggable validation rules
3. Named retry presets
4. Recording + Queue interviewers

**Phase 2 (Core architecture):**
5. Typed event system
6. HTTP server mode with SSE
7. Model capability catalog
8. Environment variable filtering (security)

**Phase 3 (Advanced features):**
9. LLM client middleware chain
10. Tool call hooks (pre/post)
11. Subagent spawning with depth control
12. Parallel join policies

**Phase 4 (Composition & orchestration):**
13. Manager loop / nested pipeline handler
14. Graph composition / merge transform
15. Steering injection
16. Pipeline-as-library API
