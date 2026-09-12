# Auditoria histórica do baseline Nushell

> Documento histórico. Não descreve a implementação Go atual e não deve orientar UX ou arquitetura nova.

## Escopo auditado

Fonte: ZIP `nushell-config-main.zip` enviado pelo usuário.

Arquivos relevantes inspecionados:

- `CLAUDE.md`
- `rules.md`
- `tasks.md`
- `config.nu`
- `modules/ai_profiles/mod.nu`
- `modules/ai_profiles/README.md`
- `modules/ai_profiles/acp-integration.md`
- `modules/ai_profiles/known-issues.md`
- `modules/ai_profiles/future-ideas.md`
- `modules/repo_zip/*.nu`
- `modules/repo_zip/README.md`
- `docs/audits/2026-07-13-project-audit.md`
- `docs/plans/ai-profile-linux-support.md`
- `docs/plans/cross-platform-nushell-compat.md`

Não existe `AGENTS.md` na raiz do ZIP atual. O repo usa `CLAUDE.md` + `rules.md`; o novo monorepo adiciona `AGENTS.md` como ponto de entrada cross-agent.

---

## `ai-profile`: comportamento observado

### Comandos

```text
ai-profile <tool> list
ai-profile <tool> new <nome>
ai-profile <tool> rename <antigo> <novo>
ai-profile <tool> delete <nome>
ai-profile <tool> run <perfil> ...args
ai-profile <tool> acp <perfil> ...args
ai-profile <tool> apply-statusline <perfil> [template]
```

`list` é ação default quando a ação é omitida.

Tools atuais:

- `claude`
- `codex`

`agy` foi deliberadamente removido do suporte por risco de isolamento/keychain e **não deve voltar durante esta migração**.

### Estado atual

- root: `~/.ai-profiles`
- índice: `~/.ai-profiles/index.nuon`
- diretório físico tem ID opaco estável;
- alias é apenas rótulo no índice;
- rename altera alias, nunca move o diretório físico.

Essa invariável é crítica porque credenciais/configs podem depender do path absoluto.

### Especificação de tools

Claude:

- binário: `claude`
- config env: `CLAUDE_CONFIG_DIR`
- remove do ambiente herdado:
  - `CLAUDE_CODE_USE_BEDROCK`
  - `CLAUDE_CODE_USE_VERTEX`
  - `CLAUDE_CODE_USE_FOUNDRY`
  - `ANTHROPIC_API_KEY`
  - `ANTHROPIC_AUTH_TOKEN`
  - `CLAUDE_CODE_OAUTH_TOKEN`
  - `ANTHROPIC_BASE_URL`
- ACP: `claude-agent-acp`

Codex:

- binário: `codex`
- config env: `CODEX_HOME`
- remove `OPENAI_API_KEY`
- ACP: `codex-acp`
- herda `AGENTS.override.md` e `AGENTS.md` globais via symlink quando o perfil ainda não possui override próprio.

### ACP

O wrapper atual usa `exec` para substituir o processo Nushell.

Contrato essencial:

- stdin intacto;
- stdout reservado ao JSON-RPC do adapter;
- diagnóstico humano só em stderr;
- args preservados como argv;
- exit status/sinais devem ser propagados tão fielmente quanto a plataforma permitir.

### Problemas confirmados no código/docs atuais

1. `index.nuon` é ponto único de falha.
2. Escrita do índice não é atômica.
3. Não há lock em read-modify-write concorrente.
4. Não há recuperação automática de índice corrompido/órfãos.
5. `safe-remove` usa containment por prefixo textual com `/`, incompatível com Windows.
6. `apply-statusline` aceita Codex, embora o template padrão seja específico do Claude.
7. Guardas ACP verificam adapter, mas não necessariamente a CLI principal.
8. Linux/Windows têm status de isolamento de credencial não verificado.

### Pendências anteriores relevantes

A migração Go substitui/absorve as pendências de path seguro e standalone binary. Ela **não autoriza** marcar isolamento de credenciais Linux/Windows como validado sem teste empírico apropriado.

---

## `repo-zip`: comportamento observado

### Comando

```text
repo-zip [source]
  -o, --output <zip>
  -n, --name <nome>
  --git
  -s, --suffix <sufixo>
  -v, --version <versao>
  -f, --force
```

### Saída padrão

```text
<repo>/.tmp/repo-zip/<nome>.zip
```

### Seleção de arquivos

O código usa o próprio Git:

```text
git ls-files -z --cached --others --exclude-standard
```

