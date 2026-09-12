# Resumo sanitizado das decisões históricas do `ai-profile`

Este arquivo substitui a cópia de documentos históricos que continham nomes/paths/contexto pessoal desnecessário. Não contém credenciais nem paths reais.

Decisões relevantes para o port:

- diretórios de profile têm identidade opaca estável; rename altera apenas alias;
- registry único de tools evita duplicar lógica por CLI;
- ordem pública atual é `ai-profile <tool> <action> ...`;
- `run` precisa repassar flags/argv sem interpretação pelo wrapper;
- `apply-statusline` é manual/opt-in e mescla apenas `statusLine`;
- perfis Codex herdam `AGENTS.override.md`/`AGENTS.md` globais por symlink quando não existe destino local;
- `CLAUDE_CONFIG_DIR` e `CODEX_HOME` são os mecanismos de isolamento usados atualmente;
- `agy` foi removido deliberadamente: a estratégia avaliada dependia de comportamento de credential store global/fallback e não fornecia isolamento robusto;
- ACP usa stdout para JSON-RPC e não aceita logs do wrapper nesse stream;
- macOS é o ambiente em que `run/acp` de Claude/Codex foram declarados suportados; Linux/Windows permanecem sem validação equivalente.

Para detalhes executáveis, a fonte de verdade congelada é `mod.nu`; para o port, a fonte de verdade é `../../../ai-profile-parity.md` + `../../../ai-profile-contract.md`.
