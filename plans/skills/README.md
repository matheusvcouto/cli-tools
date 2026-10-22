# skills (proposta — não implementado)

> Prévia do README da futura CLI, para validar o contrato antes do
> planejamento. Plano de implementação e regras de segurança em [`PLANO.md`](PLANO.md).
> Comparação com o `skills` da Vercel no fim do arquivo; erros dela, achados nos
> seus repos e regras de engenharia em [`ANALISE.md`](ANALISE.md).
>
> **Já decidido:** Go, em `cmd/skills/` do cli-tools, usando o CLI Core;
> binário `skills` (o mesmo nome do `npx skills`, que você não instala globalmente;
> `skl` pode virar um alias depois); host padrão Claude + Codex; layout
> `skills/<categoria>/<nome>/` aceito nos repos de skills (opcional; o layout atual na raiz continua valendo); config, estado e backups em
> `~/.cli-tools/skills/`; índice automático dos destinos usados (`status --all`);
> dependências: `go.yaml.in/yaml/v3` para o frontmatter e lista com toggle própria
> sobre `golang.org/x/term` (já no go.mod), ambas registradas em ADR. O `skill-sync` em Deno do repo
> `skills` continua lá, mas **não** serve de base nem de dependência.

Instala, atualiza e empacota skills de agentes (Claude Code, Codex) a partir
dos seus repositórios de skills locais — por exemplo, um público e um privado.

## Ideia em uma frase

Você declara uma vez quais repos de skills existem na máquina. O `skills`
indexa tudo (inclusive por categorias), compara **versão + conteúdo** com o
destino (global ou um projeto) e mostra uma lista com toggle do que é novo,
do que precisa atualizar e do que foi alterado à mão.

## Uso rápido

```sh
skills list                         # todas as skills, agrupadas por categoria
skills status --global              # instalado × disponível no global
skills update --global              # lista com toggle só do que mudou
skills install --project ~/code/app # escolher o que instalar num projeto
skills build                        # gera .skill (zip) das escolhidas
```

Sem nomes, `install`, `update`, `uninstall` e `build` abrem a lista
(multi-select). Com nomes ou `--all`, não perguntam nada:

```sh
skills install acp-integration atividade-mapa --global
skills install marketing/ --global   # categoria inteira
skills update --global --all --yes
```

## Exemplo de tela

```text
$ skills update --global
Destino: claude ~/.claude/skills · codex ~/.agents/skills

  publico
  [x] dev/acp-integration            3.1.1 → 3.3.0   update
  [ ] dev/js-distribution-beta       0.4.0           modificado no destino ⚠
  privado
  [x] estudo/atividade-mapa          2.5   → 2.5     conteúdo mudou
      marketing/book-traffic-secrets 1.0.0           atualizado

↑/↓ mover · espaço marcar · a todos · / filtrar · enter confirmar
```

## Organização dos repos de skills

Mesmo layout que o ecossistema `skills` (Vercel) usa, para que o repo público
também possa ser instalado por terceiros com `npx skills add matheusvcouto/skills`:

```text
skills/                       # repo público  (github.com/matheusvcouto/skills)
├── skills.toml               # opcional
└── skills/
    ├── dev/
    │   ├── acp-integration/SKILL.md
    │   └── js-distribution-github-releases-beta/SKILL.md
    └── estudo/
        └── atividade-mapa/SKILL.md

private-skills/               # repo privado — mesmo layout
└── skills/
    └── marketing/
        ├── copywriting/
        │   └── vtsd-premissas/SKILL.md
        └── livros/
            └── traffic-secrets/SKILL.md
```

Categorias são **opcionais**. O layout de hoje (`<nome>/SKILL.md` na raiz do
repo) funciona sem mudar nada; dá para passar para `skills/<categoria>/` quando
quiser, e o `skills` não perde o rastreio das instaladas (o lock usa o nome).

Regras de descoberta:

- `SKILL.md` direto na raiz do repo, em `<nome>/SKILL.md` (layout atual);

- procura `SKILL.md` em `skills/` até **3 níveis** (`<nome>`, `<cat>/<nome>`,
  `<cat>/<subcat>/<nome>`);
- um `SKILL.md` encontrado num nível mais raso esconde o que está abaixo dele
  (a pasta de uma skill pode ter `examples/`, `tests/` etc. sem gerar falsas skills);
- **categoria** = caminho entre `skills/` e a pasta da skill (`marketing/livros`).
  Serve para agrupar, filtrar e selecionar; não vai para o destino;
- no destino a skill sempre fica **plana**: `<destino>/skills/<name>/`, porque
  é assim que Claude e Codex carregam;
- `name` do frontmatter é a identidade e deve ser **único entre todos os repos**;
  duplicado é erro (sem prioridade implícita).

