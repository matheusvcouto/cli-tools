# Validation — snapshot de 2026-09-26

**Baseline:** `go.mod` exige Go 1.27.1 e módulo `github.com/matheusvcouto/cli-tools/v2`. O sandbox local Linux amd64 contém Go 1.23.2. `GOTOOLCHAIN=local go test ./...` retornou: `go: go.mod requires go >= 1.27.1 (running go 1.23.2; GOTOOLCHAIN=local)`. Toolchain oficial não pôde ser obtido por bloqueio de rede/DNS. Não alteramos a baseline nem reintroduzimos o backend `root_legacy` para fabricar um PASS.

## Executado nesta árvore

- `gofmt -l` sobre **113 arquivos Go**: nenhum arquivo fora do formato; apenas evidência de sintaxe que a versão local do formatador compreende, **não** type-check Go 1.27.1.
- Parser JSON: **11 arquivos**; parser YAML: **3 arquivos**, incluindo os workflows.
- Revisão estática dos grafos `needs`, dos seis labels de runner nas matrizes, pinning sintático SHA-40 das actions, `go.mod`, referências a import paths `/v2`, change record, Win32 errno e smoke que rejeita reparse point: passaram verificações de consistência.
- `bash -n` nos scripts shell: passou.
- `git diff --check`: sem whitespace inválido.
- Revisão documental em fontes oficiais listadas em `CODE_REVIEW.md`.

O parsing YAML e a inspeção de SHA-40 **não** provam a execução da action, disponibilidade de rede para seus downloads nem o build do aplicativo. Versões/pins foram mantidos conforme revisão anterior; a compatibilidade nativa da combinação completa exige o workflow real.

## Evidência histórica — não reutilizar como gate atual

Rodadas anteriores relataram `check-safe`, testes Linux, cross-build/vet Windows amd64+arm64 e compilação de testes Windows antes de elevar Go mínimo e remover `root_legacy`. Esses resultados pertencem àquela árvore anterior. O fuzz completo também não terminou naquela rodada (timeout após quatro alvos).

## Gates ainda necessários no commit exato desta árvore

- GitHub Actions CI nativo em Linux, macOS e Windows para amd64 e arm64 usando Go 1.27.1, incluindo Go test/vet, testes específicos do Windows e race somente onde o Go o suporta.
- Validação do isolamento de DACL em dados legados com ACE explícita permissiva (ver risco residual em `CODE_REVIEW.md`).
- `release prepare --suite-version v2.0.0` primeiro em preview e, após gates, `--write`; conferir migração do contrato público e manifests de produtos previstos para 1.1.0.
- Criar a tag v2 somente depois de CI verde. O workflow de release deve passar os seis smokes nativos sobre os mesmos bytes gerados pelo builder e publicar os arquivos verificados. Nenhum desses resultados foi observado neste sandbox.

## Auditoria adicional de workflows — 2026-09-26

**Execução desta rodada (local):** parse PyYAML 6.0.3 de ambos workflows, estrutura de 7 jobs CI/6 jobs release, verificação estática de `needs`, inputs de `setup-go`, SHA-40 de todas as Actions e runner labels; `bash -n` dos scripts (inclusive `check-workflows.sh` e `install-test-shells.sh`); `gofmt` em toda a árvore. Verificações de contrato adicionais e SHA-256 do ZIP serão registradas no fechamento do snapshot. Isto é inspeção estática, **não** CI remota verde.

**Bloqueios preservados:** Go local 1.23.2: `GOTOOLCHAIN=local go test ./...` recusa `go.mod` com Go 1.27.1. Não houve instalação de actionlint nesta VM nem execução dos 6 runners remotos; actionlint torna-se gate real ao integrar esta árvore. A existência do download/checksum oficial não equivale à execução do binário.

**Antes de criar v2.0.0:** integrar branch padrão com regras de proteção, rodar CI completa, conferir job `workflow-lint`, `change-records` em PR e se houver Merge Queue, e o Windows nativo em x64/ARM64. `release prepare --write` apenas após gates; tag v2 e `release.yml` exigem smokes de bytes idênticos antes de publicar o draft.

## Continuação — integridade da publicação remota (2026-09-26)

**Executado nesta árvore atualizada:** parser PyYAML (usando `BaseLoader` para preservar a chave `on`) dos dois workflows; consistência de `needs` em **7 jobs CI e 6 release**; seis targets nativos por matriz; checkout sem persistência de token e todos `uses:` presos a SHA-40; verificação estática de que a atestação pertence ao builder e de que `publish` possui somente `contents: write`; `bash -n` individual nos **5 scripts shell**; `gofmt -l` em **113 arquivos Go**; parser dos arquivos JSON. Estes resultados cobrem sintaxe/contratos estáticos, não execução GitHub.

**Não executado:** `actionlint` v1.7.12 (download não disponível nesta VM), chamadas remotas de `gh api`/draft/upload/download, shellcheck não presente, Go test/vet/build com Go 1.27.1, seis runners nativos, rerun real e checks de release imutável. A tentativa real `GOTOOLCHAIN=local go test ./...` foi bloqueada por `go.mod requires go >= 1.27.1 (running go 1.23.2)`. Um parser YAML não comprova o actionlint nem o comportamento de Actions.

Os novos guards de release permanecem **pré-condições fail-closed ainda não confirmadas pelo CI**. A primeira execução em GitHub deve verificar explicitamente: token `contents: read` na consulta do tag; atestação no builder; `upload-artifact` re-run; download autenticado dos assets de draft; comparação SHA-256 remota; promoção a publicado e consulta de pós-condição. A opção de immutable releases e regras de branch/tag devem ser auditadas na configuração remota, que não acompanha o ZIP.

