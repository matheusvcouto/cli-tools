# ai-profile

Gerencia perfis isolados para Claude e Codex.

## Uso

Listar perfis:

```sh
ai-profile claude list
ai-profile codex list --json
```

Criar, renomear e excluir perfis:

```sh
ai-profile claude new personal
ai-profile claude rename personal work
ai-profile claude delete work
```

Executar uma ferramenta usando um perfil:

```sh
ai-profile claude run personal
ai-profile codex run work
```

Executar pelo protocolo ACP:

```sh
ai-profile claude acp personal
ai-profile codex acp work
```

Também é possível aplicar uma statusline a um perfil Claude:

```sh
ai-profile claude apply-statusline personal
```

Veja todos os comandos disponíveis com:

```sh
ai-profile --help
```

## Armazenamento

Por padrão, os perfis ficam em:

```text
~/.ai-profiles
```

Para usar outra raiz, defina `AI_PROFILE_ROOT`:

```sh
AI_PROFILE_ROOT=/path/to/profiles ai-profile claude list
```

O projeto mantém os perfis separados por ferramenta e remove do ambiente
herdado as configurações que poderiam misturar os contextos.

## Completions

Scripts de completion podem ser gerados para Bash, Fish ou Zsh:

```sh
ai-profile completion bash
ai-profile completion fish
ai-profile completion zsh
```
