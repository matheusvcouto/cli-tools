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

## Portável hoje

- CLI Core: compile/parser/binding/help/schema/contract/docs;
- completion engine e geração Fish/Nushell 0.114+/Bash/Zsh/PowerShell;
- JSON store/schema e naming;
- seleção/guards Git;
- `archive/zip` create/verify;

Completion **gerada** ser portátil não significa que cada shell foi executado em todo SO. Native E2E é evidência separada.

## Específico hoje

- `filelock`;
- `fscommit`;
- process replacement do `ai-profile`;
- publicação final do `repo-zip`.

Essas primitivas permanecem fora de `cli/`.

## macOS e Linux

Compartilham implementações Unix quando a semântica é igual: lock/replace apropriados, `exec` para `run/acp` e primitives de publicação do `repo-zip`. Linux possui runtime tests locais; macOS é validado em runner nativo de CI.

## Windows

Windows continua compile-only para o produto quando falta primitive equivalente de runtime. Stubs/capabilities devem retornar erro explícito antes de tocar estado. Para promover suporte, são necessários runtime tests nativos de lock, replace, processo/stdio/exit e publicação force/no-clobber.

PowerShell completion é um adapter independente dessa declaração: geração/conformance não prova os demais casos de uso do produto no Windows.

## Terminal

O composition root detecta stdio como character device usando stdlib e passa `cli.Terminal`. Interaction padrão não faz prompt quando stdin não é TTY. Largura/cor são capabilities separadas e não são inferidas artificialmente.

## `os.Root`

Releases usam Go 1.27.1 e `internal/safefs` usa `os.Root`; fallback de toolchain antiga existe apenas para bootstrap/desenvolvimento e é mais conservador.

## Estados de suporte

- `supported`: implementação + runtime tests nativos;
- `partial`: só parte das capabilities;
- `untested`: implementação existe, evidência insuficiente;
- `unsupported`: garantia necessária não implementada.
