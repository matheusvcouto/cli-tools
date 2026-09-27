# Plataformas

## Regra

Separar implementação por SO somente quando a garantia realmente muda. O CLI Core modela **capability** e **requirement**, não booleans de SO espalhados pelo domínio.

```text
Spec declara necessidade
      ↓
availability conhecida sem I/O
      ↓
requirement/probe seguro
      ↓
handler somente se preflight passou
```

Capability ausente falha antes de efeito de domínio.

## Portável

- CLI Core: compile/parser/binding/help/schema/contract/docs;
- completion engine e geração Fish/Nushell 0.114+/Bash/Zsh/PowerShell;
- JSON store/schema e naming;
- seleção/guards Git;
- `archive/zip` create/verify.

Completion **gerada** ser portátil não significa que cada shell foi executado em todo SO. Native E2E é evidência separada.

## Adapters de plataforma

As regras de domínio não selecionam SO. Quando a semântica muda, a implementação fica em arquivos com build tags:

- `filelock`: Unix / Windows;
- `fscommit`: Unix / Windows;
- process runner do `ai-profile`: Unix / Windows;
- publicação do `repo-zip`: Unix / Windows;
- paths de completion: macOS / Linux / Windows.

macOS e Linux compartilham backend Unix apenas onde a primitive e sua garantia são equivalentes.

## Windows

A implementação Windows cobre as capabilities de runtime necessárias:

- lock exclusivo do profile store via `LockFileEx`/`UnlockFileEx`;
- replace confinado de commits via `safefs.Root.Rename` (`os.Root` no Go de release);
- `ai-profile run/acp` via processo nativo, sem shell, com stdin/stdout/stderr diretos e argv literal; o processo nasce suspenso, entra em um Job Object e só então é retomado, com cancelamento/cleanup da árvore inteira e propagação exata do exit code quando a contenção termina normalmente;
- wrappers npm `.cmd` conhecidos de Claude/Codex/Grok/ACP são resolvidos para o entrypoint validado e executados diretamente por `node.exe`; wrappers desconhecidos continuam fail-closed;
- ambiente Windows é tratado com nomes de variáveis case-insensitive, inclusive isolamento de credenciais e sanitização `GIT_*`;
- publicação `repo-zip`: force por rename confinado; no-clobber por hard link confinado + remoção do temporário;
- completion paths possuem adapter Windows próprio;
- release gera `.zip` com `.exe` para `windows/amd64` e `windows/arm64`.

CI nativo usa `windows-2025` (x64) e `windows-11-vs2026-arm` (ARM64), além de cross-build separado. O matrix principal também cobre nativamente Linux e macOS em amd64+arm64, alinhado aos seis targets publicados. Windows x64 executa o race detector suportado pelo Go; Windows ARM64 não o executa porque o race detector oficial não suporta essa combinação. A release é construída uma única vez; ambos os runners Windows baixam e executam os archives desse bundle, e a publicação reutiliza exatamente o mesmo artifact após os gates nativos.

## Terminal

O composition root detecta stdio como character device usando stdlib e passa `cli.Terminal`. Interaction padrão não faz prompt quando stdin não é TTY. Largura/cor são capabilities separadas e não são inferidas artificialmente.

## `os.Root`

O módulo e as releases exigem Go 1.27.1. `internal/safefs` usa `os.Root` como única implementação; toolchains antigas são recusadas em vez de receber um fallback com garantias inferiores.

## Estados de suporte

- `supported`: implementação + runtime tests nativos observados;
- `partial`: só parte das capabilities;
- `untested`: implementação existe, evidência nativa ainda não foi observada;
- `unsupported`: garantia necessária não implementada.

Cross-build nunca promove sozinho o estado de suporte.
