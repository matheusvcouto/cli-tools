package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"sort"
	"strings"
)

const publicAPIContractSchemaVersion = 1

type publicAPIContract struct {
	SchemaVersion int               `json:"schema_version"`
	Package       string            `json:"package"`
	Symbols       []publicAPISymbol `json:"symbols"`
}

type publicAPISymbol struct {
	ID          string `json:"id"`
	Declaration string `json:"declaration"`
}

func generatePublicAPIContract(dir string) (publicAPIContract, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		return publicAPIContract{}, err
	}
	if len(pkgs) != 1 {
		return publicAPIContract{}, fmt.Errorf("expected exactly one package in %s, found %d", dir, len(pkgs))
	}
	var pkg *ast.Package
	for _, item := range pkgs {
		pkg = item
	}

	contract := publicAPIContract{SchemaVersion: publicAPIContractSchemaVersion, Package: pkg.Name}
	fileNames := make([]string, 0, len(pkg.Files))
	for name := range pkg.Files {
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)
	for _, fileName := range fileNames {
		file := pkg.Files[fileName]
		for _, decl := range file.Decls {
			symbols, err := apiSymbolsFromDecl(fset, decl)
			if err != nil {
				return publicAPIContract{}, fmt.Errorf("%s: %w", fileName, err)
			}
			contract.Symbols = append(contract.Symbols, symbols...)
		}
	}
	sort.Slice(contract.Symbols, func(i, j int) bool {
		if contract.Symbols[i].ID != contract.Symbols[j].ID {
			return contract.Symbols[i].ID < contract.Symbols[j].ID
		}
		return contract.Symbols[i].Declaration < contract.Symbols[j].Declaration
	})
	unique := contract.Symbols[:0]
	for _, symbol := range contract.Symbols {
		if len(unique) == 0 || unique[len(unique)-1].ID != symbol.ID {
			unique = append(unique, symbol)
			continue
		}
		if unique[len(unique)-1].Declaration != symbol.Declaration {
			return publicAPIContract{}, fmt.Errorf("platform-specific public API symbol %q has incompatible declarations %q and %q", symbol.ID, unique[len(unique)-1].Declaration, symbol.Declaration)
		}
	}
	contract.Symbols = unique
	return contract, nil
}

func apiSymbolsFromDecl(fset *token.FileSet, decl ast.Decl) ([]publicAPISymbol, error) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if !ast.IsExported(d.Name.Name) {
			return nil, nil
		}
		id := "func " + d.Name.Name
		if d.Recv != nil {
			receiver := receiverTypeName(d.Recv)
			if receiver == "" || !ast.IsExported(receiver) {
				return nil, nil
			}
			id = "method " + receiver + "." + d.Name.Name
		}
		clone := *d
		clone.Doc = nil
		clone.Body = nil
		clone.Recv = stripFieldNames(d.Recv)
		clone.Type = canonicalFuncType(d.Type)
		text, err := printNode(fset, &clone)
		if err != nil {
			return nil, err
		}
		return []publicAPISymbol{{ID: id, Declaration: text}}, nil
	case *ast.GenDecl:
		var out []publicAPISymbol
		var inheritedConstType ast.Expr
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				if !ast.IsExported(s.Name.Name) {
					continue
				}
				clone := *s
				clone.Doc = nil
				clone.Comment = nil
				clone.Type = publicTypeExpr(s.Type)
				text, err := printNode(fset, &clone)
				if err != nil {
					return nil, err
				}
				out = append(out, publicAPISymbol{ID: "type " + s.Name.Name, Declaration: "type " + text})
			case *ast.ValueSpec:
				typeExpr := s.Type
				if d.Tok == token.CONST {
					if len(s.Values) == 0 {
						typeExpr = inheritedConstType
					} else {
						inheritedConstType = s.Type
					}
				}
				for _, name := range s.Names {
					if !ast.IsExported(name.Name) {
						continue
					}
					kind := strings.ToLower(d.Tok.String())
					declaration := kind + " " + name.Name
					if typeExpr != nil {
						typeText, err := printNode(fset, typeExpr)
						if err != nil {
							return nil, err
						}
						declaration += " " + typeText
					}
					out = append(out, publicAPISymbol{ID: kind + " " + name.Name, Declaration: declaration})
				}
			}
		}
		return out, nil
	default:
		return nil, nil
	}
}

