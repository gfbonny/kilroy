# Solitaire Demo

This demo runs the fast Klondike solitaire pipeline in Attractor.

## Validate

```bash
./kilroy attractor validate --graph demo/solitaire/solitaire-fast.dot
```

## Run

```bash
./kilroy attractor run --skip-cli-headless-warning --graph demo/solitaire/solitaire-fast.dot --config demo/solitaire/run.yaml
```

## Notes

- Model/provider is set in the graph stylesheet: `openai` + `gpt-5.3-codex-spark`.
- If your default CXDB has registry conflicts, use isolated CXDB ports/container settings in a copied run config.
