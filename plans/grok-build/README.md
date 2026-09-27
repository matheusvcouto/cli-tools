# Provider ativo: `grok` no `ai-profile`

1. **Estado vigente:** `CODE_REVIEW_003.md` revisa e supera detalhes de `IMPLEMENTATION_REVALIDATION.md` e `VALIDATION.md` do SNAPSHOT-002.
2. Leia `REPORT_AND_PLAN.md` para a investigação original e a seção final de implementação/revalidação.
3. Leia `VALIDATION.md` antes de promover qualquer gate de compatibilidade Grok real.
4. **Status deste snapshot:** implementação do adapter concluída por código/revisão documental; execução do binário oficial Grok, login, ACP autenticado e Windows nativo permanecem gates reais pendentes no ambiente atual.
5. A suíte padrão usa probes apenas para validar o launcher (argv/env/exit status). Esses probes nunca são evidência de compatibilidade funcional com Grok.
6. Não habilitar `--always-approve`/`--yolo` automaticamente, não executar `.cmd` via shell e não copiar credenciais de `~/.grok`.
