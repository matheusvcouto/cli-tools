> **Registro histórico do SNAPSHOT-002.** Para o código vigente e as correções de isolamento/compatibilidade validadas em 26/09/2026, leia [`CODE_REVIEW_003.md`](CODE_REVIEW_003.md). Em particular, os overrides de compatibilidade herdados agora são *removidos*, não forçados `false`; Codex `skills`/`hooks` constam da referência oficial mais recente; e `GROK_DISABLE_API_KEY_AUTH` é preservado por ser restrição de segurança.

# Grok Build no `ai-profile`: investigação, riscos, conflitos e plano de integração

**Projeto:** `github.com/matheusvcouto/cli-tools/v2` · **Data da investigação:** 2026-09-26  
**Entrada:** `SNAPSHOT_ACTIONS.zip` (216 entradas) disponibilizado nesta conversa.  
**Estado:** plano original executado no SNAPSHOT-002; provider Grok adicionado ao código. Gates funcionais reais permanecem explicitamente separados em `VALIDATION.md`.  
**Escopo:** acrescentar `ai-profile grok ...` como **novo provider de CLI**, não implementar a API REST da xAI nem confundir o modelo `grok-*` com o agente Grok Build.

## 1. Resultado objetivo

O Grok Build disponibiliza o executável oficial `grok` e três interfaces distintas:

| Interface | Contrato documentado | Papel no futuro adapter |
| --- | --- | --- |
| TUI | `grok` | `ai-profile grok run <perfil> [args...]`, com terminal real. |
| Headless | `grok -p "..." --output-format json\|streaming-json` | Mesmo `run` e repasse **opaco** de `argv`; JSON pertence ao Grok e não deve ser reinterpretado pelo `ai-profile`. |
| ACP | `grok agent stdio` | `ai-profile grok acp <perfil> [args...]`; JSON-RPC/ACP cru em stdin/stdout, sem wrapper npm de terceiros. |

**Decisão proposta:** `ToolSpec{Name:"grok", Binary:"grok", ConfigEnv:"GROK_HOME", ACP:&ACPTool{Binary:"grok", Args:[]string{"agent","stdio"}}}`, porém somente depois de implementar o tratamento dos riscos abaixo. `GROK_HOME` é suportado oficialmente para realocar `auth.json`, `config.toml`, sessões SQLite, memória e logs. Para instalações de automação, `--no-auto-update` deve ser oferecido de maneira documentada e validada com `grok --help` da versão-alvo, sem inserir flags à força antes de argumentos fornecidos pelo usuário.

O Grok Build **já implementa ACP nativamente**: instalar `grok-acp`, `claude-agent-acp` ou outro bridge é um erro de arquitetura para este provider. `ai-profile` é lançador e isolador de perfil; não é cliente ACP. Se for necessário hospedar uma sessão, o cliente externo deve negociar `initialize`, `authenticate` quando exigido, `session/new`, `session/prompt` e `session/update` conforme versão/capabilities. Não escrever banners nem status em stdout do processo ACP.

## 2. Evidência e ressalvas sobre fontes

Documentação oficial do produto e código do repositório público `xai-org/grok-build` foram consultados via páginas públicas. O repositório do Grok declara ser **sincronizado periodicamente** do monorepo e usa `SOURCE_REV`; **`main` não garante correspondência bit-a-bit com o binário instalado**. Documentação antiga no diretório `xai-grok-shell` diverge de documentação mais recente em detalhes de autenticação; para segurança, adotar as instruções atuais de `02-authentication.md` e medir o comportamento da versão efetivamente instalada. O pacote oficial npm `@xai-official/grok` informa suporte a macOS arm64, Linux x64/arm64 e Windows x64; isso **não prova** distribuição nativa Windows arm64 nem macOS Intel na mesma modalidade. Nunca tratar o runner Windows ARM64 do `cli-tools` como evidência de que o provider Grok possui binário nativo arm64.

Fontes oficiais e pontos verificados:

