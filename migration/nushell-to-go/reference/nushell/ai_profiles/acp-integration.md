# Integração ACP no `ai-profile`

Como expor cada CLI também como **agente ACP** (Agent Client Protocol),
isolado por perfil, declarando por tool qual ACP ele tem e como lançar.

Status: **lançador implementado** (`ai-profile <tool> acp <perfil>`).
Verificado: os binários `claude-agent-acp` e `codex-acp` existem no PATH;
caminhos de erro testados. **Não testado** ponta-a-ponta com um cliente
ACP real conectando (o adapter é um servidor stdio de vida longa, não dá
pra exercitar num teste de linha única). O registry tool×profile no app
cliente continua planejado.

## Ideia central

Lançar um agente ACP isolado é o MESMO mecanismo do `run` — aplicar o
isolamento (env var / `$HOME`) e spawnar um binário — só que o binário é o
comando ACP (stdio) em vez do interativo. Então cai como uma ação nova
`acp` no mesmo `ai-profile`.

## Configuração: declarar que um tool TEM ACP e QUAL usar

Cada entrada do `TOOLS` ganha um campo **opcional** `acp`. Se presente, o
tool suporta ACP e o campo diz como lançá-lo. Se ausente, `ai-profile
<tool> acp ...` deve dar erro claro ("<tool> não tem ACP configurado").

Formato proposto (bin + args, porque cada CLI expõe de um jeito):

```nushell
const TOOLS = [
    {
        name: "claude"
        bin: "claude"
        config_env: "CLAUDE_CONFIG_DIR"
        clear_env: [ ... ]
        # ACP via adapter externo (binário separado que embrulha o claude):
        acp: { bin: "claude-agent-acp", args: [] }
    }
    {
        name: "codex"
        bin: "codex"
        config_env: "CODEX_HOME"
        clear_env: ["OPENAI_API_KEY"]
        # codex NÃO tem subcomando `codex acp` (verificado: só tem mcp,
        # mcp-server, app-server, remote-control). O ACP é um binário
        # SEPARADO `codex-acp`, que respeita CODEX_HOME.
        acp: { bin: "codex-acp", args: [] }
    }
]
```

(`agy` não está em `TOOLS` — removido do módulo, ver `agy-keychain-issue.md`.)

"Dizer que um tem e deve usar determinado ACP" = é exatamente o campo
`acp`: a presença declara que tem; o `{bin, args}` declara qual comando é o
ACP daquele tool. Fonte única de verdade — quem suporta ACP é derivado do
array, nunca hardcoded em outro lugar (mesmo princípio do `isAcpAgentId`
derivado do registry, do skill de ACP).

## Lançador stdout-limpo (resultado da investigação)

ACP fala JSON-RPC por **stdout**. Qualquer byte extra corrompe o stream e o
agente trava silenciosamente. A investigação no Nushell 0.110 concluiu:

- Usar o builtin **`exec`** (não `run-external`): substitui o processo nu
  pelo binário → stdin/stdout intocados, sinais entregues corretamente
  (SIGTERM "trappável" = shutdown limpo), exit code propagado.
- Diagnóstico humano vai com **`print -e`** (stderr). `print` normal vai
  pra stdout e corromperia o protocolo.
- Tudo antes do `exec` (validação, `print -e`); código depois do `exec` é
  inalcançável.
- `with-env { exec $bin ...$args }` aplica isolamento E preserva stdio.
- Passar args como lista espalhada (`...$args`) evita o parsing de flags
  com `-` do nu.

Esboço:

```nushell
def acp-tool-profile [tool: string, profile: string, args: list<string>] {
    let spec = (tool-spec $tool)
    if ($spec.acp? | is-empty) {
        error make { msg: $"($tool) não tem ACP configurado" }
    }
    let dir = (existing-profile-dir $tool $profile)
    let overrides = ($spec.clear_env | reduce --fold {($spec.config_env): $dir} {|e, acc| $acc | insert $e null})

    print -e $"acp: ($spec.acp.bin) profile=($profile)"   # stderr — nunca stdout
    with-env $overrides {
        exec $spec.acp.bin ...$spec.acp.args ...$args
    }
}
```

E no `match $action` do `ai-profile`, um caso `"acp"` chamando isso.

## Registry tool × profile no app cliente (saber qual usar / qual está em uso)

No app que consome ACP (`@agentclientprotocol/sdk`), o registry deixa de
ser "Claude, Codex" e passa a ser **tool × profile**:

- id do agente codifica o par, ex: `claude:perfil-b`, `codex:work`.
- `primary` de cada definição = `["ai-profile", "<tool>", "acp", "<profile>"]`.
- A lista pode ser **dinâmica**: o app chama `ai-profile <tool> list` por
  tool que tenha `acp` e monta o cross-product → perfil novo aparece
  sozinho como agente.
