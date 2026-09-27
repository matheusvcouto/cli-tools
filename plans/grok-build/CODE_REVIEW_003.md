# Code review independente — Grok Build adapter (SNAPSHOT-003)

**Data:** 26/09/2026 · **Repositório:** `github.com/matheusvcouto/cli-tools/v2` · **Escopo:** código Grok introduzido no SNAPSHOT-002, contratos comuns, integração Windows/npm, CLI/ACP e regressões documentais. Este é um relatório de revisão e correções aplicadas — **não** uma certificação de execução do binário Grok.

## 1. Resumo das decisões após reconsulta oficial

O wrapper deve selecionar o estado por `GROK_HOME` e preservar argv/stdio: `run` encaminha argumentos como recebidos, `acp` lança `grok agent <opções> stdio`. Não tenta imitar xAI, autenticar por conta própria, ativar auto-approve nem executar shims `.cmd` por shell. A versão publicada do executável é independente do Go/toolchain usado por `cli-tools`.

A **referência de configuração mais nova** (documentação pública consultada em 26/09) lista `compat.codex.skills` e `compat.codex.hooks`, enquanto o guia introdutório `05-configuration.md` ainda os chama de reservados/inertes. Para fail-closed, o default de novos perfis define **ambos como `false`**. A efetividade no binário específico continua dependente do gate `grok inspect` com a versão instalada; não foi inventada uma prova de que o CLI antigo já os ativa. Os `sessions` seguem desativados como configuração defensiva e são descritos como staged/inertes no guia conceitual.

A referência também classifica `GROK_DISABLE_API_KEY_AUTH` e `GROK_FORCE_LOGIN_TEAM_ID` como restrições que podem ter valor administrativo. Sanitizar indiscriminadamente essas variáveis poderia reduzir a política de login. O SNAPSHOT-003 preserva ambas e remove somente overrides herdados de **identidade** (API key, OIDC/OAuth2, paths, host e external auth). Administradores podem piná-las por `requirements.toml`; o wrapper não tenta contornar os pins.

## 2. Defeitos/risco concretos encontrados e corrigidos

| ID | Local | Falha ou conflito da versão anterior | Correção vigente | Limite de validação |
|---|---|---|---|---|
| R01 | `internal/aiprofile/grok.go` / `service.go` | Parent shell podia conter aliases de autenticação ou redirects (`GROK_AUTH_PATH`, inline `GROK_AUTH`, chave antiga, endpoints de modelos). Um `GROK_HOME` isolado sozinho não impede todos os overrides externos. | `grokClearEnv` ampliada para credenciais, auth OIDC/OAuth2, relay websocket, overlays, proxy, modelos, arquivo/bucket/URL de trace upload e agentes externos. `GROK_HOME` sempre definido para o profile. | Revisão estática; login real com contas distintas pendente. |
| R02 | `grokForcedEnvironment` | SNAPSHOT-002 forçava compat Claude/Cursor `false` por env, derrotando qualquer `skills=true` editado intencionalmente no TOML do perfil. | Remover flags de compatibilidade **herdadas** e não reintroduzir overrides globais; defaults continuam `false` no config inicial, edição posterior do usuário pode optar por importar. | Unidade com runner de captura prevista, mas Go 1.27.1 indisponível neste sandbox. |
| R03 | `grokDefaultConfig` | `compat.codex` inicial tinha somente `sessions=false`; o catálogo oficial recente lista também `skills` e `hooks`. | Acrescentados `skills=false` e `hooks=false`. Mantido `sessions=false` como compat defensiva. | Documentação `05` e `26` diverge; inspeção contra binário oficial é gate separado. |
| R04 | `grokClearEnv` / política empresarial | Remover `GROK_DISABLE_API_KEY_AUTH` poderia permitir método de autenticação proibido no ambiente herdado. | Variável agora preservada. `GROK_FORCE_LOGIN_TEAM_ID`, `GROK_SANDBOX`, versões mínimas/máximas obrigatórias e requisitos administrados também preservados. | Verificação da composição de ambiente; não equivale a validar policies via IdP de uma organização. |
| R05 | `Service.ensureGrokProfile` | Um `Lstat` único seguido pelo launch admitia troca do leaf `config.toml` entre pré-verificação e próxima operação. | Abre `config.toml` via root confinado (`os.Root`), confere `f.Stat`, `root.Lstat` e `os.SameFile` nas três observações; rejeita symlink, objeto especial, desaparecimento ou troca de identidade. | Não impede alteração **depois** do preflight por outro processo do mesmo usuário. |
| R06 | `internal/aiprofile/service_test.go`, `integration/e2e_test.go` | Casos herdados não cobriam opt-in de compatibilidade, credenciais alternativas, casings Windows nem controles administrativos. | Casos de regressão de ambiente, edição local do TOML, comparação case-insensitive e trace/auth alternatives. Testes E2E de launcher continuam sintéticos, sem alegação de compatibilidade Grok real. | Go 1.27.1/Windows reais não executados. |
| R07 | `cmd/ai-profile/README.md` | Exemplo usava `grok-build` como model ID, que não é garantia contratual da conta/canal atual. | Exemplo ajustado para `grok-4.6` conforme guia oficial de ACP; disponibilidade continua variável por conta/versão. | É documentação, não uma chamada real ao serviço. |
| R08 | Docs históricas | SNAPSHOT-002 afirmava compat sempre forçada false e Codex hooks/skills inertes, ambos inadequados para estado atual. | Estado vigente registrado neste relatório, em `AGENTS.md`, `CONTEXT.md`, ADR e docs. Documentos SNAPSHOT-002 ganharam aviso explícito de histórico/superação, sem reescrever a evidência original. | Conferência estática. |

