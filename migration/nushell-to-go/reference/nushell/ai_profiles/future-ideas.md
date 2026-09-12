# Ideias futuras — `ai-profile`

Caderno de ideias e explorações que NÃO estão implementadas. Nada aqui é
decisão tomada; é material pra quando/se quiser evoluir. Sinta-se à vontade
pra adicionar mais ideias no fim.

Para o que JÁ está decidido/implementado, ver `adr.md`, `README.md`,
`acp-integration.md`. Para problemas conhecidos, `known-issues.md`.

---

## 1. Overlay transparente estilo `mise` (shims)

**Origem:** observação de que o `mise` (escrito em Rust) dá um overlay
transparente — você digita `node`/`codex` e ele resolve versão+env e faz
`exec` do binário real. Na máquina atual o `codex` já passa por um shim do
mise (`~/.local/share/mise/shims/codex`, binário compilado); `claude` e
`agy` não.

**Como seria pro `ai-profile`:** um **shim** (executável no PATH, com o nome
`claude`/`codex`/`agy`, colocado ANTES do binário real) que:
1. descobre o **perfil ativo** (ver "como escolher" abaixo),
2. injeta a env de isolamento (`CLAUDE_CONFIG_DIR` / `CODEX_HOME` / `HOME`),
3. faz `exec` do binário real.

Resultado: você digita `claude` normal e ele usa o perfil certo — o
"acessar como acessaria normalmente" de verdade, sem `ai-profile run`.

**Rust é obrigatório?** Não. Um shim pode ser um script `sh` de poucas
linhas. Rust só compensa se quiser robusto/rápido/cross-platform como o
mise.

### A pergunta difícil: como o shim escolhe o perfil?
(o mise resolve por diretório via `.tool-versions`; pra conta/perfil as
opções têm tradeoffs)

- **Env var `$AI_PROFILE`** — por sessão/comando, **concorrente**,
  transparente dentro da sessão. Melhor fit pro caso "um segundo usuário usa a máquina às
  vezes" (ela abre um terminal, `export AI_PROFILE=perfil-a`, e tudo ali usa a
  conta dela; concorrente com a sessão principal).
- **Arquivo `.ai-profile` por diretório** (estilo mise) — ótimo pra "este
  projeto sempre usa a conta X". Transparente + concorrente.
- **Perfil ativo global** (um arquivo de estado) — modelo "switch", **não
  concorrente** (uma conta por vez). Mais simples, menos potente.

Dá pra combinar: `.ai-profile` do diretório > `$AI_PROFILE` da sessão >
default.

### Custos / riscos (que o `ai-profile run` explícito não tem)
- **Blast radius maior:** o shim sombreia o `claude` real; um bug nele
  quebra o `claude` em todo lugar, não só num comando explícito.
- **Encadeamento com o mise:** `codex` JÁ é um shim do mise — o nosso teria
  que vir antes e chamar o do mise (shim sobre shim), ordenação de PATH
  frágil.
- **agy fora de escopo:** removido do `TOOLS` (ver `agy-keychain-issue.md`),
  então um shim cobriria só `claude`/`codex`.
- **Não ajuda no ACP via app .ts:** lá o app seta o env no spawn direto
  (ver `acp-integration.md`); shim é pra uso **interativo** no terminal.

### Recomendação registrada
Não substituir o `ai-profile run` por shims — **somar** depois, como camada
opcional. Base = `ai-profile <tool> run/acp` (explícito, baixo risco, já
pronto); camada transparente = shim dirigido por `$AI_PROFILE`, só pros
tools que valem, sabendo do blast radius e do encadeamento com o mise.

---

## 2. Resiliência do índice (`~/.ai-profiles/index.nuon`)

Hoje é ponto único de falha sem recuperação (ver `known-issues.md` #1).
Ideias:
- **Escrita atômica** (arquivo temp + `mv`) pra não corromper em escrita
  interrompida.
- **`.bak`** do índice a cada escrita (o próprio codex faz isso com
  `.codex-global-state.json.bak`).
- **`ai-profile doctor`** — varre `~/.ai-profiles/` e mostra/re-adota pastas
  órfãs (dirs que existem mas não estão no índice).
- **`ai-profile <tool> adopt <dir>`** — registra uma pasta órfã no índice
  com um alias.

---

## 3. `acp-env` pra apps .ts consumirem (sem parsear `.nuon`)

`ai-profile <tool> acp-env <perfil>` → imprime JSON com o env de isolamento
(`{ "CLAUDE_CONFIG_DIR": "...", "ANTHROPIC_API_KEY": null, ... }`). O app
cliente ACP lê uma vez por perfil, aplica no `env` do spawn e chama o
adapter direto — sem nushell em runtime, sem duplicar a lista `clear_env`.
Detalhes em `acp-integration.md` (Opção 1).

---

## 4. Índice em JSON (em vez de `.nuon`)

Tornaria o índice legível por qualquer programa (apps .ts, um futuro
binário Rust) sem depender do nushell. Custo: migração do formato atual.
Combina com a ideia 3 e com um eventual binário standalone.

---

## 5. `ai-profile` como binário standalone (Rust/Go)

Só vale se virar **CLI universal** chamada por muitos programas não-nu
(spawn direto `ai-profile ...`, sem `nu -c`, cross-platform). Hoje é
overkill — ver análise em `acp-integration.md` ("Vale reescrever em
Rust?"). Pré-requisito natural: índice em JSON (ideia 4).

---

## 6. agy multi-conta robusta (Tipo C: keychain por perfil) — DESCARTADA

Considerada e rejeitada em 2026-06-23: o `agy` foi removido do `TOOLS`
em vez disso (zero risco de keychain > multi-conta). Ficaria assim só se
multi-conta concorrente do `agy` virar requisito firme de novo. Detalhes
do mecanismo (keychain próprio por perfil, macOS-only, exigiria validação
com 2 logins reais) e a ironia "load-bearing" em `agy-keychain-issue.md`.

---

## Mais ideias
(adicionar aqui)
