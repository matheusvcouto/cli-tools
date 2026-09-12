# Adicionando uma nova CLI

Use este fluxo quando a suite crescer.

## 1. Defina o contrato

Antes do código, registre sintaxe, entradas, saídas, efeitos colaterais, riscos e plataformas suportadas.

## 2. Crie o mínimo

```text
cmd/<nome>/main.go
cmd/<nome>/ADR.md
internal/<dominio>/...
```

`main.go` fino; domínio testável. O ADR local registra decisões próprias da
ferramenta; decisões que afetam toda a suíte continuam no `ADR.md` da raiz. Se
houver diferença por SO, o domínio define a capability e o backend específico
fica separado conforme `docs/platforms.md`. Não espalhe `runtime.GOOS` pela
regra de negócio.

## 3. Não compartilhe cedo demais

Comece local ao domínio. Extraia para `internal/<pacote-coeso>` somente quando outra CLI precisar exatamente da mesma semântica.

## 4. Dependências

Aplique `docs/engineering.md`. Dependência é decisão técnica, não falha moral; mas precisa resolver problema concreto.

## 5. Plataformas

Declare suporte por capability. Uma plataforma futura pode ficar `untested`/`unsupported` com stub explícito; não implemente Windows/Linux apenas para preencher uma matriz e não chame cross-build de suporte.

## 6. Testes

- unit;
- integration isolada;
- falhas/rollback se houver mutação;
- runtime cross-platform quando declarar suporte.

Nunca usar dados/contas/repos reais para “provar” a nova ferramenta.

## 7. Release

Uma CLI corretamente colocada em `cmd/<nome>` deve ser descoberta pelo tooling de release automaticamente. Se for preciso editar várias listas manuais de nomes, corrija o tooling em vez de duplicar configuração.

Todo binário distribuído deve aceitar `--version` e imprimir **somente** a versão compartilhada da suite injetada no build. O smoke test de release percorre `bin/*` dinamicamente e valida esse contrato; assim uma nova CLI não escapa da validação por esquecimento de uma lista manual.

## 8. Docs

Atualize README apenas se a ferramenta for relevante ao usuário. Adicione `AGENTS.md` aninhado somente quando aquele diretório tiver restrições materiais diferentes das regras da raiz.

## Biblioteca Go pública

Se “nova lib” significar pacote Go para consumo externo, não colocá-la em `internal/`. Antes:

- identificar consumidor real;
- escolher package path estável;
- definir API mínima;
- registrar no ADR do escopo afetado porque a API pública passa a ter custo de compatibilidade.
