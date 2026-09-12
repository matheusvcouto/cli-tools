# Referências oficiais

Consultadas na revisão do plano em 2026-09-11.

- Go release history / 1.27.1: https://go.dev/doc/devel/release
- Go modules/toolchain: https://go.dev/doc/toolchain
- `os.Root`: https://pkg.go.dev/os#Root
- `os.Rename`: https://pkg.go.dev/os#Rename
- Race detector: https://go.dev/doc/articles/race_detector
- `archive/zip`: https://pkg.go.dev/archive/zip
- mise GitHub backend: https://mise.jdx.dev/dev-tools/backends/github.html

Pontos importantes usados no plano:

- `os.Root` oferece operações confinadas, incluindo `Lstat`, `Readlink`, `RemoveAll` e `Rename`;
- a documentação de `os.Rename` não promete atomicidade equivalente em plataformas não-Unix;
- o race detector oficial exige requisitos de CGO/toolchain conforme a plataforma;
- mise encontra `bin/` dentro de archives e pode expor múltiplos executáveis.
