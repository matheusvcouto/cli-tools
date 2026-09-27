# ADR — `ai-profile`

Decisões técnicas vigentes da CLI `ai-profile`. Decisões compartilhadas pela
suíte permanecem no [`ADR.md` da raiz](../../ADR.md).

## AP001 — Diretório físico é a identidade estável do profile — Accepted

Alias é apenas um rótulo. Rename nunca move o diretório físico, porque
credenciais e ferramentas nativas podem vincular estado ao caminho absoluto.
O índice JSON é versionado; mutações usam lock, backup, commit confinado e
rollback quando há mais de um efeito persistente. A criação inicial do arquivo
de lock usa create-exclusive; concorrentes validam o arquivo já criado antes de
abri-lo e sua identidade é revalidada antes de adquirir o lock.

## AP002 — Cada profile possui todos os roots nativos da ferramenta — Accepted

Codex recebe `CODEX_HOME=<profile-dir>`.

Claude recebe dois roots distintos e confinados ao mesmo profile:

```text
CLAUDE_CONFIG_DIR=<profile-dir>
ANTHROPIC_CONFIG_DIR=<profile-dir>/.anthropic
```

`CLAUDE_CONFIG_DIR` contém configuração, login, contexto e estado próprios do
Claude Code. `ANTHROPIC_CONFIG_DIR` isola profiles Anthropic e Workload Identity
Federation que Claude Code também consulta. O subdiretório `.anthropic` deve
ser um diretório real; symlink ou outro tipo de objeto é erro antes do spawn.

## AP003 — Overrides herdados não podem substituir o profile selecionado — Accepted

Antes de `run` ou `acp`, o wrapper remove variáveis fixas da ferramenta que
selecionam credencial, federation, provider, endpoint ou um root alternativo.
Isso inclui as superfícies públicas atuais de workload identity do Codex e do
Claude. A variável é removida do ambiente; nunca é mantida com valor vazio.

Credenciais genéricas de projeto, como `AWS_PROFILE` e credenciais GCP/Azure,
são preservadas. Elas podem ser necessárias aos comandos executados pelo
agente e só selecionam um provider Claude quando a configuração própria do
profile o habilita. Nomes arbitrários apontados por `env_key` em uma
configuração de provider também são intencionais e não podem ser descobertos
ou removidos genericamente pelo wrapper.

## AP004 — Contexto global pertence ao profile; contexto de projeto é nativo — Accepted

O wrapper preserva o cwd. Codex lê `AGENTS.override.md` ou `AGENTS.md` dentro
do `CODEX_HOME` selecionado e continua descobrindo a hierarquia `AGENTS.md` do
projeto. Nenhum guidance é copiado ou linkado do `~/.codex` padrão.

Claude mantém `CLAUDE.md`, `rules/`, `skills/` e `agents/` dentro do profile e
continua descobrindo contexto do projeto pelo cwd. Profiles não default também
recebem `claudeMdExcludes` para os arquivos de memória/rules do `~/.claude`
padrão; managed policy e configuração de projeto continuam válidas.

## AP005 — `run` e `acp` substituem o processo em Unix — Accepted

Argumentos são argv literal, sem shell intermediário. Stdin, stdout, stderr e
exit status pertencem ao processo final. Em ACP, o wrapper não escreve em
stdout porque o stream é reservado ao protocolo. Plataforma sem primitiva
equivalente segura retorna erro explícito.

## AP006 — Statusline integrada foi retirada da superfície ativa — Retired

`apply-statusline` não faz parte da CLI, do domínio nem dos artifacts ativos.
A implementação anterior permanece somente na referência histórica da migração
Nushell para eventual consulta; reintrodução futura exige nova decisão e testes.

## AP007 — Migração NUON é transitória — Accepted

`tools/migrate-ai-profile-index` lê somente o schema legado necessário, nunca
executa Nushell e não entra nos artifacts. Após o cutover autorizado, a
ferramenta e os documentos da migração devem ser arquivados juntos.

## AP008 — Superfície de CLI deriva do CLI Core — Accepted

Comandos Claude/Codex/Grok, help, schema, contract e completions derivam de uma única
Spec compilada. `run/acp` usam argumento trailing `opaque`, portanto opções do
processo filho nunca são reinterpretadas pelo wrapper. Exclusão usa a policy de
`Interaction` do core: stdin não interativo é recusado antes da mutação.

## AP009 — Grok Build usa `GROK_HOME` isolado e ACP nativo — Accepted

Grok Build é provider de primeira classe. Cada profile usa `GROK_HOME=<profile-dir>`
e nasce com `config.toml` regular, criado antes do commit do índice. A configuração
inicial desativa importação global Claude/Cursor e sessões Codex compatíveis; o
wrapper nunca copia credenciais do `~/.grok` padrão e nunca recria silenciosamente
um `config.toml` ausente em profile existente.

Overrides herdados que podem trocar autenticação, endpoint, overlay de config ou
path de log são removidos. Variáveis de política/segurança não são apagadas em
massa: sandbox, requisitos administrados e demais guardrails continuam valendo.
Auto-update é desligado durante processos lançados pelo wrapper para tornar TUI,
headless e ACP reprodutíveis.

ACP usa o servidor nativo do Grok, com argv `grok agent <args-do-usuário> stdio`.
O sufixo é obrigatório porque as opções de `agent` precedem o transporte. O
wrapper não ativa `--always-approve`/`--yolo`; autorização permanece decisão do
cliente/usuário e das políticas do Grok.

No Windows, shims npm não passam por `cmd.exe`. O resolver aceita o package
`@xai-official/grok` somente quando o manifesto instalado confirma o entrypoint
esperado `bin/grok`, e então o executa por `node.exe`. Mudança upstream do
manifesto falha fechada em vez de reinterpretar argumentos por shell.


## AP010 — Grok: separar ambiente de identidade de requisitos administrativos — Accepted (2026-09-26)

O provider mantém `GROK_HOME` exclusivo, remove `XAI_API_KEY`, seletores de
autenticação, endpoint e compatibilidade herdados e nunca reescreve um
`config.toml` existente. Diferentemente do SNAPSHOT-002, o wrapper **não**
força compatibilidade `false` pelo ambiente: o TOML inicial a desativa e
edições explícitas do usuário podem habilitá-la sem colisão com o shell.
`[compat.codex]` inicial também desativa `skills` e `hooks`, presentes na
referência oficial atual.

Restrições externas `GROK_DISABLE_API_KEY_AUTH`, `GROK_FORCE_LOGIN_TEAM_ID`,
`GROK_SANDBOX` e `GROK_REQUIRED_*` continuam herdadas. Não copiar credenciais
de shells externos nem desabilitar políticas para obter login no sandbox.
`os.Root` + comparação de identidade da âncora `config.toml` reduz ataques
por troca de symlink durante o preflight; isto não equivale a proteger contra
modificações posteriores do mesmo usuário. Execuções reais permanecem gates
separados. Ver `plans/grok-build/CODE_REVIEW_003.md`.
