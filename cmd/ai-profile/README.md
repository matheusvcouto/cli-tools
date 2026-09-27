# ai-profile

Gerencia perfis isolados para Claude, Codex e Grok Build.

Decisões técnicas: [`ADR.md`](ADR.md).

## Uso

Listar perfis:

```sh
ai-profile claude list
ai-profile codex list --json
ai-profile grok list
```

Criar, renomear e excluir perfis:

```sh
ai-profile claude new personal
ai-profile grok new xai
ai-profile claude rename personal work
ai-profile claude delete work
```

Executar uma ferramenta usando um perfil:

```sh
ai-profile claude run personal
ai-profile codex run work
ai-profile grok run xai
ai-profile grok run xai -- -p "revise este projeto" --output-format json
```

Executar pelo protocolo ACP:

```sh
ai-profile claude acp personal
ai-profile codex acp work
ai-profile grok acp xai
ai-profile grok acp xai -- --model grok-4.6
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

Para Grok Build, cada profile é o seu `GROK_HOME`; perfis novos recebem um `config.toml` mínimo que desativa importação global Claude/Cursor e auto-update durante execuções pelo wrapper. `ai-profile` não ativa `--always-approve`/`--yolo`; permissões continuam sob controle do Grok/cliente ACP.

Para Claude, cada profile contém tanto o `CLAUDE_CONFIG_DIR` quanto um
`ANTHROPIC_CONFIG_DIR` próprio em `<profile>/.anthropic`. Para Codex, cada
profile é o seu `CODEX_HOME`. Consulte a documentação detalhada em
[`docs/ai-profile.md`](../../docs/ai-profile.md).

## Versão

```sh
ai-profile --version
ai-profile version --json
```

`--version` mostra a versão do produto; `version --json` também informa a versão da suíte/build.

## Completions

Fish, Nushell, Bash, Zsh e PowerShell são gerados do mesmo grafo da CLI:

```sh
ai-profile completion list
ai-profile completion generate fish
ai-profile completion install fish
ai-profile completion status fish
ai-profile completion doctor fish
ai-profile completion uninstall fish
```

`install` nunca edita silenciosamente arquivos de configuração/profile do shell.