Público × privado são **repos git diferentes** — o git não deixa uma pasta de um
repo público ser privada. A ferramenta trata os dois igualmente.

## Onde ficam as origens

`~/.cli-tools/skills/config.toml` (pasta central da suíte, `internal/toolhome`; criado com o padrão na primeira execução):

```toml
[[source]]
name = "publico"
path = "~/matheusvcouto/root/skills"

[[source]]
name = "privado"
path = "~/matheusvcouto/root/private-skills"
```

- Funciona de qualquer diretório.
- Sem config, usa o repo de skills em que você está.
- `--source <path>` (repetível) substitui a config naquela execução.
- `skills sources add <path>` / `skills sources list` editam a config.

## Metadados lidos

```yaml
---
name: acp-integration          # obrigatório: minúsculas e hífens, igual à pasta
description: ...               # obrigatório
metadata:
  version: "3.3.0"             # recomendado: SemVer ou X.Y
  internal: true               # opcional: oculta da lista (WIP)
---
```

- **versão**: `metadata.version`; sem versão, compara só pelo hash;
- **hash**: `skills-hash-v1`, SHA-256 dos arquivos ordenados por bytes, com
  caminho, bit de execução e tamanho (contrato em [`ANALISE.md` §3](ANALISE.md#3-hash-e-identidade-contrato));
- **quais arquivos**: numa origem que é repo git, vale a visão do git
  (rastreados + novos não ignorados). O `.gitignore` vale para descoberta,
  hash, install e build — por isso `acp-integration-old/` não conflita. Fora do
  git: ignora `.git`, `node_modules`, `.DS_Store` e `.env*`;
- erros de especificação (nome, descrição, `metadata` não string) bloqueiam
  `install` e `build`; o `doctor` lista tudo.

## Comandos

| Comando     | O que faz                                                         |
| ----------- | ----------------------------------------------------------------- |
| `list`      | Skills das origens: categoria, nome, versão, origem.              |
| `status`    | Origem × destino com o estado de cada uma. Não altera nada.       |
| `install`   | Instala novas; oferece atualizar as já instaladas.                |
| `update`    | Só as instaladas com diferença; pré-marca `update`/`conteúdo mudou`. |
| `uninstall` | Tira do destino as instaladas pelo `skills`, movendo para `backups/` (nada é apagado). |
| `restore`   | Traz de volta a versão guardada em `backups/` (desfaz `update`/`uninstall`). |
| `diff`      | Arquivos que diferem entre origem e destino de uma skill.         |
| `build`     | Valida e gera `<name>-v<versão>.skill` em `dist/`.                |
| `doctor`    | Valida frontmatter, nomes duplicados e layout dos repos.          |

Flags comuns:

| Flag                        | Significado                                         |
| --------------------------- | --------------------------------------------------- |
| `--global`                  | Destino global dos hosts.                           |
| `--project <path>`          | `<path>/.claude/skills` e `<path>/.agents/skills`.  |
| `--host claude\|codex\|all` | Padrão: `all`.                                      |
| `--source <path>`           | Origem desta execução (repetível).                  |
| `--all`                     | Seleciona tudo, sem lista.                          |
| `--yes`                     | Sem perguntas; falha em conflito.                   |
| `--json`                    | Saída estruturada; nunca abre prompt.               |
| `--exit-code`               | `status`: sai com 1 se houver algo para atualizar.  |

Help e completions (fish, nushell, bash, zsh, PowerShell) vêm do CLI Core da
suíte, como nas outras ferramentas.

## Destinos

| Host   | Global                     | Projeto                   |
| ------ | -------------------------- | ------------------------- |
| Claude | `~/.claude/skills`         | `<proj>/.claude/skills`   |
| Codex  | `~/.agents/skills`         | `<proj>/.agents/skills`   |

> **[validar]** O Codex também usa `~/.codex/skills` (nesta máquina há 5 skills lá).
> A sonda 2 de [`ANALISE.md` §8.5](ANALISE.md#85-marcadores-de-quem-decide-e-sondas-antes-de-construir) decide se ele entra como destino.

O global do Claude é sempre `~/.claude/skills`, mesmo que `CLAUDE_CONFIG_DIR`
esteja definido. Levar as skills para os perfis (ai-profile) é papel do
[`agent-protocol`](https://github.com/matheusvcouto/agent-protocol)
(`inherit.skills`), não do `skills`. Por isso a raiz de destino pode ser um
symlink: o `skills` resolve o caminho real antes de escrever, sem quebrar o link.

## Estados

| Estado           | Quando                                               | Na lista      |
| ---------------- | ---------------------------------------------------- | ------------- |
| `novo`           | Na origem, não no destino.                           | desmarcado    |
| `atualizado`     | Mesma versão e mesmo hash.                           | oculto        |
| `update`         | Versão da origem maior.                              | marcado       |
| `conteúdo mudou` | Mesma versão, hash diferente (versão não foi subida). | marcado      |
| `modificado ⚠`   | O destino foi editado depois da instalação.          | pede confirmação |
| `downgrade`      | Versão do destino maior que a da origem.             | desmarcado    |
| `não gerenciado` | Pasta com o mesmo nome, não instalada pelo `skills` (ex.: `npx skills`). | bloqueado sem confirmação; symlink nunca é tocado |
| `órfão`          | Instalada pelo `skills`, mas sumiu das origens.       | sugere `uninstall` |

## O que fica gravado no destino

Um arquivo por raiz de skills, sem mexer dentro da pasta da skill:

```text
<destino>/skills/.skills-lock.json
```

```json
{
  "schema_version": 1,
  "skills": {
    "acp-integration": {
      "version": "3.3.0",
      "hash": "skills-hash-v1:sha256:…",
      "source": "publico",
      "category": "dev",
      "installed_at": "2026-10-21T22:30:00Z"
    }
  }
}
```

É assim que o `skills` distingue `modificado` (hash do destino ≠ hash gravado)
de `não gerenciado` (pasta sem entrada no lock).

## Instalação

- Cópia (não symlink): o destino não quebra se o repo mudar de branch ou de lugar.
- Copia para uma pasta temporária ao lado do destino, valida e troca.
- Antes de substituir, guarda a versão anterior em
  `~/.cli-tools/skills/backups/<data>/<destino>/<name>/`, fora das pastas que os
  agentes leem. Backups com mais de **30 dias** são apagados por uma goroutine
  que roda junto com o `update`; o comando espera por ela antes de sair (para não
  ser interrompida no meio) e falha de limpeza vira só aviso. A limpeza só remove
  pastas dentro de `backups/` com o formato de nome que ela mesma cria.
- Só mexe no que está no **próprio** lock. Nunca segue nem apaga symlink;
  nunca toca em `synced/` (pasta do app Claude) nem em entradas da Vercel.
- Trava o destino durante a operação (dois `update` ao mesmo tempo não se atropelam).
- Depois de copiar, recalcula o hash do destino e confere com a origem.
- Nunca apaga o lock: formato antigo é migrado; formato desconhecido é recusado.

## Build

```sh
skills build                        # lista interativa
skills build acp-integration        # dist/acp-integration-v3.3.0.skill
skills build marketing/ --out ~/Downloads
```

O `.skill` é um zip com a pasta da skill na raiz, no formato aceito pelo
upload do Claude.ai. O build valida o frontmatter antes e é reprodutível: o
mesmo conteúdo gera os mesmos bytes (ordem fixa, data fixa). `build_dir` pode ser
definido no `skills.toml` do repo (substitui o `OUTPUT_DIR` do `.env` atual).

## Fora do escopo (por enquanto)

- Instalar de URL/GitHub/marketplace (para isso existe `npx skills`).
- Symlink/modo dev.
- Outros agentes além de Claude e Codex.
- Histórico, rollback multinível, journal.
- Telemetria (nenhuma, nunca).

---

## Por que não usar só o `skills` da Vercel

Estudado: `vercel-labs/skills` v1.7.4 (código-fonte, commit `a80e4c9`, 19/10/2026).

**O que ele faz bem e vamos copiar:**

- layout `skills/<categoria>/<nome>/SKILL.md` com busca até 3 níveis e
  “o nível mais raso esconde o que está abaixo”;
- hash da pasta com caminho relativo + conteúdo, ignorando `.git`/`node_modules`;
- separar `check` (só olha) de `update` (aplica) — aqui: `status` × `update`;
- `metadata.internal` para esconder skills em andamento;
- flags `--all`, `--yes`, seleção por nome, multi-select interativo;
- lock separado do conteúdo da skill.

**Por que ele não atende sozinho:**

| Necessidade             | `npx skills` (Vercel)                                              |
| ----------------------- | ------------------------------------------------------------------ |
| Atualizar de repo local | `update`/`check` **pulam** skills de caminho local (“Local path”). |
| Versão da skill         | Ignora `metadata.version`; só hash. Sem “3.1.1 → 3.3.0” nem downgrade. |
| Edição no destino       | Não detecta que o destino foi alterado à mão.                      |
| Build `.skill`          | Não existe.                                                        |
| Claude global           | Usa `$CLAUDE_CONFIG_DIR/skills` se definido.                       |
| Dependências            | Node ≥ 22.20 via `npx`; telemetria ligada por padrão.              |

Ele continua sendo o caminho para **terceiros** instalarem o repo público, por
isso o layout é compatível.