- Produto e ACP: https://docs.x.ai/build/overview ; https://docs.x.ai/build/cli/headless-scripting ; https://docs.x.ai/build/cli/reference
- Código-fonte público, caráter de espelho do monorepo: https://github.com/xai-org/grok-build/blob/main/README.md
- Home, overlays, compatibilidade cruzada e política: https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/05-configuration.md
- Autenticação e prioridade real das credenciais: https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/02-authentication.md
- Instruções herdadas: https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/12-project-rules.md
- Modo headless, dados persistidos e opções: https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/14-headless-mode.md
- Segurança/permissões/ACP: https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/22-permissions-and-safety.md
- Instalação npm oficial e plataformas: https://www.npmjs.com/package/@xai-official/grok ; https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/npm/grok/README.md
- Launcher JS npm observado no repositório: https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/npm/grok/bin/grok
- Abordagem externa para comparação, não norma: https://github.com/awslabs/cli-agent-orchestrator/blob/main/docs/grok-cli.md

**Limitação explícita:** o container não resolve `github.com` via `git ls-remote`, não contém `grok` nem Go 1.27.1, e nenhum login xAI/Windows nativo foi disponibilizado. Foi possível ler as páginas oficiais via busca/navegação web e revisar o código local do `ai-profile`; **nenhum teste funcional Grok, login, ACP ou Windows Grok foi executado**. A indicação de npm `bin/grok` no espelho deve ser confirmada contra o `package.json` **da versão npm realmente instalada** antes de aceitar `.cmd`.

## 3. Auditoria da arquitetura existente (estado do ZIP de entrada)

| Local no repositório | Achado concreto | Alteração planejada |
| --- | --- | --- |
| `internal/aiprofile/model.go` | `Tools` define somente `claude` e `codex`; `ToolSpec` já permite `ConfigEnv`, `ClearEnv`, ACP com binário+prefixo de argumentos. | Declarar provider Grok sem interface nova genérica. |
| `internal/aiprofile/service.go` | `Run`/`ACP` aplicam ambiente isolado; `profileEnvironment` só tem exceção Claude; `prepareProfile` falha para ferramentas diferentes. | Estender whitelist com `grok`; preflight específico que valide perfil/config e compatibilidade. |
| `internal/aiprofile/store.go` | `StoreSchemaVersion=1`, dados contêm `{tool,alias,dir,...}`; pastas únicas com prefixo de ferramenta; `safefs/os.Root` e locks. | Adição de `grok` não exige schema v2; **validar** código do decoder para aceitação de terceira ferramenta. |
| `internal/aiprofile/platform/process_windows.go` | Executa PE nativo sob Job Object; `.cmd` conhecido vira entrypoint npm verificado (`claude`, `codex`, bridges). `.cmd` desconhecido falha fechado. | `grok.exe` instalado oficialmente funciona pelo caminho nativo **se estiver no PATH**; npm `.cmd` precisa verificação de pacote/entrypoint oficial; jamais `cmd.exe /C`. |
| `internal/aiprofile/platform/process_unix.go` | Spawn/exec sem shell. | Manter execução direta e propagação de status/sinais. |
| `internal/aiprofile/cli/app.go` | Comandos `list/new/rename/delete/run`; ACP gerado quando `ACP!=nil`. | `grok` é incluído automaticamente; rever IDs, help, contratos e completions. |
| `cmd/ai-profile/*.json`, `cli.contract.json`, docs e golden tests | Referências e expectativas Claude/Codex. | Regenerar contratos usando a CLI real e atualizar testes com **assertivas reais** de Grok. |
| `integration/e2e_test.go` | Matriz de Claude/Codex usa binários-probe internos para testar **o launcher**, não os CLIs reais. | Adicionar contratos de repasse/isolamento sem confundir probe com prova do Grok; adicionar testes nativos reais opcionais separados. |
| `.github/workflows/ci.yml`, `release.yml` | Já há matriz das seis plataformas. | Testes unitários Grok entram em CI; integração com binário Grok verdadeiro só em runners/plataformas suportados e com auth apropriada; não criar gate fictício. |
| `plans/windows-support/CODE_REVIEW.md` | DACL de root protegida não elimina ACEs explícitas permissivas em filhos legados. | O novo provider não deve habilitar persistência de credenciais em perfil legado sem revisão dessa proteção. |

