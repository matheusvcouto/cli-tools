> **Registro histórico do SNAPSHOT-002.** Para o código vigente e as correções de isolamento/compatibilidade validadas em 26/09/2026, leia [`CODE_REVIEW_003.md`](CODE_REVIEW_003.md). Em particular, os overrides de compatibilidade herdados agora são *removidos*, não forçados `false`; Codex `skills`/`hooks` constam da referência oficial mais recente; e `GROK_DISABLE_API_KEY_AUTH` é preservado por ser restrição de segurança.

# Grok Build adapter — implementação e revalidação

**Projeto:** `cli-tools` / `ai-profile`  
**Data:** 2026-09-26  
**Snapshot:** `SNAPSHOT-002_cli-tools`  
**Estado:** implementação concluída por revisão de código + documentação oficial; gates funcionais com Grok real permanecem separados.

## Resultado

O `ai-profile` agora possui um terceiro provider `grok` com as mesmas operações de ciclo de vida dos demais providers:

```text
ai-profile grok list
ai-profile grok new <profile>
ai-profile grok rename <old> <new>
ai-profile grok delete <profile>
ai-profile grok run <profile> [args...]
ai-profile grok acp <profile> [agent-options...]
```

O adapter não implementa a API xAI. Ele isola e lança o CLI oficial Grok Build.

## Contrato implementado

### Estado do profile

Cada profile usa:

```text
GROK_HOME=<diretório físico estável do profile>
```

`new` cria `config.toml` antes de publicar o profile no `index.json`. Se a inicialização falhar, o diretório é removido e o índice não é alterado. O arquivo inicial é criado como regular/privado; em Windows, a proteção do root do store é herdada pela DACL já implementada no projeto.

O default desativa descoberta global de compatibilidade Claude/Cursor e sessões Codex compatíveis. Variáveis Claude/Cursor de compatibilidade são reassertadas como `false` durante o launch porque env tem precedência sobre TOML. Isso impede que um shell pai com compatibilidade ligada reverta silenciosamente o default seguro.

### Config do usuário

Depois da criação, `config.toml` é propriedade do profile/usuário. O wrapper não o reescreve a cada `run`/`acp`. Porém ele exige que exista e seja um arquivo regular real; arquivo ausente, symlink ou objeto especial falha antes do spawn.

### Autenticação e overlays

O child não herda `XAI_API_KEY`, external auth command/OIDC, deployment key, `GROK_CONFIG`/`GROK_CONFIG_PATH`, proxy base URL, custom agent herdado nem path de log que possa redirecionar a identidade/configuração do profile. `GROK_HOME` é sempre substituído pelo profile selecionado.

Não é feita limpeza genérica de `GROK_*`. Sandbox, telemetry/policy, requisitos administrados, feature controls e demais guardrails não são apagados indiscriminadamente. `RUST_LOG` é preservado porque altera verbosidade, não identidade.

### Auto-update

Profiles novos recebem `[cli] auto_update=false` e processos lançados pelo wrapper recebem `GROK_DISABLE_AUTOUPDATER=1`. Isso impede alteração da instalação durante uma execução gerenciada. Não equivale a pin de versão; a versão instalada continua responsabilidade do ambiente/release.

### ACP

O contrato correto, revalidado contra a documentação atual, é:

```text
grok agent <agent-options...> stdio
```

Por isso `ACPTool` ganhou `SuffixArgs`: Grok usa prefixo `agent`, argv opaco do usuário no meio e sufixo `stdio`. Isso evita o erro `grok agent stdio --model ...`, porque opções como `--model`, `--reauth`, `--agent-profile` e `--always-approve` pertencem antes do modo de transporte.

O wrapper **não** adiciona `--always-approve`, `--yolo` ou `yoloMode`. O cliente ACP/Grok continua responsável por permissões; deny rules, hooks e sandbox continuam aplicáveis.

### Windows

O resolver mantém a política shell-free:

1. prefere `grok.exe` real quando `PATH` o fornece;
2. se `grok` resolve para `.cmd`, não executa `cmd.exe`;
3. localiza o package oficial `@xai-official/grok` pelos layouts npm/pnpm já suportados;
4. valida `package.json.name` e `bin.grok == bin/grok`;
5. exige entrypoint regular e `node.exe` real;
6. executa `node.exe <package>/bin/grok <argv...>` diretamente.

O launcher público oficial `bin/grok` chama `grok-bootstrap.js`. Se a xAI alterar o manifesto/entrypoint, o resolver falha fechado em vez de improvisar shell parsing.

