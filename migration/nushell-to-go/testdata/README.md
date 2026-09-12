# testdata

Somente fixtures sintéticas.

Nunca copiar configs, logs, tokens, índices ou repositórios reais do usuário para cá.

Fixtures planejadas:

- `ai-profile/index-v1-valid.json`
- `ai-profile/index-v1-corrupt.json`
- `ai-profile/index-v1-duplicate-alias.json`
- `ai-profile/legacy-index.nuon` (100% sintético)
- `statusline/default.json`
- casos de nomes/paths inválidos quando uma fixture for mais clara que table tests.

Repositórios Git de integração não devem ser fixtures persistentes: crie-os em runtime com `t.TempDir()`.
