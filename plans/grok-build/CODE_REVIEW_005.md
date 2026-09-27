# SNAPSHOT-005 — Revisão de continuidade: perfis, recuperação e Windows

**Data:** 2026-09-26. **Base:** SNAPSHOT-004. **Alvo:** `github.com/matheusvcouto/cli-tools/v2`, Go 1.27.1. Esta revisão distingue código alterado, análise estática e validação de runtime pendente. Não equipara uma suíte escrita a uma suíte executada.

## Achados e correções

| ID | Severidade | Evidência no código anterior | Ajuste implementado | Gate real |
|---|---|---|---|---|
| S05-01 | Alta (disponibilidade) | `json.Unmarshal("null", &settings)` produz mapa nil em `ensureClaudeContextIsolation`, mas a função escrevia `settings["claudeMdExcludes"]`: o fluxo `claude run/acp` podia entrar em panic. | Rejeitar JSON `null` com erro explícito, sem alterar o settings nem lançar o processo. Teste de regressão incluído. | `go test ./internal/aiprofile -run TestClaudeRunRejectsNullSettingsWithoutPanic`. |
| S05-02 | Alta (integridade) | `loadRootOrEmpty` tratava todo `index.json` ausente como armazenamento novo, inclusive se `index.json.bak` ou diretórios de perfis persistiam. Isso pode criar índice desconectado de perfis e credenciais. | Detectar backup residual e diretórios com a nomenclatura real de alocação, recusar operação e instruir recuperação manual. Varredura por lotes de 128 para não materializar todo o diretório na memória. | `TestMissingIndexWithBackupFailsClosed`, `TestMissingIndexWithManagedProfileDirFailsClosed` e casos de root realmente vazio. |
| S05-03 | Média (disponibilidade/Windows) | `verifiedNPMEntrypoint` usava `os.ReadFile(package.json)` ilimitado, apesar de ser metadado de instalação externa. | Limite de 2 MiB antes/durante leitura, verificação da identidade de arquivo aberto contra Lstat inicial/final. Mantém manifesto/entrypoint oficiais e launcher Node direto. | `GOOS=windows GOARCH=amd64/arm64 go test ./internal/aiprofile/platform` **nativamente em Windows**; teste específico `TestWindowsVerifiedNPMEntrypointRejectsOversizedManifest`. |
| S05-04 | Baixa (documentação) | README raiz ainda listava só Claude/Codex; recuperação de índice não tinha procedimento documentado. | Atualizados README, change record e `docs/ai-profile.md` com suporte Grok e recuperação conservadora. | Revisão de documentação e teste de fluxo real. |

## Revalidação transversal

- **Grok/ACP:** `grok agent <opções> stdio`; sem `--always-approve` silencioso. `GROK_HOME` é específico por perfil; configurações locais não são reescritas automaticamente; overrides de autenticação herdados removidos seletivamente, restrições administrativas mantidas. A referência oficial atual documenta precedência de flags, ambiente, requirements/MDM, overlay e config. O upstream também admite `.mcp.json`, `.grok/` e instruções de projeto: não apresentar isolamento de home como sandbox do projeto.
- **Claude:** variáveis de autenticação herdadas e `CLAUDE_CONFIG_DIR`/`ANTHROPIC_CONFIG_DIR` revisados. `settings.json` não objeto JSON falha fechado; outras chaves existentes são preservadas. Não prometer que credenciais cloud genéricas de AWS/Google/Azure estejam isoladas, pois isso pode ser resolvido pela própria ferramenta via provider explicitamente selecionado.
- **Codex:** `CODEX_HOME` é perfil-local; ACP permanece binário externo `codex-acp`. Credenciais e contexto local do projeto são conceitos distintos. Não assumir que a assinatura Codex disponibilize API de cotas sem contrato documentado.
- **Store/FS:** backup atômico, lock, `os.Root`, diretório de quarentena para exclusão, tamanho de index/settings em 8 MiB, unicidade de diretórios. Novo guard fecha o caso em que o índice simplesmente desaparecia. Backup contém o índice **anterior** e não deve ser restaurado automaticamente sem auditoria de diretórios criados depois.
- **Windows:** locking Win32 e Job Object revisados estruturalmente. A resolução de npm/pnpm conserva a validação exata de pacote e entrypoint; pacote oficial Grok e Node devem ser instalados para o alvo em que serão executados. A DACL protegida do root ainda **não conserta ACEs explícitas abertas de arquivos legados**. Esse gate permanece pendente de validação nativa e projeto de migração seguro.
- **CI/release:** a matriz declarada mantém 6 plataformas e smoke dos artefatos que serão publicados; Actions pinadas. Isso é análise estrutural, não prova de uma execução remota de CI/release do SNAPSHOT-005.

