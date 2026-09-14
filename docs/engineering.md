# Engenharia

## Princípios

1. evidência antes de conclusão;
2. simplicidade antes de abstração;
3. segurança antes de “zero dependências”;
4. comportamento observável antes de equivalência presumida;
5. uma fonte de verdade para contratos derivados.

## CLI Core

A superfície de uma CLI é uma `cli.App` declarativa compilada uma vez. É proibido manter árvores paralelas de parser/help/completion/docs.

- stable IDs representam identidade; nomes/aliases representam sintaxe;
- compiler rejeita Spec inválida antes do runtime;
- nomes/aliases de comandos e flags são tokens seguros para adapters: letras/dígitos Unicode mais `-`, `_` e `.`; metacaracteres de shell são rejeitados no compile;
- `ArgMode`/`FlagAction` desconhecidos, aliases/args duplicados e defaults incompatíveis também falham no compile;
- strict/partial usam a mesma gramática;
- flags globais podem aparecer antes/depois do subcomando quando herdadas; `FlagSet` usa a última ocorrência e ações append/count preservam sua semântica própria;
- short clusters (`-abc`) não são sintaxe suportada; use flags curtas separadas;
- não existe `--no-*` implícito: negation/tri-state precisa ser declarada explicitamente;
- posicionais numéricos tipados aceitam negativos (`-1`, `-0.5`) quando a gramática não os torna ambíguos; `--` permanece o escape explícito;
- handlers recebem valores já tipados;
- `ArgOpaque` impede o wrapper de reinterpretar argv de subprocesso;
- completion dinâmica não faz prompt e deve respeitar cancelamento;
- `cursor_arg`/`cursor_offset` são coordenadas exatas; offset é contado em Unicode scalars e posições fora do argv/token falham fechado;
- help/version/schema/contract/completion gerada permanecem livres de I/O de domínio;
- shell-specific behavior fica no adapter.
- `cli/` é uma API Go pública reutilizável por outros módulos; `cli/internal/` não é contrato público;
- `cli/api.contract.json` protege a superfície exportada: breaking exige major, adição exige atualizar o lock para passar a ser protegida;

## Evidência

Classifique suporte como observado, inferido ou não verificado. Cross-build prova compilação, não runtime. Se um gate não terminou, registrar **não executado/inconclusivo**, nunca verde.

## Erros Go

- preservar causa com `%w`;
- usar `errors.Is/As`;
- diagnostics estruturados para erros de CLI previsíveis;
- contexto sem segredo;
- lógica nunca depende de texto renderizado do erro.

## Interfaces e composição

Não criar interface por estética. Use-a em fronteira real de plataforma/teste/implementação. Dependências caras ou que tocam estado são lazy. Não usar registries globais mutáveis nem `init()` mágico para compor comandos.

## Plataforma

Regra de negócio não seleciona SO. Diferença semântica real vira capability estreita/build tag conforme `docs/platforms.md`; `unsupported` é melhor que fallback inseguro.

## Paths/filesystem

- `filepath`, não separadores manuais;
- containment nunca por prefixo textual;
- `os.Stat` + `os.SameFile` para identidade física quando aplicável;
- `Lstat`/`Readlink` quando symlink é parte da política;
- `os.Root`/`safefs` quando a operação precisa ficar confinada;
- temporário imprevisível/create-exclusive;
- não usar remove-then-rename para substituir estado importante;
- não prometer atomicidade além da primitive comprovada no SO.

## Estado persistente

Schema versionado; decode valida invariantes; corrupção não vira estado vazio; read-modify-write é protegido; commit é seguro por plataforma; backup é recuperação, não substituto de commit correto.

## Processos

- argv como slice, sem shell intermediário;
- env filho explícito quando há isolamento;
- secrets não aparecem em logs/schema/completion;
- preservar exit status conforme contrato;
- ACP reserva stdout ao protocolo filho;
- broken pipe (`EPIPE`) é encerramento normal no composition root, não erro interno renderizado ao usuário.

## Dependências

Política **stdlib-first, não stdlib-only**. CLI Core atualmente não exige library externa. Antes de adicionar qualquer dependência: provar lacuna, avaliar licença/manutenção/transitivas, preferir pacote focado, registrar decisão e testar a semântica motivadora. Não adicionar Cobra/Viper por conveniência.

## Toolchain e documentação

`mise.toml` fixa a baseline; não executar `go env -w`; testes herméticos bloqueiam download automático. Docs ativas descrevem o estado implementado; planos concluídos podem ir para histórico depois do cutover.
