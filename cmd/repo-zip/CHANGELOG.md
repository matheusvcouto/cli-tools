# repo-zip changelog

## [1.0.0] - 2026-09-14

### Alterado

- **repo-zip:** Migrate to CLI Core and reserve --version for product version; use --suffix for archive suffixes

## [1.0.1] - 2026-09-14

### Corrigido

- **repo-zip:** Fix generated Nushell completions

## [1.1.0] - 2026-09-27

### Adicionado

- **repo-zip:** Implement confined Windows force and no-clobber archive publication

### Corrigido

- **repo-zip:** Open Windows Git bundle temp files with WRITE_DAC before applying the protected DACL