## 3. Revisão arquitetural: invariantes conservadas

- **Mutações transacionais:** `Store.createProfile` roda o inicializador `initGrokProfile` durante o lock; só publica o índice depois de criar e sincronizar `config.toml`. Em caso de erro anterior ao commit, o diretório novo é removido. Perfil existente nunca tem TOML reconstruído silenciosamente.
- **Separação de providers:** `ToolSpec` isolado por nome, sem alterar modelo de autenticação Claude/Codex; `grok` usa `GROK_HOME`, Claude `CLAUDE_CONFIG_DIR` e Codex `CODEX_HOME`. `env_key` de credenciais genéricas intencionais do projeto permanece assunto do provider e dos subprocessos.
- **Protocolo ACP:** `ACPTool.Args=[agent]`, argv do cliente no meio, `SuffixArgs=[stdio]`. Não há banners no stdout introduzidos pelo wrapper. Permissões/approval são delegadas explicitamente a Grok/cliente ACP; não se injeta `--always-approve`.
- **Windows:** `Runner.Replace` contém árvore por Job Object, preserva código de saída/cancelamento e usa argv real. `resolveLaunch` prefere `grok.exe`. Ao encontrar `.cmd` npm, exige package oficial `@xai-official/grok`, valida manifesto `bin.grok=bin/grok` e executa launcher por Node sem `cmd.exe`. Instalação upstream Windows ARM64 **não** é anunciada — o npm oficial documenta Windows x64.
- **Camada de segurança:** requisito externo por environment ou `requirements.toml` é preservado. `GROK_HOME` isola identidade/estado **global do provider**, não impõe sandbox para filesystem, rede, scripts de projeto ou instruções `AGENTS.md`/`CLAUDE.md` do cwd.

## 4. Problemas possíveis ainda não classificados como corrigidos

| Prioridade | Risco remanescente | Condição de ocorrência / ação correta |
|---|---|---|
| Alta | **Windows legados com ACE explícita permissiva** | O root do store recebe DACL protegida e ACEs herdáveis; arquivos existentes com ACL própria podem manter acesso indevido. Exige inspeção nativa dos security descriptors de profile/config/auth legados e um gate com usuário diferente. Não confundir proteção do root com saneamento recursivo. |
| Alta | **Identidade x políticas empresariais** | Se a empresa depender de `GROK_AUTH_PROVIDER_COMMAND` somente no shell, o wrapper o remove e o login pode falhar; isso é fail-closed, mas pede configuração autorizada dentro do profile/requirements. Testar restrições de IdP em ambiente empresarial e conferir `grok inspect` sem dados sensíveis. |
| Alta | **Overlay futuro/credencial não catalogada** | Grok muda rápido e um novo env, variável de credencial específica de modelo ou configuração MCP do projeto pode redirecionar dados; `grokClearEnv` é inventário **explícito**, não promessa de limpeza de todo o ambiente. Revisar `26-config-reference` e `grok inspect --json` em cada upgrade publicado. |
| Média | **Concorrência e sessão/leader** | Lock do store cobre índice, não runtime Grok. Dois `acp`/`run` simultâneos com o mesmo `GROK_HOME` podem competir por sessão, bootstrap, atualização ou leader. Necessita prova com binário real antes de impor locking que quebraria integrações legítimas. |
| Média | **Projeto potencialmente hostil** | Grok reconhece contexto e policies do projeto atual, além das compatibilidades explicitamente reabilitadas. Preferir diretório confiável e `grok inspect`; não declarar `GROK_HOME` sandbox. |
| Média | **Config alterada após preflight** | `os.Root` e comparação de identidade impedem uma classe de troca durante validação, mas não bloqueiam modificações posteriores pelo mesmo usuário, nem uma edição in-place. Se o threat model incluir o mesmo usuário concorrente, testar e projetar mecanismo específico antes de prometer prevenção. |
| Média | **Matriz upstream Windows** | `cli-tools` compila amd64/arm64, mas Grok upstream só anuncia Windows x64. Em Windows ARM64 a CLI wrapper pode existir sem Grok instalado; não criar um gate de Grok ARM64 obrigatório antes de evidência upstream. |
| Média | **Deriva do `main` público** | Mirror GitHub não representa necessariamente o pacote instalado. Registrar `grok --version`, package version, manifesto e SHA do binário real no gate; nada disso foi executado no ambiente atual. |
| Média | **Sobrescrita de política e logs** | Sandbox, permission defaults, config do projeto, hooks e managed config podem mudar comportamento sem tocar `GROK_HOME`. Nunca inserir `--yolo`, desabilitar requisitos ou copiar tokens para gerar sucesso de laboratório. |

