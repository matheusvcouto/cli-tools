# ADR — `ai-profile`

Decisões técnicas vigentes da CLI `ai-profile`. Decisões compartilhadas pela
suíte permanecem no [`ADR.md` da raiz](../../ADR.md).

## AP001 — Diretório físico é a identidade estável do profile — Accepted

Alias é apenas um rótulo. Rename nunca move o diretório físico, porque
credenciais e ferramentas nativas podem vincular estado ao caminho absoluto.
O índice JSON é versionado; mutações usam lock, backup, commit confinado e
rollback quando há mais de um efeito persistente.

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

## AP006 — Statusline é mutação Claude-only e confinada — Accepted

`apply-statusline` aceita somente profiles Claude, mescla apenas `statusLine`
em `settings.json`, preserva as demais chaves e usa commit dentro da safety
root já validada do profile.

## AP007 — Migração NUON é transitória — Accepted

`tools/migrate-ai-profile-index` lê somente o schema legado necessário, nunca
executa Nushell e não entra nos artifacts. Após o cutover autorizado, a
ferramenta e os documentos da migração devem ser arquivados juntos.