func stripFieldNames(list *ast.FieldList) *ast.FieldList {
	if list == nil {
		return nil
	}
	clone := *list
	clone.List = make([]*ast.Field, 0, len(list.List))
	for _, field := range list.List {
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for range count {
			copyField := *field
			copyField.Doc = nil
			copyField.Comment = nil
			copyField.Names = nil
			clone.List = append(clone.List, &copyField)
		}
	}
	return &clone
}

func canonicalFuncType(fn *ast.FuncType) *ast.FuncType {
	if fn == nil {
		return nil
	}
	clone := *fn
	// Type parameter names are intentionally preserved because their identifiers
	// can be referenced by parameter/result types. Ordinary parameter/result
	// names are not part of Go call compatibility and are stripped.
	clone.Params = stripFieldNames(fn.Params)
	clone.Results = stripFieldNames(fn.Results)
	return &clone
}

func publicTypeExpr(expr ast.Expr) ast.Expr {
	st, ok := expr.(*ast.StructType)
	if !ok {
		return expr
	}
	clone := *st
	fields := make([]*ast.Field, 0, len(st.Fields.List))
	for _, field := range st.Fields.List {
		copyField := *field
		copyField.Doc = nil
		copyField.Comment = nil
		if len(field.Names) == 0 {
			// Embedded fields affect the promoted method/field surface and are
			// therefore part of the public compatibility contract.
			fields = append(fields, &copyField)
			continue
		}
		var names []*ast.Ident
		for _, name := range field.Names {
			if ast.IsExported(name.Name) {
				names = append(names, ast.NewIdent(name.Name))
			}
		}
		if len(names) == 0 {
			continue
		}
		copyField.Names = names
		fields = append(fields, &copyField)
	}
	clone.Fields = &ast.FieldList{List: fields}
	return &clone
}

func receiverTypeName(fields *ast.FieldList) string {
	if fields == nil || len(fields.List) != 1 {
		return ""
	}
	expr := fields.List[0].Type
	for {
		switch x := expr.(type) {
		case *ast.StarExpr:
			expr = x.X
		case *ast.IndexExpr:
			expr = x.X
		case *ast.IndexListExpr:
			expr = x.X
		case *ast.Ident:
			return x.Name
		default:
			return ""
		}
	}
}

func printNode(fset *token.FileSet, node any) (string, error) {
	var b bytes.Buffer
	if err := printer.Fprint(&b, fset, node); err != nil {
		return "", err
	}
	return strings.TrimSpace(b.String()), nil
}