## 5. Checklist de validação (evidência honesta)

| Camada | Método | Status no SNAPSHOT-003 |
|---|---|---|
| Formatação e parsing Go | `gofmt`, `./scripts/check-safe.sh fmt` | Executar e registrar saída local neste snapshot; não significa build. |
| Configuração e contrato | parse TOML literal de `grokDefaultConfig` e JSON de contratos; revisão da composição argv/env | Verificação estática; testes de comportamento de runner são casos de unidade não executados. |
| YAML/Actions | parse semântico de todos os workflows | Sintaxe local, sem equivalência com execução de runner do GitHub. |
| Testes Go | `GOTOOLCHAIN=local go test ./...`, `go vet ./...` com toolchain mínimo | Não executáveis com Go 1.23.2 do sandbox (o repositório exige 1.27.1). Não baixar/mudar código para contornar. |
| Funcional Grok | oficial `grok --version`, 2 profiles autenticados, TUI/headless, ACP client JSON-RPC e `grok inspect` | **Não executado.** Requer CLI oficial, conta de teste e ambiente autorizado. |
| Windows real | npm x64, native `.exe`, DACL, Job Object, ctrl-c/cancelamento, argv com `%`, `&`, aspas, Unicode e spaces | **Não executado** no Linux; runners Windows são gates futuros. |
| Integração CI/release | `actionlint`, CI windows-latest + Windows ARM64 wrapper e matriz publish dos 6 targets | Workflow descrito e revisado; execução remota do commit desta revisão pendente. |

### Gates concretos no ambiente real

1. Em host com **Go 1.27.1 oficial**, executar suíte completa, vet, lint/CI e os alvos Windows nativos configurados; capturar logs do commit exato.
2. Instalar **Grok Build oficial** em Linux/macOS e Windows x64, registrar versão, `grok inspect --json` em dois perfis diferentes e fontes de compatibilidade (global, projeto e perfil).
3. Autenticar dois `GROK_HOME` separados com identidades de teste, verificar sessão/credenciais não cruzadas sem imprimir tokens; validar endpoint e auth OIDC/API key conforme política da organização.
4. Abrir sessão ACP com cliente real: `initialize`, auth se necessário, `session/new`, prompt, eventos `session/update`, solicitações de permissão e cancelamento; conferir stdio puro.
5. No Windows x64, verificar installer, pacote npm oficial, Node, DACL de arquivos legados, Job Object, encerramento de descendentes e argumentos hostis de CLI. Não usar test double como prova da CLI Grok.
6. Só promover o gate quando cada evidência existir; se a versão real do Grok divergir do mirror, ajustar adapter à versão instalada sem baixar baseline nem criar prova artificial.

## 6. Fontes primárias, consultadas em 26/09/2026

- Configuração/precedência e políticas pinned: https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/26-config-reference.md
- Guia de compatibilidade (com discrepância Codex documentada): https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/05-configuration.md
- ACP e posição de opções: https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/15-agent-mode.md
- Segurança/permissions: https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/22-permissions-and-safety.md
- Distribuição npm e arquiteturas: https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/npm/grok/README.md
- Launcher npm: https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/npm/grok/bin/grok
- Código relevante deste projeto: `internal/aiprofile/{grok.go,model.go,service.go,service_test.go,store.go}`, `internal/aiprofile/platform/process_windows.go`, `integration/e2e_test.go`.

### Resultado das verificações efetivamente executadas em 26/09/2026

- `./scripts/check-safe.sh fmt` -> exit **0**, sem alteração de baseline.
- Parser JSON padrão em **12** arquivos -> sucesso; parser PyYAML em **2** workflows (`ci.yml`/`release.yml`) -> sucesso.
- Parser TOML padrão aplicado ao literal `grokDefaultConfig` -> sucesso; Claude/Cursor em seis células false e Codex em `skills`/`hooks`/`sessions` false.
- Revisão estática automatizada conferiu whitelist de remoção, ausência dos nomes de restrição administrativa nessa whitelist, `os.SameFile`, `ACPTool.SuffixArgs` e token `node.exe` no resolver Windows -> sucesso. Esses checks são textuais/estruturais, **não** compilação nem execução.
- `GOTOOLCHAIN=local go test ./...` -> exit **1** com `go.mod requires go >= 1.27.1 (running go 1.23.2; GOTOOLCHAIN=local)`; `go vet` recusou pelo mesmo motivo. **Não** rotular estes comandos como PASS de testes.
- `grok` e `actionlint` não instalados neste ambiente. Nenhum Grok autêntico, Windows nativo nem GitHub Actions do commit desta revisão foi executado.
