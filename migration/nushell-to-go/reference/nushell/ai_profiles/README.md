# ai-profile

Contas isoladas entre identidades distintas para CLIs de
IA, sem conflito de sessão. Um único comando cobre qualquer CLI configurada
em `TOOLS` dentro de `mod.nu` (hoje: `claude`, `codex`).

`agy` (Antigravity/Google) não é suportado por este módulo — a credencial
dele vive num item global do Keychain, não por config dir, então isolar
por `$HOME` não dá multi-conta de verdade e quebra o keychain padrão.
Use o `agy` direto, sem perfil. Detalhes em `agy-keychain-issue.md`.

## Uso

```
ai-profile <tool> list
ai-profile <tool> new <nome>
ai-profile <tool> rename <nome-antigo> <nome-novo>
ai-profile <tool> delete <nome>
ai-profile <tool> run <nome> ...args
ai-profile <tool> acp <nome> ...args   # lança o agente ACP isolado (só tools com ACP)
ai-profile <tool> apply-statusline <nome> [template]   # ver seção "statusLine" abaixo
```

`list` é o padrão se você omitir a ação (`ai-profile claude` == `ai-profile
claude list`). A saída inclui o nome do perfil, a variável de ambiente usada
pela CLI e o diretório de configuração correspondente. Por exemplo,
`ai-profile codex list` mostra o `CODEX_HOME` de cada perfil; assim esse mesmo
perfil pode ser selecionado em outro projeto configurando `CODEX_HOME` com o
valor exibido na coluna `dir`.

## Exemplos

```
ai-profile claude new perfil-a
ai-profile claude run perfil-a
ai-profile claude rename perfil-a perfil-b
ai-profile claude acp perfil-b           # servidor ACP do claude, perfil perfil-b
ai-profile codex acp work              # servidor ACP do codex, perfil work
```

Exemplo de uso direto de um perfil listado, sem passar por `ai-profile run`:

```
with-env { CODEX_HOME: "/caminho/exibido/em/dir" } { codex }
```

## ACP

`ai-profile <tool> acp <nome>` lança o adapter ACP daquele tool com o
mesmo isolamento por perfil, falando JSON-RPC por stdio (pra ser spawnado
por um cliente ACP). Só funciona em tools que declaram o campo `acp` no
`TOOLS` — hoje `claude` (via `claude-agent-acp`) e `codex` (via
`codex-acp`). Detalhes e o registry tool×profile no app cliente:
`acp-integration.md`.

Na primeira vez que você roda `ai-profile <tool> run <nome>`, a CLI não tem
login ainda — faça o login normal dela dentro dessa sessão (ex: `/login`
no Claude) escolhendo a conta certa. Fica salvo isolado daquele perfil.

## Instruções globais do Codex

O Codex procura `AGENTS.override.md` ou `AGENTS.md` dentro do `CODEX_HOME`.
Como cada conta usa um `CODEX_HOME` isolado, o módulo cria automaticamente
links simbólicos no perfil para os arquivos existentes em `~/.codex` ao
executar `run` ou `acp`. Assim, o `auth.json` continua separado, enquanto as
instruções globais continuam iguais às de uma execução normal do Codex.

Um `AGENTS.md` ou `AGENTS.override.md` já existente no próprio perfil nunca
é substituído. Isso permite instruções específicas por conta: remova o link
e crie um arquivo normal no mesmo caminho. O Codex monta a cadeia de
instruções no início da sessão, portanto reinicie a sessão depois de alterar
esses arquivos.

## statusLine

Cada perfil tem seu próprio `settings.json` isolado (é o que `CLAUDE_CONFIG_DIR`/
`CODEX_HOME` aponta pra ele). Por isso a chave `statusLine` do seu
`settings.json` global (`~/.claude/settings.json`) **não** aparece quando
você roda `ai-profile claude run <nome>` — o perfil nunca lê o global.

`apply-statusline` resolve isso sem quebrar o isolamento: mescla só a chave
`statusLine` (lida de um template em `statusline-templates/<nome>.json`,
dentro deste módulo) no `settings.json` do perfil, mantendo todo o resto
(`model`, `enabledPlugins`, `theme`...) intocado. É manual e opt-in — nunca
roda em `new`/`run`/`acp`, só quando você chama:

```
ai-profile claude apply-statusline perfil-b            # usa statusline-templates/default.json
ai-profile claude apply-statusline trabalho detalhado # usa statusline-templates/detalhado.json
```

Templates diferentes permitem statuslines diferentes por perfil — crie um
`.json` novo em `statusline-templates/` (mesmo formato da chave `statusLine`
do `settings.json`) e use o nome do arquivo (sem `.json`) como argumento.

## O que não fazer

- Não mova/renomeie a pasta de um perfil manualmente (`~/.ai-profiles/...`).
  Use `ai-profile <tool> rename`, que só troca o rótulo sem tocar na pasta
  — é assim que o login não se perde (detalhes em `adr.md`).
- `ai-profile <tool> delete <nome>` apaga a pasta **permanentemente** (não
  vai pra lixeira). Confirme o nome antes de aceitar.

## Adicionar uma CLI nova

Uma entrada em `const TOOLS` (em `mod.nu`) é suficiente — nenhum comando
novo precisa ser escrito. Veja o comentário acima do array pra cada campo.