## Limitações e evidências

O ambiente atual tem Go **1.23.2**, inferior ao mínimo **1.27.1** do módulo. Não rebaixamos a baseline, alteramos o código de produção para satisfazer o sandbox nem alegamos execução de `go test` / `go vet`. Os testes de regressão foram **escritos, mas não executados**. Também não houve nesta rodada login de Grok/Claude/Codex, ACP autenticado, processo Windows nativo ou execução dos workflows no GitHub.

A revisão documental usa:

- https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/15-agent-mode.md
- https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/05-configuration.md
- https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/26-config-reference.md
- https://go.dev/src/os/root_openat.go (confinamento da operação Root.Rename; Windows usa API nativa com semântica própria)
- https://go.dev/blog/osroot (limites de confinamento, sistemas de arquivos e Windows)

## Gates pendentes para declarar compatibilidade de produção

1. Go 1.27.1 oficial no target: `./scripts/check-safe.sh all`, `go test ./...`, `go vet ./...`, `go test -shuffle=on -count=3 ./...`; race detector onde suportado.
2. GitHub Actions do commit exato em Linux/macOS/Windows AMD64/ARM64 e teste de release não publicado, com checksums e smoke dos mesmos bytes.
3. Grok oficial autenticado: dois perfis independentes, TUI, headless, ACP JSON-RPC, permissões por sessão, configuração customizada e reinício; Windows x64 prioritário. Verificar disponibilidade nativa do Grok no Windows ARM64 separadamente.
4. Win32 nativo com perfil novo e legado: segurança DACL de arquivos existentes, locking entre processos, Job Object, instalação npm/pnpm real, tamanho/identidade do `package.json` e Unicode/paths com espaços.
5. Cenários de recuperação de índice real com backup desatualizado, queda entre criação e commit e quarentena após falha de exclusão. Não recuperar automaticamente os dados.
6. A futura **View Limits** permanece somente arquitetada em `plans/view-limits/README.md`, nunca deve ler tokens sem consentimento, confundir quota de API e assinatura ou inferir cotas inexistentes.

## Validação estática desta rodada — executada no ambiente disponível

- `gofmt -l .`: **0 arquivos pendentes**.
- `bash -n` de todos os scripts `scripts/*.sh`: **PASS**.
- Parse de **12 arquivos JSON**: **PASS**.
- Parse de **2 workflows YAML**, checagem estrutural dos **13 jobs**, `needs` existentes e SHA completo em `uses:` externos: **PASS estrutural** (não equivale a `actionlint` nem a execução no GitHub).
- Parse real por `tomllib` de `grokDefaultConfig` e verificação de defaults `compat.* = false`: **PASS**.
- O requisito de Go foi confrontado com `GOTOOLCHAIN=local go test ./internal/aiprofile ./internal/aiprofile/platform`: **BLOQUEADO** — `go.mod requires go >= 1.27.1 (running go 1.23.2; GOTOOLCHAIN=local)`. Nenhum teste Go foi marcado como PASS.