## Problemas encontrados durante a implementação e corrigidos

1. **Ordem ACP errada:** `agent stdio + args` quebraria opções de agent. Corrigido com `SuffixArgs`.
2. **Limpeza `GROK_*` ampla demais:** poderia remover sandbox/policies. Substituída por lista explícita e estreita.
3. **Compatibilidade de sessões:** config false podia ser sobreposta por env do processo pai. Claude/Cursor `SESSIONS_ENABLED=false` agora também é reassertado.
4. **Campos Codex inexistentes/inertes:** default foi reduzido para `compat.codex.sessions=false` conforme documentação atual.
5. **`RUST_LOG` removido sem necessidade:** retirado da lista de sanitização; ele não troca identidade do profile.
6. **ACL recursiva específica Grok:** descartada. Reescrever toda a árvore poderia interferir em estado/plugins e não adicionava uma garantia necessária além do root protegido e da verificação da âncora config.
7. **Windows `.cmd`:** Grok foi incluído somente no caminho já fail-closed de manifest verification + Node direto; nenhum shell foi adicionado.

## Conflitos/limitações que permanecem por design

- **Contexto de projeto:** cwd é preservado. Grok continua podendo consumir regras do projeto, `.grok` e instruções genéricas conforme seu folder trust. `GROK_HOME` isola estado global; não promete neutralizar contexto do checkout.
- **Credencial por env:** `XAI_API_KEY` herdada é removida. Isso protege isolamento, mas significa que CI que dependa exclusivamente de uma API key no shell não deve esperar herança automática pelo wrapper. A autenticação deve ser estabelecida explicitamente no profile/fluxo autorizado.
- **npm bootstrap:** ao executar o launcher npm com `GROK_HOME` isolado, o bootstrap upstream pode materializar/selecionar binário no home do profile. Isso pode duplicar binários por profile; deve ser medido no Windows real, não contornado com shell.
- **Arquitetura:** disponibilidade do `ai-profile` não prova disponibilidade do binário Grok para a mesma arquitetura. A matriz upstream deve ser verificada por versão publicada.
- **Espelho público:** `xai-org/grok-build` é sincronizado periodicamente do monorepo; documentação/código público justificam o design, mas suporte funcional precisa registrar a versão real do binário executado.

## Validação executada neste ambiente

PASS real:

- `gofmt` / `./scripts/check-safe.sh fmt`;
- parse de todos os JSON do repo;
- parse YAML dos workflows;
- verificações estáticas do contract: `claude`, `codex`, `grok`, subcomandos e ACP suffix;
- inspeção estática do resolver Windows e dos testes adicionados;
- reconsulta das páginas oficiais de configuração, headless, ACP, permissions/sandbox e launcher npm.

Bloqueado / não executado:

- Go 1.27.1: o ambiente possui Go 1.23.2; a tentativa de download do toolchain falhou por DNS/rede para `proxy.golang.org`;
- `go test ./...`: com `GOTOOLCHAIN=local`, o Go recusa corretamente porque `go.mod` exige 1.27.1;
- `actionlint`: o script pinado tentou baixar o artifact oficial, mas `github.com` não resolveu no sandbox;
- `grok` real: binário não está instalado, não foram fornecidas credenciais xAI e não existe runner Windows/macOS nativo local.

Nenhum desses bloqueios foi substituído por mock como evidência de compatibilidade real. Os probes unitários/E2E adicionados comprovam somente lógica do wrapper.

## Gates reais para promover “Grok validado”

1. Registrar `grok --version` e canal de instalação oficial.
2. Criar dois profiles, autenticar separadamente e provar que `auth.json`/sessions não cruzam `GROK_HOME`.
3. Executar TUI e headless real, cobrindo saída JSON/streaming, cancelamento e exit status.
4. Executar ACP real com client: `initialize`, auth quando aplicável, `session/new`, prompt/update e permission requests.
5. Executar Windows x64 nativo: installer oficial e npm, PATH com espaços/Unicode, Job Object, cancelamento e argv opaco.
6. Verificar `grok inspect`/folder trust em checkout controlado com sentinelas Claude/Cursor.
7. Rodar Go 1.27.1: testes, race nas plataformas suportadas, vet e matriz CI/release já configurada.

Até esses gates rodarem, o status correto é **adapter implementado e revalidado por código/documentação; compatibilidade funcional real pendente de execução**.
