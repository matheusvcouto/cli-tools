# Plano executado e próximos gates

## 1. Arquitetura — concluído

Monorepo Go, módulo único, `cmd/*` para binários e `internal/*` para domínio privado. Compartilhamento só existe para invariantes reais (`safefs`, lock, commit, versão e erro mínimo).

## 2. Independência de shell — concluído

Os binários recebem argv/env/stdio diretamente. Nenhuma regra de negócio executa ou parseia Nushell. Completions opcionais existem para Bash/Fish/Zsh.

## 3. `ai-profile` — implementação concluída

- Claude/Codex;
- store JSON versionado;
- create/list/rename/delete/run/acp/apply-statusline;
- environment isolation;
- process replacement Unix;
- guidance Codex;
- templates embutidos;
- completions;
- migração transitória do schema legado específico.

O formato visual antigo não é contrato. Capacidade e segurança são.

## 4. `repo-zip` — implementação concluída

- Git seleciona arquivos;
- `archive/zip` cria/verifica;
- default `.tmp/repo-zip`;
- symlinks preservados;
- guards de submodule/sparse/skip-worktree/tracked output/`.git`;
- `--git` via bundle restaurável + metadata mínima, sem copiar internals de `.git`;
- safety root no destino;
- verificação antes de publish;
- file-set recheck;
- no-clobber e force separados.

Não existe backend `zip/unzip` por plataforma.

## 5. Plataformas — implementação atual

- Linux: runtime sintético testado localmente;
- macOS: mesma implementação Unix, cross-build verde; falta execução real no GitHub Actions;
- Windows: compile-only. Stubs deixam explícitas as capabilities nativas ainda não implementadas.

## 6. Qualidade — local concluído

Gates locais: format, test, vet, shuffle/repeat e race. Cross-build cobre seis pares OS/arch.

## 7. Release — implementação concluída, execução externa pendente

A workflow de tag:

1. testa Linux e macOS;
2. só então gera artifacts;
3. smoke-testa o archive Linux;
4. valida SHA256SUMS;
5. publica GitHub Release.

A instalação alvo é `mise use -g github:matheusvcouto/cli-tools@latest`.

## 8. Gate externo pendente

No repositório real:

1. CI macOS verde;
2. criar tag de teste/release;
3. instalar essa release com mise em HOME/MISE_* temporários;
4. validar que `ai-profile --version` e `repo-zip --version` saem do mesmo archive.

## 9. Cutover autorizado

Somente quando o usuário decidir:

1. backup do estado legado;
2. converter `index.nuon` para `index.json` com paths explícitos, se necessário;
3. instalar a suite Go;
4. validar profiles com dados reais de forma não destrutiva;
5. retirar comandos/imports antigos do ambiente;
6. manter rollback por uma janela curta.

## 10. Arquivamento

Após todos os gates e o cutover:

```text
git mv migration/nushell-to-go docs/history/migrations/2026-09-nushell-to-go
```

Nenhuma doc ativa deve depender dessa pasta depois do move.