**Separação por plataforma:** mantenha domínio Genérico em `aiprofile`, o adaptador de processo Unix e Windows já existentes e, se necessário, acrescentar descoberta Grok npm apenas em arquivo Windows ou tabela Windows. Não implementar lógica de caminho Windows em `service.go` nem acoplar a release de Go aos binários Grok externos.

## 4. Matriz de falhas e conflitos (ordenada por consequência técnica, não por conveniência)

| ID | Falha / conflito | Impacto possível | Prevenção e evidência necessária |
| --- | --- | --- | --- |
| G01 | Apenas trocar `ConfigEnv` por `GROK_HOME` e declarar isolamento completo. | Leitura de instruções, skills, hooks e MCP do `~/.claude`, `~/.cursor`, `~/.claude.json`, `.mcp.json`, projeto; comandos inesperados; mistura de contexto. | Perfil novo com `config.toml` explícito desativando **cada célula** `[compat.claude]` e `[compat.cursor]` (agents/rules/skills/mcps/hooks/sessions), `grok inspect` com origens checadas. **Arquivos top-level `CLAUDE.md` do projeto continuam válidos** e devem ser declarados como contexto do projeto, não erro do `GROK_HOME`. |
| G02 | Herdar `XAI_API_KEY`, credenciais OIDC externas, `GROK_DEPLOYMENT_KEY` ou `GROK_AUTH_PROVIDER_COMMAND`. | Perfil roda com outra conta, endpoint ou privilégio; chaves vazam para processos filhos. | `ClearEnv` Grok específico, keys case-insensitive no Windows; remover credenciais/routing Herdados, permitir importação **explícita** e controlada quando o usuário requer API key/SSO; auditar subprocess env da própria CLI. Nunca ler/cópia tokens de `~/.grok/auth.json`. |
| G03 | Presumir que API key tem prioridade sobre login do perfil. | Conta selecionada diferente; cobrança/confusão. | Guia oficial atual: chave específica de modelo > token de sessão > `XAI_API_KEY` de fallback. Testar com instalação **real**; logs não revelam chaves. Documentar que assinaturas e chave API são formas distintas de acesso. |
| G04 | Herdar `GROK_CONFIG`/`GROK_CONFIG_PATH`, `GROK_AGENT`, `GROK_MODELS_BASE_URL`, `GROK_CLI_CHAT_PROXY_BASE_URL`, `GROK_LOG_FILE`/`GROK_DEBUG_LOG`, caminhos extras, telemetry overrides. | Overlay muda modelo/regras; endpoint externo; arquivos/logs fora do perfil; isolamento aparente. | Remover overlays e overrides não confiáveis da origem (inventário completo pela versão real); produzir config local auditada com `safefs`, permissões; não supor que overlay controla política de segurança — ele é intencionalmente limitado. |
| G05 | Executar `grok.cmd` ou `grok.ps1` por shell no Windows. | Interpretação de `&`, `%`, aspas etc.; argumentos arbitrários e ruptura do stdio ACP. | Preferir `grok.exe`. Para npm, adicionar **somente** mapping `@xai-official/grok` com `bin` verificado para a versão instalada, alvo de nó confiável, sem shell. Rejeitar metadados ausentes/divergentes e wrappers desconhecidos. |
| G06 | Supor Grok Build Windows arm64 e macOS Intel nativos porque `cli-tools` os compila. | Gate exige binário inexistente, erro de runner. | Matriz de descoberta por plataforma/arquitetura; npm oficial documenta Windows x64 e macOS arm64. Windows ARM64: marcar integração Grok nativa **não verificada** (x64/emulação só após teste); nunca prometer instalação universal. |
| G07 | Misturar ACP e headless/TUI. | stdout não é JSON-RPC, cliente falha initialize, prompts de login corrompem protocolo. | ACP chama `grok agent stdio`, headless usa `-p`, TUI usa `grok`. Preflight de autenticação antes do client ACP; stderr separado; não imprimir banners no stdout. |
| G08 | Presumir que basta iniciar ACP e escrever prompt diretamente em stdin. | Negociação/autenticação quebradas; sessão sem resposta. | `ai-profile acp` repassa bytes; cliente ACP externo faz initialize, lê authMethods, authenticate, session/new, prompt e update; teste real de handshake sob credenciais do teste. |
| G09 | `--always-approve`/`--yolo` por padrão ao usar ACP/CI. | Ferramentas, hooks e MCP executam sem intervenção humana. | Nunca auto-habilitar; opção somente por decisão explícita de usuário/pipeline; deny rules/hard limits via Grok; documentar que o Job Object só controla processos, não acesso de arquivos/rede nem conteúdos de prompts. |
| G10 | Config project `.grok/config.toml`, `.mcp.json`, `AGENTS.md` não confiáveis ou hooks de terceiros. | Código local inicia servidores/execuções ao abrir workspace. | Distinguir isolamento por *perfil* de confiança do *projeto*. Respeitar controles de trust do Grok, bloquear projeto não confiável ou exigir autorização; inventário via `grok inspect`; não prometer bloqueio global via `GROK_HOME`. |
| G11 | Arquivos Grok (`auth.json`, SQLite, logs) herdarem ACE explícita permissiva em perfil Windows legado. | Vazamento entre usuários locais. | Antes de autenticar, endurecer permissões de **toda a árvore relevante** com APIs Win32/fail-closed; não confiar apenas em DACL herdável do root. Gate Windows real de leitura por outra identidade onde viável. |
| G12 | Edição concorrente de `~/.grok/config.toml`, `auth.json` ou sessões dentro do mesmo perfil. | Erros de lock, race, corrupção, perda de token/estado. | `Store` serializa índice **somente**, não o runtime Grok. Testar duas instâncias reais compartilhando `GROK_HOME` (headless + ACP/TUI) e decidir serialização opt-in/advertência conforme comportamento confirmado. |
| G13 | Preflight escrever config.toml a cada `run` e substituir alterações do usuário. | Config do usuário sobrescrita; corrida, symlinks e perda de identidade. | Criar no `new` de forma transacional via `safefs` somente **se inexistente**, declarar modelo de propriedade, e no `run` verificar condições críticas sem reset silencioso; config existente incompatível falha com instrução específica. |
| G14 | Corrigir mistura de contexto sobrescrevendo `HOME`/`USERPROFILE` inteiro. | Git/SSH/Node/npm, credential helpers, PATH e aplicações do projeto mudam; ferramentas quebram. | Usar `GROK_HOME` oficial + política de compatibilidade. Não reescrever `HOME`; explicar limites dos arquivos de projeto. |
| G15 | Segredos nos logs/telemetria/cópias de sessão (incluindo `.md`, snapshots e CI artifacts). | Exfiltração de prompts, tokens ou caminho interno. | Redação por padrão, arquivos privados, excluir `auth.json`, SQLite, logs, diagnósticos e `.grok` privados de ZIP/CI; telemetria conforme escolha do usuário e política corporativa, sem alegar desativação total sem medir. |
| G16 | `--no-auto-update` inserido na posição errada ou flags desconhecidas de outra versão Grok. | Erro de parser; mudanças de comportamento por atualização automática. | Detectar `grok --version` e flags oficiais em laboratório real, congelar versão testada, não depender de `main`; documentar instabilidade e política de updates. |
| G17 | `os/exec` Windows escolher wrapper/shim inesperado quando coexistem executável oficial, WinGet e npm. | PATH errado, CLI errada, comportamento intermitente. | Precedência documentada: `grok.exe` nativo verificado quando localizado; npm conhecido só com manifesto coerente; diagnósticos com caminho/canal/versão **sem emitir secrets**. Teste WinGet, PowerShell installer, npm local/global/pnpm quando suportado. |
| G18 | Confundir erro de autenticação, cancelamento, erro de rede e código do agente. | UI informa sucesso ou retorna código errado. | Preservar código filho como já faz Runner; categorizar somente no `doctor` com evidência; testar cancelamento de árvore Windows/Unix, broken pipe e EOF em ACP. |
| G19 | Integrar CLI em `grok` mas sem atualizar contratos e arquivos de versão. | `--help`, completion JSON, docs e E2E divergem do runtime; release quebra. | Atualizar testes, golden, doc/manifest, contrato JSON, changelog e change-record `ai-profile` após codificar; liberar somente conforme política atual `/v2` e seis targets. |
| G20 | Chamar documentação do espelho GitHub de prova de funcionamento do binário distribuído. | Premissas de flags, package `bin`, ACP e auth envelhecem. | Capturar **versão instalada, manifesto npm, hashes e testes reais**; documentação serve para fundamentar implementação, não substitui testes quando hardware/conta existem. |

