# AGENTS.md

Regras canônicas para qualquer agente que trabalhe neste repositório.

## 1. Contexto mínimo

Leia sempre:

1. `README.md`
2. `ADR.md`
3. `docs/engineering.md`
4. `docs/architecture.md`
5. `docs/testing.md`
6. `docs/platforms.md`

Se `migration/` contiver uma migração ativa relacionada à tarefa, leia o `README.md` daquela migração e siga a ordem indicada lá.

`docs/history/` é histórico: não deve ser carregado por padrão. Consulte apenas para regressão, auditoria ou decisão antiga específica.

## 2. Restrições inegociáveis

- Nunca testar contra estado real do usuário.
- Nunca escrever, renomear ou apagar `~/.ai-profiles` em testes ou durante a implementação da migração.
- Nunca executar `claude`, `codex`, `claude-agent-acp` ou `codex-acp` reais em testes.
- Nunca acessar Keychain, Credential Manager, Secret Service, tokens ou contas reais.
- Nunca alterar Git config global/sistema.
- Repositórios Git de teste devem existir somente sob diretório temporário controlado pelo teste.
- A suíte atual não depende de rede.
- Não copiar para fixtures/logs dados reais e depois “redigir”; fixtures já nascem sintéticas.
- Não commitar, pushar, publicar release ou alterar configuração real do usuário sem pedido explícito.

## 3. Arquitetura da suite

- Um único módulo Go.
- Todo binário distribuído fica em `cmd/<nome>/` e deve implementar `--version` usando a versão compartilhada da suite.
- Regra de negócio de cada CLI fica em `internal/<dominio>/`.
- Regra de negócio não pode depender de `runtime.GOOS`, build tags, Win32/POSIX ou utilitário específico do SO; use ports/backends conforme `docs/platforms.md`.
- Plataforma/capability não implementada deve retornar erro explícito; é proibido fallback silencioso menos seguro.
- `cmd/*` contém apenas entrypoints distribuíveis. Ferramentas internas de build ficam em `tools/`, nunca em `cmd/`.
- Não criar `utils`, `helpers`, `common` ou `shared` genéricos.
- Só extrair pacote reutilizável quando houver semântica comum comprovada, não apenas chamadas parecidas à stdlib.
- Novo utilitário deve seguir `docs/adding-tools.md`.
- Pacote Go público fora de `internal/` exige consumidor externo real e registro em `ADR.md`.

## 4. Segurança e filesystem

- Nunca validar containment com prefixo textual.
- Preferir APIs confinadas (`os.Root`) quando a operação deve permanecer dentro de uma raiz.
- Usar `Lstat` quando symlink não deve ser seguido.
- Nunca assumir que `os.Rename` oferece a mesma atomicidade em todos os SOs.
- Build/cross-build verde não significa suporte de runtime; só declarar `supported` após teste real da capability naquele SO.
- Operação destrutiva só remove/quarentena objetos que foram validados como pertencentes à raiz controlada.
- Temporário destinado a publicação deve ficar no mesmo filesystem/diretório lógico do destino.
- Nunca apagar um arquivo de destino para “facilitar” replace; usar primitiva de commit apropriada e testada.

## 5. Processos

- Construir argv como lista; sem shell intermediária.
- ACP reserva stdout integralmente ao processo/protocolo filho.
- Logs/diagnóstico do wrapper vão para stderr.
- Variáveis de autenticação a limpar são removidas do ambiente, não preenchidas com string vazia.
- Testes de subprocesso usam fakes cujo caminho resolvido é comprovadamente temporário antes do spawn.

## 6. Dependências

Política: **stdlib-first, não stdlib-only**.

Dependência externa só entra quando torna a implementação comprovadamente mais segura/correta/manutenível. Antes de adicionar:

1. provar a lacuna concreta;
2. avaliar licença/manutenção/transitivas;
3. preferir dependência pequena e focada;
4. registrar a decisão em `ADR.md` se afetar arquitetura/plataforma;
5. adicionar teste que cubra a semântica motivadora.

`golang.org/x/sys` é candidato aceitável para primitivas nativas de lock/replace, se necessário. Não adicionar Cobra/Viper apenas por conveniência.

## 7. Qualidade

Conforme aplicável, prefira o runner sandboxed:

```sh
mise run check
# ou, sem mise já configurado:
./scripts/check-safe.sh all
```

Os subcomandos `fmt`, `test`, `vet`, `shuffle`, `race` e `fuzz` também podem ser executados pelo mesmo script. O runner usa HOME/TMP/caches temporários, não herda secrets/configurações Git do usuário e bloqueia rede/download automático do Go durante os testes.

`-race` é um job separado: não force `CGO_ENABLED=0` nele. Builds de release devem permanecer `CGO_ENABLED=0`, salvo ADR explícito.

Se algo não pôde ser executado, registrar **não executado**; nunca chamar de aprovado.

## 8. Definition of done

Uma mudança só está pronta quando:

1. comportamento/contrato está claro;
2. implementação é mínima e legível;
3. testes relevantes passam;
4. falhas e rollback foram testados quando há mutação;
5. docs/ADR foram atualizados quando necessário;
6. nenhum estado real foi tocado;
7. não há afirmação de suporte sem evidência correspondente.

Durante uma migração ativa, atualizar também o checklist indicado pelo `README.md` daquela migração.
