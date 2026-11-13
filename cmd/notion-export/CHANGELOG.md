# Changelog — notion-export

## Unreleased

- Nova CLI experimental, sucessora em Go da CLI Deno `notion-public-exporter`:
  export de página pública do Notion para Markdown/HTML com assets locais,
  política `ask/yes/no` por tipo, estimativa de tamanho, downloads validados e
  concorrentes, publicação em staging com `--force` validado e rollback, e
  `--from-json`; `--name`, progresso vivo em terminal, avisos de espera do
  Notion e tamanhos em KiB/MiB.

## [0.1.1] - 2026-11-13

### Corrigido

- **notion-export:** Release experimental notion-export 0.1.1: export a public Notion page (URL, UUID or page ID, no token) to Markdown and script-free HTML with local CSS and CSP, page.json and a versioned manifest; ask/yes/no download policy per asset type after 1-byte size probes; bounded concurrent downloads with Content-Type, length and stall validation; bounded retries for the unofficial API; staged publication that never replaces an existing export unless --force validates it belongs to the same page, with rollback; --from-json reprocessing.
