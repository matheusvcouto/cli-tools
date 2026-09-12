# Plataformas

## Regra

Separar implementação por SO somente quando a garantia realmente muda.

```text
domínio
  ├── stdlib portátil
  └── capability nativa → arquivo/build tag específico
```

Não espalhar `runtime.GOOS` pela regra de negócio.

## Portátil hoje

- parsing e UX;
- JSON store/schema;
- registry de Claude/Codex;
- seleção/guards Git;
- `archive/zip` create/verify;
- statusline JSON merge;
- naming/path rules;
- completions Bash/Fish/Zsh.

## Específico hoje

- `filelock`;
- `fscommit`;
- process replacement do `ai-profile`;
- publicação final do `repo-zip`.

## macOS e Linux

Compartilham as implementações Unix quando a semântica é a mesma:

- `flock` no lock do store;
- rename dentro da mesma safety root para replace;
- `exec` para `run/acp`;
- hard-link + remoção do temp para publicação no-clobber do `repo-zip`;
- rename para `--force`.

Linux possui testes runtime sintéticos executados localmente. macOS executa os mesmos testes no GitHub Actions e o workflow de release exige o job macOS antes de publicar.

## Windows

Windows permanece compile-only. Stubs explícitos impedem fallback inseguro onde ainda faltam garantias equivalentes.

O desenho permite implementar Windows depois sem reescrever domínio. Para promover a suporte real, implementar e testar ao menos:

- lock do store;
- replace do store/settings;
- execução/preservação de stdio/exit do `ai-profile`;
- publicação `force` e `no-clobber` do `repo-zip`.

## `os.Root`

Releases usam Go 1.27.1; portanto `internal/safefs` usa `os.Root`. O fallback para toolchains antigas existe somente para desenvolvimento/bootstrap e é mais conservador, recusando traversal por symlink.

## Estados de suporte

- `supported`: implementação + runtime tests no SO;
- `partial`: só parte das capabilities;
- `untested`: implementação existe, evidência ainda insuficiente;
- `unsupported`: garantia necessária ainda não implementada.
