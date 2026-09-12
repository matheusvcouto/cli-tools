# Portabilidade de paths e testes

Este documento registra lições reutilizáveis por agentes e por outros projetos.
Ele complementa as regras canônicas do [`AGENTS.md`](../AGENTS.md).

## Identidade não é grafia

Um mesmo objeto de filesystem pode ter mais de uma representação textual. No
macOS, diretórios temporários frequentemente aparecem como `/var/...` para quem
os criou e como `/private/var/...` depois de `chdir` + `getcwd`. Mounts, links e
outros aliases produzem a mesma classe de problema em diferentes sistemas.

Portanto:

- `filepath.Clean` normaliza componentes textuais, não aliases físicos;
- `filepath.Abs` torna o path absoluto, mas não cria uma identidade canônica;
- `EvalSymlinks` muda a semântica quando o programa precisa rejeitar ou preservar
  symlinks e não deve ser usado como correção genérica;
- igualdade de strings serve para contratos lexicais, não para provar que dois
  paths existentes representam o mesmo objeto;
- para comparar dois objetos existentes, usar `os.Stat` nos dois e
  `os.SameFile`; usar `os.Lstat` quando o objeto link, e não seu alvo, faz parte
  da decisão de segurança.

Exemplo para diretórios existentes:

```go
func sameDirectory(a, b string) (bool, error) {
	aInfo, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	bInfo, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	return aInfo.IsDir() && bInfo.IsDir() && os.SameFile(aInfo, bInfo), nil
}
```

`os.SameFile` não substitui containment. Para provar que um caminho está dentro
de uma raiz, use uma API confinada como `os.Root` ou calcule um relativo com
`filepath.Rel` e recuse `..`; nunca use prefixo textual.

## Diretório de trabalho em testes

`os.Chdir` altera o cwd do processo. Em Unix, um teste que usa apenas
`os.Chdir` pode deixar `PWD` com a grafia anterior, e `os.Getwd` pode devolver
outra grafia física válida. Além disso, `chdir` é global ao processo: o teste
não pode rodar em paralelo e precisa restaurar o cwd mesmo quando falha.

Em projetos cuja versão mínima já ofereça `testing.T.Chdir`, prefira essa API.
Quando a compatibilidade declarada ainda inclui uma versão anterior, faça
`os.Chdir` com restauração por `defer` e escreva assertions por identidade
física quando esse for o contrato real. Não aumente a versão mínima apenas para
obter um helper de teste.

## Processo de correção

Quando CI expuser uma diferença de plataforma:

1. leia o log completo e registre nome do teste, SO, arquitetura e toolchain;
2. separe falha de produto de falha de assertion;
3. localize a função exata pelo nome, não por um trecho que possa se repetir;
4. revise o diff no contexto da função antes de testar;
5. adicione a menor regressão que modele a classe do problema;
6. execute format, compilação/testes, vet, shuffle e race em ambiente isolado;
7. confirme no runner nativo que originalmente falhou.

Cross-build prova apenas que o código compila. Diferenças de filesystem,
credenciais, locks e processos exigem execução nativa no sistema declarado como
suportado.