Isso é uma decisão correta e deve ser preservada. Não reimplementar `.gitignore` em Go.

### Proteções atuais

- source precisa existir e ser diretório;
- resolve raiz via `git rev-parse --show-toplevel`;
- recusa submodule;
- recusa sparse checkout/skip-worktree;
- não inclui `.tmp/repo-zip/`;
- saída explícita dentro do repo é excluída do snapshot;
- recusa sobrescrever arquivo tracked;
- recusa saída dentro de `.git`;
- recusa destino existente sem `--force`;
- recusa destino que não seja arquivo comum;
- preserva symlink (`zip -y`);
- temporário no mesmo diretório do destino;
- valida ZIP antes de publicar;
- `--git` inclui `.git` e exige `.git` como diretório normal;
- `--git` recusa alternates externos;
- `--git` checa locks Git conhecidos;
- `--git` compara HEAD/status antes/depois.

### Dependências runtime atuais

- Nushell
- `git`
- `zip`
- `unzip`

**Decisão histórica supersedida:** a implementação Go final não mantém esse backend. Criação e verificação usam `archive/zip`; `zip`/`unzip` externos não fazem parte do runtime.

### Limitação importante do snapshot atual

A comparação de `HEAD + status porcelain` antes/depois detecta várias mudanças de estado Git, mas não é uma transação de filesystem. A migração deve documentar claramente o nível de consistência e pode fortalecer a verificação sem alegar atomicidade impossível.

---

## Integração atual com Nushell

`config.nu` importa diretamente:

```nu
use modules/ai_profiles [ai-profile]
use modules/repo_zip [repo-zip]
```

No cutover, esses imports devem ser removidos somente depois que os binários Go estiverem instalados e validados no PATH. **Decisão posterior:** completions são geradas pelo próprio binário para Bash/Fish/Zsh; não existe camada Nushell obrigatória.

---

## O que é observado vs. não verificado

### Observado no código/documentação

- contratos acima;
- riscos do índice;
- path containment atual;
- dependências de `repo-zip`;
- regras de ACP;
- support matrix declarada.

### Não deve ser “reprovado” durante a migração usando contas reais

- comportamento real de Keychain/Secret Service/Credential Manager;
- login Claude/Codex;
- validade de uma conta real;
- isolamento real de credencial em Linux/Windows.

Esses pontos ficam fora da suíte automática e devem permanecer com nível de confiança explícito.

---

## Complemento de paridade descoberto na revisão V3

### `ai-profile` — detalhes que também fazem parte da substituição

Além dos comandos, o módulo atual oferece comportamento interativo que não pode ser perdido sem decisão explícita:

- `list` produz registros estruturados no Nushell com `profile`, `env`, `dir`, ordenados por alias;
- completers para tools, actions, profiles e templates;
- `new`, `rename` e `delete` retornam/informam o diretório do profile;
- delete exige duas confirmações (`alias` exato + `y/yes`);
- `run` herda env normal, remove apenas as credenciais declaradas e repassa argv literal;
- ACP usa `exec` no Nushell atual e reserva stdout ao JSON-RPC;
- guidance Codex é sincronizado em todo `run/acp`, não apenas em `new`;
- `apply-statusline` é manual e aceita qualquer tool registrada no dispatcher atual;
- templates são extensíveis por arquivo no módulo atual;
- `agy` continua fora por decisão explícita de segurança.

Como executável standalone não devolve automaticamente valores tipados ao pipeline Nushell, a implementação oferece `list --json` como interface machine-readable. **Decisão posterior:** não há integração Nushell obrigatória para completions/estrutura; o shell não faz parte do contrato.

### Suporte atual por SO

O módulo `platform/require-runtime` trata `untested` como bloqueio, não como sucesso.

`ai-profile run/acp`:

```text
macOS   supported
Linux   untested
Windows untested
```

`repo-zip`:

```text
macOS   supported
Linux   unsupported
Windows unsupported
```

O `repo-zip` macOS atual requer `git`, `/usr/bin/zip` e `unzip`, usando `unzip -tqq` para verificação.

**Decisão histórica supersedida:** a implementação Go usa `archive/zip` em macOS/Linux. Apenas capacidades realmente específicas de SO ficam separadas; Windows permanece compile-only nas capabilities ainda não implementadas.

### Referências adicionais congeladas

A V3 adiciona à referência histórica:

- `ai_profiles/decision-summary.md` (sanitizado, sem contexto pessoal);
- `platform/mod.nu`.

Esses arquivos contêm decisões de compatibilidade/suporte que não devem depender da memória do agente.