### 4.1 Política concreta de isolamento para um perfil novo

Config **proposta** (`<perfil>/config.toml`), a ser validada contra a versão instalada, criada apenas após consentimento da semântica de *perfil isolado*:

```toml
[cli]
auto_update = false

[compat.claude]
skills = false
rules = false
agents = false
mcps = false
hooks = false
sessions = false

[compat.cursor]
skills = false
rules = false
agents = false
mcps = false
hooks = false
sessions = false

[compat.codex]
sessions = false
```

**Não é garantia total de sandbox.** `GROK_HOME` separa a configuração global Grok, porém o projeto escolhido continua podendo conter `AGENTS.md`, `CLAUDE.md`, `.grok/` e `.mcp.json` reconhecidos pelo Grok; managed config e requirements empresariais prevalecem; `--yolo`, ferramentas de shell, arquivos do projeto e rede possuem políticas independentes. Definir claramente se usuários esperam separação de contas/sessões ou sandbox de segurança; o `ai-profile` atual fornece a primeira, não a segunda. `grok inspect` e testes de proveniência precisam confirmar o isolamento observado. Config existente de usuário não pode ser substituída pelo adaptador sem consentimento.

**Lista inicial de sanitização do processo Grok** (revisar automaticamente com a documentação de variáveis da versão-alvo antes do merge): `XAI_API_KEY`, `GROK_HOME` (substituído), `GROK_CONFIG`, `GROK_CONFIG_PATH`, `GROK_AGENT`, `GROK_AUTH_PROVIDER_COMMAND`, `GROK_AUTH_PROVIDER_LABEL`, `GROK_AUTH_TOKEN_TTL`, `GROK_AUTH_EXPIRED`, `GROK_OIDC_ISSUER`, `GROK_OIDC_CLIENT_ID`, `GROK_CLI_CHAT_PROXY_BASE_URL`, `GROK_MODELS_BASE_URL`, `GROK_MODELS_LIST_URL`, `GROK_DEPLOYMENT_KEY`, `GROK_LOG_FILE`, `GROK_DEBUG_LOG`, `GROK_SANDBOX` (se política local exigir), overrides de compatibilidade e tracing/telemetria herdados. **Esta lista não é completa nem prescrição para limpar variáveis genéricas do projeto**; chaves/proxies customizados e campos `env_key` merecem revisão caso a caso. É preciso preservar `PATH`, arquivos e variáveis do ambiente de desenvolvimento, salvo colisões específicas explicitamente justificadas. Para login por API key, criar fluxo **explícito**, não restaurar chave arbitrária que veio da shell.