func marshalPublicAPIContract(contract publicAPIContract) ([]byte, error) {
	raw, err := json.MarshalIndent(contract, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func loadPublicAPIContract(path string) (publicAPIContract, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return publicAPIContract{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var contract publicAPIContract
	if err := dec.Decode(&contract); err != nil {
		return publicAPIContract{}, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return publicAPIContract{}, fmt.Errorf("API contract must contain exactly one JSON object")
		}
		return publicAPIContract{}, err
	}
	if contract.SchemaVersion != publicAPIContractSchemaVersion {
		return publicAPIContract{}, fmt.Errorf("unsupported public API contract schema_version %d", contract.SchemaVersion)
	}
	if strings.TrimSpace(contract.Package) == "" {
		return publicAPIContract{}, fmt.Errorf("public API contract package is required")
	}
	seen := map[string]struct{}{}
	for _, symbol := range contract.Symbols {
		if strings.TrimSpace(symbol.ID) == "" || strings.TrimSpace(symbol.Declaration) == "" {
			return publicAPIContract{}, fmt.Errorf("public API contract contains an empty symbol")
		}
		if _, ok := seen[symbol.ID]; ok {
			return publicAPIContract{}, fmt.Errorf("public API contract contains duplicate symbol %q", symbol.ID)
		}
		seen[symbol.ID] = struct{}{}
	}
	return contract, nil
}

func checkPublicAPICompatibility(baseline, current publicAPIContract) error {
	if baseline.Package != current.Package {
		return fmt.Errorf("public API package changed from %q to %q", baseline.Package, current.Package)
	}
	currentByID := make(map[string]publicAPISymbol, len(current.Symbols))
	for _, symbol := range current.Symbols {
		currentByID[symbol.ID] = symbol
	}
	var breaking []string
	for _, old := range baseline.Symbols {
		now, ok := currentByID[old.ID]
		if !ok {
			breaking = append(breaking, old.ID+" was removed")
			continue
		}
		if now.Declaration != old.Declaration {
			breaking = append(breaking, fmt.Sprintf("%s changed from %q to %q", old.ID, old.Declaration, now.Declaration))
		}
	}
	if len(breaking) != 0 {
		sort.Strings(breaking)
		return fmt.Errorf("breaking public Go API change: %s", strings.Join(breaking, "; "))
	}
	return nil
}

func publicAPIAdditions(baseline, current publicAPIContract) []string {
	locked := make(map[string]struct{}, len(baseline.Symbols))
	for _, symbol := range baseline.Symbols {
		locked[symbol.ID] = struct{}{}
	}
	var additions []string
	for _, symbol := range current.Symbols {
		if _, ok := locked[symbol.ID]; !ok {
			additions = append(additions, symbol.ID)
		}
	}
	sort.Strings(additions)
	return additions
}

func verifyPublicAPILock(dir, lock string) (int, error) {
	current, err := generatePublicAPIContract(dir)
	if err != nil {
		return 0, fmt.Errorf("generate: %w", err)
	}
	baseline, err := loadPublicAPIContract(lock)
	if err != nil {
		return 0, fmt.Errorf("baseline: %w", err)
	}
	if err := checkPublicAPICompatibility(baseline, current); err != nil {
		return 0, err
	}
	if additions := publicAPIAdditions(baseline, current); len(additions) != 0 {
		return 0, fmt.Errorf("public API lock is missing additive symbols: %s; run `go run ./tools/release api write` and review the lock", strings.Join(additions, ", "))
	}
	return len(baseline.Symbols), nil
}

func runAPICommand(args []string) {
	if len(args) == 0 || (args[0] != "check" && args[0] != "write") {
		fatalf("api: expected subcommand check or write")
	}
	mode := args[0]
	fs := flagSet("release api " + mode)
	var dir, lock string
	var allowBreaking bool
	fs.StringVar(&dir, "dir", "cli", "directory containing the public Go package")
	fs.StringVar(&lock, "lock", "cli/api.contract.json", "public Go API compatibility lock")
	if mode == "write" {
		fs.BoolVar(&allowBreaking, "allow-breaking", false, "replace an existing lock after an intentional major API break")
	}
	if err := fs.Parse(args[1:]); err != nil {
		fatalf("api %s flags: %v", mode, err)
	}
	if fs.NArg() != 0 {
		fatalf("api %s: unexpected arguments: %v", mode, fs.Args())
	}
	current, err := generatePublicAPIContract(dir)
	if err != nil {
		fatalf("api %s generate: %v", mode, err)
	}
	if mode == "write" {
		if baseline, err := loadPublicAPIContract(lock); err == nil {
			if compatErr := checkPublicAPICompatibility(baseline, current); compatErr != nil && !allowBreaking {
				fatalf("api write: %v; use --allow-breaking only for an intentional major-version API break", compatErr)
			}
		} else if !os.IsNotExist(err) {
			fatalf("api write baseline: %v", err)
		}
		raw, err := marshalPublicAPIContract(current)
		if err != nil {
			fatalf("api write encode: %v", err)
		}
		if err := writeFileAtomic(lock, raw, 0o644); err != nil {
			fatalf("api write: %v", err)
		}
		fmt.Printf("wrote public Go API lock with %d symbol(s)\n", len(current.Symbols))
		return
	}
	locked, err := verifyPublicAPILock(dir, lock)
	if err != nil {
		fatalf("api check: %v", err)
	}
	fmt.Printf("public Go API matches %d locked symbol(s) and contains no breaking drift\n", locked)
}

// flagSet centralizes the quiet FlagSet convention used by release subcommands.
func flagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}
