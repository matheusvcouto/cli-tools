# Reanálise final da migração

## Melhorias aplicadas em relação aos planos anteriores

1. **Nushell deixou de ser referência de UX.** Só capacidades/invariantes importam.
2. **`repo-zip` não usa `zip`/`unzip`.** `archive/zip` torna create/verify portáteis.
3. **Plataforma só é abstraída quando muda semântica.** Não existe backend por SO artificial.
4. **Mini core foi reduzido.** Nada de framework interno de CLI.
5. **Store/statusline ficam dentro da mesma safety root aberta** durante commits sensíveis.
6. **Delete virou transacional por quarentena + rollback.**
7. **Output padrão virou `.tmp/repo-zip`.** O nome antigo era detalhe histórico.
8. **`repo-zip` revalida o conjunto de arquivos antes de publicar.**
9. **Fish completion foi corrigida por posição.**
10. **CI/release descobrem novas `cmd/*` automaticamente.**
11. **Release exige runtime tests Linux + macOS antes de publish.**
12. **Windows não é chamado de suportado só por compilar.**
13. **E2E virou gate permanente.** Os binários reais são executados contra estado e executáveis sintéticos.
14. **Release usa nomes de asset amigáveis ao mise (`macos`) e attestation quando disponível.**
15. **`safefs.Open` fixa a identidade da raiz.** Não confia apenas em `os.OpenRoot`, pois o path da própria raiz pode conter/trocar symlink.
16. **`ai-profile new` virou transação única.** Diretório e índice usam a mesma root+lock e há rollback em falha de commit.
17. **`repo-zip --git` recusa qualquer `*.lock` dentro de `.git`.** Não depende de uma lista incompleta de locks conhecidos.
18. **Paths Git com `\` são recusados cedo.** Um nome válido em Unix não pode virar ZIP ambíguo/inseguro para extratores Windows.

## Pontos ainda não verificáveis neste ambiente

- execução nativa macOS;
- instalação de uma GitHub Release real via mise;
- cutover do estado real do usuário.

Esses itens permanecem gates externos e não devem ser simulados com estado real só para marcar checklist.