### 4.2 Caminho Windows/npm sem execução por shell

O launcher observado no repositório npm oficial usa `#!/usr/bin/env node` e carrega `./grok-bootstrap.js`; um `.cmd` gerado por npm invoca Node. No Windows, o Runner atual só aceita `.cmd` se houver mapping do **nome do pacote e entrypoint oficial**. Antes de codificar:

1. Instalar ou inspecionar pacote oficial `@xai-official/grok` na **versão realmente testada** e registrar `package.json`/`bin.grok`, manifesto de distribuição e hash. A documentação aponta `npm i -g @xai-official/grok`; o caminho de código público `npm/grok/bin/grok` é indício, **não confirmação do manifesto publicado**.
2. Caso `grok.exe` nativo exista no PATH, usá-lo sem Node; caso `.cmd`, resolver manifesto, path sob `node_modules`, reparse/symlink, entrypoint e `node.exe` com as verificações Windows já existentes. **Nunca** invocar `.cmd`, `.bat` ou PowerShell com argumentos opacos.
3. Validar empacotamentos npm/pnpm, instalações locais e de usuário; recusar Yarn/PnP/shims desconhecidos até haver contrato verificável. Não oferecer fallback por shell no `doctor` nem na execução.
4. Rodar ACP contra o mesmo binário assim resolvido; certificar que o bootstrap Node não escreve mensagens de status em stdout antes do JSON-RPC.

