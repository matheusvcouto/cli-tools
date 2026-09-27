# CONTEXT — SNAPSHOT-008 / cli-tools

**Data:** 2026-09-26. **Repositório:** `cli-tools`; **módulo:** `github.com/matheusvcouto/cli-tools` (sem sufixo `/v2`); **próxima suíte:** `v1.1.0`, ainda não tagueada; **Go mínimo:** `1.27.1`. **Base:** SNAPSHOT-007, com a decisão de versão ajustada depois da importação. Relatório integral: `plans/release-readiness/SNAPSHOT_008_REVIEW.md`.

## Estado e escopo

- `ai-profile`: `claude`, `codex` e `grok` com `list/new/rename/delete/run/acp` e diretórios/ambientes de configuração separados. Grok ACP `grok agent <opções> stdio`, sem aprovações automáticas. `GROK_HOME` não impede a leitura de instruções do workspace nem representa sandbox absoluto.
- Armazenamento: `os.Root`, lock por SO, backup de índice, limites 8 MiB, identificação de órfãos, migração NUON explícita e delete com identidade integral sob lock (corrigido em 007). Windows: DACL protegida do root, LockFileEx, atomicidade confinada, Job Objects antes de liberar processo, e launch de npm conhecido por Node sem shell. Atenção: ACEs explícitas em arquivos Windows legados exigem auditoria nativa.
- GitHub Actions: CI com seis runners Linux/macOS/Windows amd64+arm64, tests/vet/race quando suportado, actionlint pinado/verificado e contratos. Release faz build único, atesta no builder, verifica SHA e executa seis smokes dos mesmos bytes antes de publicar draft conferido. **008** acrescenta `preflight` fail-closed antes da matriz e de shell completions: actionlint, changes validate, versão/changelog canônicos, path do módulo compatível com a tag, Go, presença de CLIs, ancestralidade do commit e tag remota. Scripts de tag alinharam regex canônico estável com builder Go. Testes para pending changes (inclusive symlink) adicionados.

## Critério de prontidão — diferenciação obrigatória

- **Pode enviar SNAPSHOT-008 como PR/branch para testar de verdade no GitHub, SEM TAG.** A análise estática é um sinal de preparo do código, **não prova de funcionamento geral nem de sucesso da CI**. Static 008: `gofmt` 119 Go, `bash -n` cinco scripts, JSON 15, TOML 1, PyYAML duas workflows/14 jobs, grafos `needs`/SHA sintaticamente coerentes; actionlint real não disponível aqui. Sem execução do Go 1.27.1 nesta máquina, os novos testes Go e vet não foram rodados. Tentativa de download oficial Go falhou por rede/DNS; não reduzir baseline.
- **NÃO distribuir/taguear como release estável ainda:** observar CI desta revisão e Windows x64/ARM64 real; executar instalações/autenticações reais Claude/Codex/Grok e ACP de cada um; checar disponibilidade real Grok ARM64, ACL explícita em perfis legados e migração a partir de backup. CI antiga não vale para código novo. Depois `release prepare --suite-version v1.1.0 --write`, commit preparado, reexecutar CI e `preflight` local; só então tag protegida, release smoke em seis targets e verificação da publicação remota.
- Change records pendentes atuais são **intencionais**: `preflight` deve rejeitar tag na árvore atual até o `prepare --write` consumir os records. Não remover gates para obter aprovação.

## Futuro — View Limits

- `plans/view-limits/README.md`: adaptadores read-only por provider/perfil (Codex, Claude Code, Grok), com consentimento, para limites de assinatura, cotas de API e uso local **separados**; nenhum token extraído pelo fluxo ordinário. A implementação não foi iniciada.

## Regras permanentes

- Nunca criar mocks/testes artificiais nem alegar que testes não rodados passaram. Compensar bloqueios externos com análise de código real e documentação oficial. Não alterar produção para adaptar ao sandbox, nem enfraquecer segurança ou gate. `fail-closed` sempre que apropriado; após corrigir um problema, prosseguir para o próximo possível.
- Entregar **repositório completo ZIP**, `CONTEXT.md` também separado e **Base64 puro** do ZIP, sem cabeçalho/fence. Nomes: `SNAPSHOT-{NNN}_cli-tools.zip`, `CONTEXT-{NNN}_cli-tools.md`, `SNAPSHOT-{NNN}_cli-tools_BASE64.md`; relatório e checksums adicionais. Validar integridade do ZIP, árvore extraída, SHA e reconstrução Base64. Preferir anexos ao fim, sem cards personalizados; caso a UI não renderize anexo nativo, usar os arquivos disponibilizados e o Base64 puro.
- macOS: `base64 -D -i ~/Downloads/SNAPSHOT-008_cli-tools_BASE64.md -o ~/Downloads/SNAPSHOT-008_cli-tools.zip`.
