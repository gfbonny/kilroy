# No Implicit Provider Failover Spec (2026-03-02)

## Source
User request on 2026-03-02:
- "We're going to remove the hardcoded defaults. No failover unless the user specifies it in the runfile."

## Requirements
1. Remove hardcoded provider failover defaults from Kilroy runtime behavior.
2. If `llm.providers.<provider>.failover` is omitted in run config, provider failover must not occur.
3. Failover is allowed only when explicitly configured in run config.
4. Keep explicit run-config failover behavior functional (routing + preflight).
5. Update tests and docs to match new explicit-only behavior.

## Non-Goals
- No changes to unrelated edge-routing fallback semantics.
- No provider API/CLI protocol behavior changes unrelated to failover policy.
