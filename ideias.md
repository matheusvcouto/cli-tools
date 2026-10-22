# Ideias

Ideias ainda sem plano. Quando uma virar trabalho, entra em [tasks.md](tasks.md).

## Configuração central da suíte

Toda ferramenta guarda configuração e estado num único lugar previsível:

```text
~/.cli-tools/            ($CLI_TOOLS_HOME para trocar a raiz)
├── skills/
│   ├── config.toml      # editável pelo usuário; criado com o padrão na 1ª execução
│   ├── state/           # dados da máquina (índices, registros)
│   ├── cache/           # descartável
│   └── backups/         # cópias antes de substituir algo
├── media-get/
└── ai-profile/
```

- A ferramenta não escolhe caminho: chama `toolhome.Resolve(nomeDoToolJSON, ...)`
  e recebe o diretório certo em qualquer sistema operacional.
- `EnsureConfig(padrão)` cria o `config.toml` com os valores padrão da
  ferramenta só quando ele não existe.
- Já implementado em `internal/toolhome` (ADR D024).

Próximos passos possíveis:

- comando comum do CLI Core, por exemplo `<tool> config path` e `<tool> config edit`,
  derivado do mesmo pacote para todas as ferramentas;
- migração de `ai-profile` e `media-get` (ver tarefa em `tasks.md`);
- limpeza comum de `backups/` por idade (o `skills` começa com 30 dias).