### 4.3 ABI, terminal e processo

`ai-profile grok run perfil` deve usar terminal herdado real. Se stdin/stdout forem pipes, **não converter silenciosamente TUI em headless**: exigir que o chamador especifique `-p` ou documentar limitação. `acp` requer três streams diretos e nenhum log inserido em stdout. Reutilizar cancelamento Unix e Windows Job Object existentes; avaliar execução de Grok com subprocessos e terminação da árvore. Não inventar ConPTY de emergência, `cmd.exe` ou `sh -c` no adaptador.

## 5. Plano de execução por etapas e gates

**Fase P0 — Contrato e baseline (antes de tocar produção):**

1. Registrar a versão Grok estável testada para cada host, plataforma e método de instalação; coletar `grok --version`, `grok --help`, `grok agent --help`, manifesto npm efetivamente publicado, `SOURCE_REV` correspondente quando disponível e documentação da mesma geração. Assinalar ausência de suporte nativo com dados, não com suposição.
2. Confirmar autenticação sem navegador em ACP com perfil limpo: token local do próprio perfil e API key apenas por caminho explícito. Observar precedência de sessão vs API key na versão testada. Validar `grok inspect` em diretório Git confiável com dados de `~/.claude` e `~/.cursor` sentinelas **somente em ambiente de teste controlado**; dados reais não entram na evidência.
3. Definir o contrato de `new`: criar config mínima segura como **default do perfil**, não modificar global `~/.grok`; perfis antigos devem falhar fechado se isolamento requerido não estiver provado. Definir se o usuário pode configurar compatibilidade intencionalmente por comando futuro (fora do MVP).
4. Registrar estado como `investigated`, não `supported`. P0 só promove após provas citadas e revisão estática.

**Fase P1 — Domínio, armazenamento e ACL:**

1. `model.go`: `ToolSpec grok`, `ConfigEnv=GROK_HOME`, `ClearEnv` específica, `ACP: grok agent stdio`. Não criar versão nova do Store se schema atual aceita outros nomes; verificar `Store.validate`/decode e migrações antes.
2. `service.go`: aceitar `grok` no `prepareProfile`, provisionar `config.toml` inicial por operação `safefs` e commit atômico (apenas quando o perfil não tinha config); validação fail-closed dos controles críticos e permissões do diretório. Não gravar em cada execução se config já existe.
3. `permissions_windows.go`: revisar DACL de filhos existentes antes de persistir `auth.json` e SQLite; testes nativos com identidades/ACEs reais. Unix: 0700/0600, symlinks, hardlinks, ausência de janela TOCTOU. Garantir ausência de cópias de credenciais nos ZIPs, logs e diagnósticos.
4. `Store` permanece como índice multiprovider e lock do índice, sem alegar serialização das sessões Grok. Conferir rename/delete quando processos ativos para evitar dangling session roots.

**Fase P2 — Plataforma e protocolo:**

1. Unix: reutilizar runner real. Windows: executável WinGet/installer oficial primeiro; opcional npm conforme manifesto comprovado. Não alterar política fail-closed de `.cmd` desconhecido. Manter testes de argv opaco, variáveis Windows case-insensitive, stdout binário e exit code.
2. ACP passthrough com prefixo `agent stdio`; nenhum parser/rewriter JSON-RPC no `ai-profile`; cliente de integração **real** deve negociar `initialize`/`authMethods`/`authenticate`/`session/new` e receber `session/update`; versões podem diferir.
3. Headless `grok -p ... --output-format json` e `streaming-json`: testes reais observando stdout, stderr, timeout, exit code. `--no-auto-update` somente após validar posição de flag na CLI da versão fixada.
4. Interação com projeto confiável e isolamento da árvore: `grok inspect`, project AGENTS/CLAUDE/.grok com origens documentadas, MCP/hook por configuração explícita.

**Fase P3 — Testes e CI, com classes de evidência distintas:**

