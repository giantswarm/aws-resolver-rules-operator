// Command importalias renames package import aliases in generated Go files since counterfeiter can't choose them
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

type aliasFlags map[string]string

func (importPathToAlias aliasFlags) Set(value string) error {
	importPath, alias, found := strings.Cut(value, "=")
	if !found || importPath == "" || alias == "" {
		return fmt.Errorf("expected <import path>=<alias>, got %q", value)
	}
	importPathToAlias[importPath] = alias
	return nil
}

func (importPathToAlias aliasFlags) String() string {
	return fmt.Sprint(map[string]string(importPathToAlias))
}

func goFilePaths(argument string) ([]string, error) {
	info, err := os.Stat(argument)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{argument}, nil
	}
	return filepath.Glob(filepath.Join(argument, "*.go"))
}

func main() {
	importPathToAlias := aliasFlags{}
	flag.Var(importPathToAlias, "alias", "`<import path>=<alias>` to enforce (repeatable)")
	flag.Parse()

	if len(importPathToAlias) == 0 || flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	for _, argument := range flag.Args() {
		filePaths, err := goFilePaths(argument)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for _, filePath := range filePaths {
			if err := renameImportAliases(filePath, importPathToAlias); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
	}
}

func renameImportAliases(filePath string, importPathToAlias map[string]string) error {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, filePath, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	oldNameToNewName := map[string]string{}
	for _, importSpec := range file.Imports {
		importPath, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil {
			return fmt.Errorf("failed to unquote import path in %q: %w", filePath, err)
		}
		alias, found := importPathToAlias[importPath]
		if !found {
			continue
		}

		// Assume the package name equals the last path element, which holds for the API versions this is used for
		oldName := path.Base(importPath)
		if importSpec.Name != nil {
			oldName = importSpec.Name.Name
		}
		if oldName == alias {
			continue
		}
		oldNameToNewName[oldName] = alias
		importSpec.Name = ast.NewIdent(alias)
	}
	if len(oldNameToNewName) == 0 {
		return nil
	}

	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		packageIdent, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		if newName, found := oldNameToNewName[packageIdent.Name]; found {
			packageIdent.Name = newName
		}
		return true
	})

	var formatted bytes.Buffer
	if err := format.Node(&formatted, fileSet, file); err != nil {
		return fmt.Errorf("failed to format %q: %w", filePath, err)
	}
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, formatted.Bytes(), fileInfo.Mode().Perm())
}
