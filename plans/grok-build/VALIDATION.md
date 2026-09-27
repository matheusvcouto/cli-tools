> **Registro histórico do SNAPSHOT-002.** Para o código vigente e as correções de isolamento/compatibilidade validadas em 26/09/2026, leia [`CODE_REVIEW_003.md`](CODE_REVIEW_003.md). Em particular, os overrides de compatibilidade herdados agora são *removidos*, não forçados `false`; Codex `skills`/`hooks` constam da referência oficial mais recente; e `GROK_DISABLE_API_KEY_AUTH` é preservado por ser restrição de segurança.

# Validação — Grok Build adapter · SNAPSHOT-002

## Confirmado por código e documentação oficial

- Provider `grok`, `GROK_HOME`, config inicial e rollback transacional implementados.
- `run` preserva argv opaco e cwd; `acp` compõe `grok agent <args> stdio` sem banners no stdout.
- Nenhum `--always-approve`/`--yolo` é inserido pelo wrapper.
- Sanitização é explícita; controles de segurança `GROK_*` não são apagados em massa.
- Windows recusa `.bat`/`.ps1` e `.cmd` desconhecido; Grok npm só é aceito via manifesto/entrypoint verificado e execução direta com Node.
- Contract/help/completion derivam do mesmo `Tools` graph; `cli.contract.json` foi atualizado para o terceiro provider.
- E2E sintético cobre launcher Grok (GROK_HOME, remoção de auth/config herdado, preservação de `GROK_SANDBOX`, argv ACP e exit status). **Isso não é execução funcional do Grok.**

## Revalidações que mudaram a implementação

1. Documentação de agent mode confirmou que opções vêm depois de `agent` e antes de `stdio`; isso motivou `ACPTool.SuffixArgs`.
2. Configuração atual confirmou que `[compat.codex]` só possui `sessions` útil; campos inertes foram removidos do default.
3. Documentação de permissions/sandbox confirmou que always-approve é uma escolha explícita e que deny rules/hooks/sandbox são camadas independentes; o wrapper não as desativa.
4. O launcher npm oficial público é `bin/grok` e chama `grok-bootstrap.js`; o resolver Windows valida o manifesto instalado antes de confiar nesse caminho.

## Ambiente e gates não executados

Ambiente disponível: Linux amd64 com Go 1.23.2, enquanto `go.mod` exige Go 1.27.1. Não há `grok` autenticado, Windows/macOS nativos ou credenciais xAI fornecidas.

Portanto permanecem **NÃO EXECUTADOS**, e não são marcados como PASS:

- `grok --version`/`--help` de uma instalação oficial registrada;
- login real e persistência de credenciais em dois `GROK_HOME` distintos;
- headless real (`-p`, JSON/streaming JSON), erros, cancelamento e exit status;
- ACP real (`initialize`, auth quando aplicável, `session/new`, `session/prompt`, permissions e `session/update`);
- `grok inspect`/proveniência de regras em projeto confiável e não confiável;
- Windows x64 nativo com installer oficial e com npm global; Job Object + bootstrap; paths com espaços/Unicode;
- disponibilidade real por arquitetura upstream, em especial Windows ARM64/macOS Intel;
- `go test ./...`, race e matriz CI usando Go 1.27.1 desta revisão.

## Fontes principais reconsultadas

- https://docs.x.ai/build/overview
- https://docs.x.ai/build/cli/reference
- https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/05-configuration.md
- https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/14-headless-mode.md
- https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/15-agent-mode.md
- https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/18-sandbox.md
- https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/22-permissions-and-safety.md
- https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/npm/grok/bin/grok

## SNAPSHOT-005 — regressões adicionadas, execução pendente

- `TestClaudeRunRejectsNullSettingsWithoutPanic`: evita panic com JSON `null` e preserva settings do usuário.
- `TestMissingIndexWithBackupFailsClosed` / `TestMissingIndexWithManagedProfileDirFailsClosed`: evita reset silencioso após perda do índice.
- `TestStoreWithoutIndexOrManagedStateIsEmpty`: preserva bootstrap normal.
- `TestManagedProfileDirectoryNameRecognition`: somente nomes físicos gerados oficialmente acionam guard de diretório.
- `TestWindowsVerifiedNPMEntrypointRejectsOversizedManifest`: impede crescimento não limitado de `package.json` no resolvedor Windows.
- `go test` e `go vet` desta rodada NÃO executados: mínimo Go 1.27.1 indisponível neste ambiente. Para evidência efetiva, rodar Go 1.27.1 e CI nas seis plataformas.
