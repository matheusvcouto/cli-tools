# Arquitetura

## Forma geral

```text
cmd/<tool>/main.go
      ↓
internal/<tool>/app.go       # parsing/UX
      ↓
internal/<tool>/service.go   # casos de uso e invariantes
      ↓
stdlib + capabilities pequenas quando a semântica muda por SO
```

A suite não possui framework de CLI próprio.

## Estrutura

```text
cmd/                  # binários distribuídos
internal/
├── aiprofile/        # domínio ai-profile
├── repozip/          # domínio repo-zip
├── cliapp/           # render de erro + exit code mínimo
├── filelock/         # lock cross-process específico de plataforma
├── fscommit/         # replace confinado específico de plataforma
├── safefs/           # operações confinadas a uma raiz
└── version/          # metadata de build
tools/                # tooling/migração; nunca release
docs/                 # estado atual
migration/            # trabalho transitório ainda aberto
```

## Compartilhamento

Extraia código somente quando duas partes precisam da **mesma semântica/invariante**.

Compartilhamentos atuais:

- `cliapp`: erro/exit mínimos;
- `safefs`: “esta operação não pode escapar desta raiz”;
- `filelock`: exclusão mútua do store;
- `fscommit`: replace do store/settings dentro de uma raiz já aberta;
- `version`: versão injetada no build.

Não criar `utils`, `helpers`, `common`, `shared` ou parser genérico de comandos.

## Plataforma

Código portátil permanece único. Hoje `archive/zip`, Git orchestration, naming e parsing não têm backend por SO. O modo `repo-zip --git` delega serialização do histórico ao próprio `git bundle`, em vez de interpretar/copiar internals de `.git`; isso mantém a mesma lógica para repositório principal e linked worktree.

Separação nativa existe somente para:

- lock de arquivo;
- replace atômico do store/settings;
- process replacement do `ai-profile`;
- publicação final do `repo-zip`.

Uma plataforma pode compilar com stub explícito sem ser considerada suportada.

## Shell

O shell não faz parte da arquitetura. `argv`, environment e stdio chegam diretamente ao binário.

Completions são adapters opcionais gerados pelo próprio `ai-profile` e consultam endpoints internos machine-readable; não leem o store diretamente.

## Novas CLIs

Comece com:

```text
cmd/<nome>/main.go
internal/<nome>/...
```

Uma nova CLI em `cmd/*` deve ser descoberta automaticamente por CI/release. Compartilhamento só é extraído depois de necessidade real.