- Unitários Go **reais** sobre regras de composição de env e config, `LookupTool`, `Service.prepareProfile`, store multi-provider, idempotência de config, conflitos/renomeações, ACL/symlinks e falha fechado. Teste de `Runner` usando executáveis-probe controlados é legítimo **somente** como evidência do runner/argv, não compatibilidade funcional Grok.
- `grok` oficial instalado de verdade (canal e versão registrados): smoke `--version` e help; perfil recém-criado; TUI com terminal real; login por perfil; headless com API key temporária aprovada pelo usuário/CI; ACP com cliente real e auth handshake; interrupção/sinais; concorrência. Não fazer login ou automação não autorizada na conta pessoal do usuário.
- Matriz: Linux x64/arm64 e macOS arm64 conforme oferta documentada; Windows x64 nativo obrigatoriamente para declaração Windows; macOS Intel e Windows ARM64 são **investigar e testar antes de suportar**, podendo compilar `ai-profile` nativamente sem o binário Grok correspondente. Não usar emulação para rotular suporte nativo.
- CI público sem segredos valida domínio/runner/contrato Go nos seis targets. E2E autenticado só em ambiente privado habilitado por credencial temporária e autorização; segredos fora de artifacts/logs, rate limits e timeout; ausência de credenciais vira `not run`, nunca PASS simulado.

**Fase P4 — Experiência, versionamento e release:**

1. `cmd/ai-profile/README.md`, `docs/ai-profile.md`, ADR, `cli.contract.json`, golden help/schema/completions, `integration/e2e_test.go` e `cmd/ai-profile/tool.json` coerentes. Adicionar `grok doctor` apenas se o grafo de CLI oferecer diagnóstico de plataforma real e sem vazamento.
2. Change record de `ai-profile` com incremento de feature após código implementado; **não** alterar `go.mod` nem `/v2` por causa deste provider. Compatibilidade do CLI de terceiros instalada pelo usuário deve constar como pré-requisito, nunca ser embutida silenciosamente na release.
3. Rodar Go 1.27.1 oficial e `scripts/check-safe.sh all`, `go test`/`vet`, Windows native, actionlint, CI seis plataformas; publicar só quando cada gate definido tiver evidência executada. Resultado só de revisão estática conta como revisão estática.

### Critério objetivo de conclusão

O provider pode ser anunciado como integrado quando `new/list/rename/delete/run/acp` estão implementados; ao criar dois perfis Grok, cada um autentica/persiste apenas sob seu `GROK_HOME`; `grok inspect` não mostra importação acidental global Claude/Cursor; não há injeção de shell no Windows; ACP oficial funciona com cliente real; versões/plataformas suportadas estão medidas; docs/help/contratos batem com o binário compilado; e os gates Go 1.27.1 + nativos Windows exigidos foram executados. Caso um gate externo esteja indisponível, avance apenas o status de **implementação por revisão fundamentada** segundo a regra do projeto; mantenha **suporte nativo comprovado** como pendente.

## 6. Erros de design a evitar

- Não usar pacote `grok` não oficial como substituto do Grok Build da xAI.
- Não inserir `GROK_HOME` sem sanitizar overrides/auth/projetos de terceiros.
- Não chamar `grok agent stdio` como se fosse o mesmo contrato de `claude-agent-acp` externo.
- Não criar prova falsa de login, de Windows ARM64 ou de handshake ACP por mocks.
- Não desabilitar proteções do Job Object ou habilitar `--yolo` apenas para fazer o teste passar.
- Não declarar `GROK_CONFIG` equivalente a troca de `GROK_HOME`; overlay não troca storage e tem allowlist.
- Não transformar `run` em `exec.Command("cmd.exe", "/C", "grok", ...)`.
- Não copiar arquivo `auth.json` real para fixtures, documentação, reports, CI e ZIP.
- Não confiar na presença de `go`/`grok` no sandbox atual como sinal de ambiente de produção.

## 7. Estado final deste snapshot

**Histórico SNAPSHOT-001:** a investigação foi concluída sem código Grok. O SNAPSHOT-002 implementa o plano; consulte a seção de implementação/revalidação abaixo e `VALIDATION.md` para distinguir código concluído de gates externos ainda não executados.

---

# Implementação e revalidação — SNAPSHOT-002 (2026-09-26)

O plano acima foi executado no código. O provider `grok` agora é parte de `Tools`, com `GROK_HOME` por profile, TUI/headless por `run` e ACP nativo por `acp`.

## Decisões finais após revalidação

