# GlobalPermits Architecture Generator (gparchgen)

This Go CLI generates a comprehensive, sectioned technical architecture blueprint (per the spec) from a full product description for the "GlobalPermits API".

Features:
- Reads your product doc (Markdown or text)
- Outputs a structured architecture blueprint covering all required sections: High-Level Overview, C4 Container Diagram, Ingestion, Processing, Storage, API, Gateway, Caching, Infra, CI/CD, Monitoring
- Safe default template you can customize later

## Quick start

1) Paste your full GlobalPermits product doc into input/globalpermits_doc.md

2) Build the CLI

   go build -o bin/gparchgen ./cmd/gparchgen

3) Generate the architecture document

   ./bin/gparchgen -in input/globalpermits_doc.md -out output/GlobalPermits_Architecture.md

The output file will contain a complete architecture blueprint, with your source document embedded at the end for traceability.

## Project layout
- cmd/gparchgen: CLI entrypoint
- internal/generator: Template-based generator
- input/: Place your product doc here
- output/: Generated blueprint

## Customize the template
If you want to tailor the template, edit internal/generator/generator.go in DefaultTemplate(). You can also split the template into separate files if preferred.

## Requirements
- Go 1.22+

## Notes
- This generator scaffolds a high-quality first draft. You should iterate to add product-specific details and decisions as needed.