- "Qual está em uso": como a sessão ACP está amarrada ao agente conectado,
  e o id codifica `tool:profile`, o próprio UI mostra qual. Pode rotular a
  sessão "Claude · perfil-b".

## Como um app .ts (cliente ACP) chama isto — e por que o shell padrão NÃO importa

Equívoco comum: "o shell padrão precisa ser nushell?". **Não.** Um cliente
ACP (via `@agentclientprotocol/sdk`) **spawna o binário do agente
diretamente** — `Deno.Command(cmd, {args, env})` / `Bun.spawn(cmd, {...})`
/ `child_process.spawn(cmd, args, {env})`. Nenhum desses passa pelo shell
de login: eles fazem exec do binário direto (PATH), com `env` explícito. Se
o teu shell padrão é fish, é **irrelevante** pro spawn.

### O ponto importante: pra ACP você nem precisa do `ai-profile`/nushell

Pra os tools que têm ACP (claude, codex), o isolamento é **só uma env var**
(`CLAUDE_CONFIG_DIR` / `CODEX_HOME`), mantendo `$HOME` real. E o registry
ACP já tem um campo `env` por agente (ver guia de cliente ACP, §7/§9). Então
o jeito mais limpo é o app **setar a env var direto no spawn** e chamar o
adapter direto — sem nushell, sem shell, sem wrapper:

```ts
// registry dinâmico: um agente por (tool, profile)
{ id: "claude:perfil-b", command: "claude-agent-acp", args: [],
  env: { CLAUDE_CONFIG_DIR: "/Users/.../.ai-profiles/claude-<id>" } }
{ id: "codex:work", command: "codex-acp", args: [],
  env: { CODEX_HOME: "/Users/.../.ai-profiles/codex-<id>" } }
```

Pra isso o app só precisa do **mapeamento perfil → diretório** (+ as env de
auth a limpar, do `clear_env`). Como dar isso ao app, sem ele ter que
parsear `.nuon`:

- **Opção 1 (recomendada):** um comando que emite o env de um perfil em
  JSON, ex. `ai-profile <tool> acp-env <perfil>` → imprime
  `{ "CLAUDE_CONFIG_DIR": "...", "ANTHROPIC_API_KEY": null, ... }`. O app
  chama uma vez por perfil, aplica no `env` do spawn (setando o dir e
  removendo as chaves `null` do env herdado), e spawna o adapter direto.
  Zero nushell em runtime de sessão; nenhuma duplicação da lista
  `clear_env`.
- **Opção 2:** índice também em JSON (hoje é `.nuon`) pro app ler direto.
- **Opção 3 (funciona, mas feio):** o app spawna
  `nu -c "use <modpath>; ai-profile claude acp perfil-b"`. Funciona
  independente do shell de login (você invoca `nu` explícito), mas acopla
  o app ao nushell + caminho do módulo.

O comando `ai-profile <tool> acp <perfil>` (já implementado) continua útil
pra **lançar ACP interativamente do prompt nu**; pro app .ts, prefira a
Opção 1 (env no spawn).

## Vale reescrever o `ai-profile` em Rust?

Pra ACP: **não é necessário.** O caminho ACP é resolvido com env-no-spawn
(acima) — não precisa nem do nushell nem de um binário.

Reescrever em Rust (binário standalone no PATH) só compensa se você quiser
o `ai-profile` como **CLI universal** chamável identicamente de qualquer
contexto (shell, app, cron) sem depender de nushell. Tradeoffs:

- ✅ `spawn("ai-profile", [...])` direto, sem `nu -c`, sem dep de nushell,
  startup rápido, cross-platform.
- ❌ Trabalho de reescrita; perde os completers nativos do nushell (teria
  que reimplementar completion à parte); vira artefato compilado a manter;
  migração do índice `.nuon` → JSON (Rust não lê nuon).
- Veredito: **overkill agora.** O módulo nushell é ótimo pro uso
  interativo (completion), e a env-no-spawn resolve o uso programático do
  ACP. Rust fica como opção futura se o `ai-profile` virar peça central
  chamada por muitos programas não-nu.

## Fase 0 (já verificada)

1. ✅ Comando ACP real de cada tool: **codex NÃO tem `codex acp`** — é o
   binário separado **`codex-acp`** (`~/.bun/bin/codex-acp`). claude usa
   **`claude-agent-acp`** (`mise shims`). Ambos confirmados no PATH.
2. Confirmar que o adapter de claude respeita `CLAUDE_CONFIG_DIR` (deve,
   pois usa o claude por baixo) — **a verificar com cliente real**.

## Decisões pendentes

- Registry dinâmico (tool×profile automático) vs estático.
- Opção 1 (`acp-env` em JSON) vs Opção 3 (`nu -c`) pro app .ts consumir.
- agy fica de fora do ACP (sem adapter ACP conhecido + dilema de
  credencial do `agy-keychain-issue.md`).