- **ACP:** o argv correto é `grok agent <opções-do-usuário> stdio`. A implementação inicial `agent stdio <args>` foi descartada porque opções de `agent` (`--model`, `--always-approve`, `--reauth`, `--agent-profile`, etc.) pertencem antes do transporte. `ACPTool` ganhou `SuffixArgs` genérico para expressar isso sem hardcode em CLI/service.
- **Permissões:** `ai-profile` não injeta `--always-approve`, `--yolo` nem `yoloMode`. Deny rules, hooks, remembered grants, folder trust e sandbox continuam sob as camadas nativas do Grok/cliente ACP.
- **Config inicial:** `config.toml` é criado antes do commit de `index.json`; falha de inicialização remove o diretório novo. Compatibilidade global Claude/Cursor é desativada explicitamente. Em `[compat.codex]`, somente `sessions=false` é usado porque os demais campos Codex estão documentados como reservados/inertes na revisão atual.
- **Config existente é do usuário:** `run/acp` não reescrevem config. Ausência, symlink ou objeto especial em `config.toml` faz o spawn falhar fechado.
- **Ambiente:** não existe limpeza ampla de `GROK_*`. O wrapper remove somente credenciais, external auth, deployment key, overlays de config, endpoint de chat e paths de logging que podem trocar identidade/destino do profile. Guardrails como `GROK_SANDBOX` e requisitos administrados são preservados.
- **Auto-update:** processos lançados pelo wrapper recebem `GROK_DISABLE_AUTOUPDATER=1`; profiles novos também nascem com `[cli] auto_update=false`. Isso evita que TUI/headless/ACP modifiquem a instalação durante uma execução controlada.
- **Windows/npm:** `grok.exe` real é preferido. Se `grok` resolver para `.cmd`, o resolver não usa `cmd.exe`: exige package instalado `@xai-official/grok`, exige que `package.json` mapeie `bin.grok` para `bin/grok`, exige entrypoint regular e executa esse arquivo por `node.exe`. O launcher oficial público `bin/grok` encaminha ao bootstrap nativo. Mudança upstream do manifesto falha fechada.
- **Store:** não houve bump de schema. `Profile.Tool` já é string e o terceiro provider não altera representação persistida.
- **ACL/permissões:** não foi adicionada caminhada recursiva específica Grok. O root `.ai-profiles` já é privado/protegido e herdável no Windows; reescrever ACLs/modos de toda árvore Grok poderia interferir com estado/plugins legítimos e não é necessário para provar a âncora `GROK_HOME`.

## Conflitos que permanecem externos ao adapter

1. **Contexto do projeto:** Grok pode consumir regras/projeto (`AGENTS.md`, `.grok`, e superfícies de compatibilidade conforme trust). Isso é contexto do cwd, não vazamento do `GROK_HOME`; o wrapper preserva cwd deliberadamente.
2. **Autenticação empresarial:** variáveis OIDC/provider herdadas são removidas para que um profile selecionado não mude silenciosamente de identidade. Uma organização que depende somente dessas variáveis deve configurá-las explicitamente no ambiente autorizado da execução/profile; não há cópia automática.
3. **Bootstrap npm e `GROK_HOME`:** o launcher npm pode materializar/selecionar binário sob o home do Grok. Como o child recebe o home isolado, isso pode gerar estado/binário por profile em instalações npm; é uma característica upstream a medir em Windows real. Não é contornado usando shell.
4. **Plataformas upstream:** o suporte de `ai-profile` e a disponibilidade do binário Grok são dimensões separadas. Não rotular Windows ARM64/macOS Intel como Grok nativo sem observar artifact oficial da versão usada.
5. **Versão espelho vs release:** `xai-org/grok-build` declara que é espelho periódico do monorepo. Código/documentação pública fundamentam a implementação, mas a compatibilidade final precisa registrar `grok --version` e a versão instalada.

## Critério de promoção

A implementação pode ser tratada como **tecnicamente completa por revisão** neste projeto. “Grok nativo validado” só deve ser promovido após execução do binário oficial em uma versão registrada, incluindo login/profile isolation, headless, ACP com cliente real e Windows x64 sem shell. Nenhum desses gates foi fabricado neste snapshot.
