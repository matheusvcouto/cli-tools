# View Limits — arquitetura futura (não implementada)

**Estado:** proposta de arquitetura, sem CLI, endpoints, coleta de credenciais ou testes de integração. **Origem:** solicitação do proprietário do projeto em 26/09/2026. Não misturar com a entrega do provider `grok`.

## Objetivo

Apresentar, por provider **e por perfil escolhido**, a disponibilidade de limites (se houver fonte oficial autorizada), janelas de reset, consumo local e dados de assinatura/API sem confundir categorias. Não inferir cota semanal/mensal pelo número de tokens de uma sessão. Nunca afirmar "0 restante" quando o provider não fornece o dado: responder **indisponível** com motivo.

## Contratos propostos (não criados no código)

- `ProfileReference`: `(provider, alias, local profile directory)`, obtida somente do `Store`; nenhum token no índice ou no snapshot.
- `UsageSource`: `{provider, accountScope, sourceKind, officialSourceVersion}`; `accountScope`: `subscription`, `api_organization`, `api_project` ou `local_session`. Identificadores de conta devem ser truncados/ocultados.
- `LimitMeasurement`: `{metric, unit, used?, limit?, remaining?, windowStart?, windowEnd?, resetAt?, measuredAt, provenance}`. `metric` deve distinguir tokens, requests, mensagens, créditos e gasto; campos desconhecidos são `null`, nunca estimativas implícitas. `provenance`: `official_api`, `official_cli`, `local_estimate`; estimativas locais nunca se apresentam como limite remoto.
- `ViewLimitsResult`: `{provider, profile, status, measurements[], observedAt, nextRefreshAfter?, errorCode?}` com `status` entre `available`, `partial`, `unsupported`, `unauthenticated`, `permission_denied`, `temporarily_unavailable`, `stale`.
- `UsageCollector.Collect(ctx, profile)` deve ser **somente leitura** e expor capacidades por provider em vez de uniformizar dados que não existem. O futuro comando pode ser `ai-profile <provider> limits <profile> --json` depois de congelar contratos e documentação.

## Pesquisa por provider (revalidar na implementação)

| Provider | Superfície observada | O que **não** assumir |
|---|---|---|
| Codex | O app-server e a CLI oficiais fornecem canais de integração; APIs de rate limit da OpenAI expõem cotas da **API** em escopo de organização/projeto. Revalidar o método oficial de **assinatura Codex** antes de chamar qualquer RPC; testar com versão concreta. | Headers `x-ratelimit-*` da API não são saldo da assinatura ChatGPT/Codex nem medida de limite semanal da conta pessoal. |
| Claude Code | A experiência oficial exibe uso do plano dentro do produto; Anthropic oferece Rate Limits API de organização/workspace com credencial administrativa e APIs de uso/custo específicas. | A credencial OAuth do Claude Code não deve ser reutilizada automaticamente como **Admin API key**; limites de API corporativa e de assinatura Claude Code são distintos. |
| Grok Build | `/usage` mostra allowance da conta na UI; `grok usage <session-id>` oferece totais **locais por sessão**, não cota do plano. Reavaliar se uma interface programática oficial de limites passa a existir. | Não consumir endpoints internos do cliente, nem automatizar TUI/scraping, nem deduzir quota semanal da contabilização local. |

Fontes primárias consultadas: [Grok slash commands](https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/04-slash-commands.md), [Grok session usage](https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/17-sessions.md), [OpenAI API rate limits](https://developers.openai.com/api/docs/guides/rate-limits), [Anthropic Rate Limits API](https://platform.claude.com/docs/en/manage-claude/rate-limits-api), [Anthropic API limits](https://platform.claude.com/docs/en/api/rate-limits). Fontes de APIs de assinatura individuais **não foram estabelecidas** nesta rodada.

## Autenticação e privacidade: requisitos obrigatórios

1. **Não acessar credenciais automaticamente durante `list`, `version`, completion ou `run`**. `limits` será uma operação explícita e não interativa por padrão; autenticação/consentimento em etapa própria.
2. Usar mecanismo **oficial e read-only** da versão instalada quando disponível; se exigir credencial com privilégio administrativo, solicitar autorização específica. Não copiar tokens de Claude/Codex/Grok para outro profile ou manter cache de segredos.
3. Nunca fazer scrape do navegador ou utilizar endpoints privados reversos. Não executar prompt pago como "probe" de cota sem consentimento e custo claramente informado.
4. No Windows, abrir arquivos autorizados por caminho confinado e verificar DACL/ACL do perfil antes de ler segredos; em todos os SOs recusar symlink/special file e não imprimir headers `Authorization`/cookies em logs.
5. Cache **somente de métricas não secretas**, em memória inicialmente, com TTL, backoff, `Retry-After`, cancelamento e isolamento `(provider, perfil, escopo)`. Períodos e timezones vêm do provider, não devem ser arbitrados pelo wrapper.
6. Account mismatch, 401, 403, 429, 5xx, logout, offline, quota por workspace e ausência de documentação são estados distinguíveis. Fail-closed: `unsupported` é resposta válida, não motivo para usar API não documentada.

## Plano de execução por gates

- **VL-01 — Contratos:** definir esquema JSON, escopos, semântica de `null`, estabilidade CLI e testes de schema; nenhum acesso remoto nesta etapa.
- **VL-02 — Pesquisa versionada:** validar oficialmente fontes de assinatura e API de cada provider, autenticação, escopo mínimo, políticas de uso e janelas; registrar versões e limitações.
- **VL-03 — Boundary:** implementar read-only collector isolado do `ProcessRunner`, sem alterar environment de `run`/ACP; storage de dados apenas após threat model de cache/ACL.
- **VL-04 — Provider por provider:** Codex/Claude/Grok independentes, retornando `unsupported` se uma capacidade não existir oficialmente. Proibir adaptadores que inferem saldo por sessões locais.
- **VL-05 — Gates reais:** três perfis autorizados distintos; negar secret leakage nos logs; testar 401/403/429, múltiplas janelas, stale/offline, logout, org/project mismatch e Windows/macOS/Linux nativos. Validar live contra as versões oficiais, sem chamadas artificiais atribuídas aos providers.

**Fora de escopo deste snapshot:** uso de OAuth pessoal por scraping, endpoints privados, coleta periódica, UI/edge badges e qualquer alteração dos fluxos existentes `run`, `acp`, `list` ou store para leitura de tokens.
