// Command check_cache_projections verifies that every cache module name a
// package under alita/ builds through cache.CacheKey is declared as a
// cache.Projection in that same package. Key shapes live in the domain's
// projections.go; a CacheKey literal that bypasses the declaration would
// drift from the projections backup and invalidation enumerate.
//
// The check is per directory (one Go package): used names must be a subset
// of declared names. Test files and the cache package itself are excluded —
// tests may build keys directly, and keys.go defines the mechanism.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const root = "alita"

// skipDir reports whether dir holds files the check does not cover.
func skipDir(dir string) bool {
	return dir == filepath.Join(root, "db", "cache")
}

type packageFacts struct {
	declared map[string]bool
	used     map[string]bool
}

func main() {
	fset := token.NewFileSet()
	pkgs := map[string]*packageFacts{}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if skipDir(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		facts := pkgs[filepath.Dir(path)]
		if facts == nil {
			facts = &packageFacts{declared: map[string]bool{}, used: map[string]bool{}}
			pkgs[filepath.Dir(path)] = facts
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		scanFile(file, facts)
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var problems []string
	for dir, facts := range pkgs {
		var missing []string
		for name := range facts.used {
			if !facts.declared[name] {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			problems = append(problems, fmt.Sprintf("%s: CacheKey uses undeclared projection(s) %s", dir, strings.Join(missing, ", ")))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, p)
		}
		fmt.Fprintln(os.Stderr, "declare them as cache.Projection in the package's projections.go")
		os.Exit(1)
	}
}

// scanFile collects declared projection names and CacheKey module arguments.
func scanFile(file *ast.File, facts *packageFacts) {
	// String constants resolve first arguments that name a module indirectly.
	consts := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if i >= len(vs.Values) {
					break
				}
				if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if s, err := strconv.Unquote(lit.Value); err == nil {
						consts[name.Name] = s
					}
				}
			}
		}
	}

	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.CompositeLit:
			// cache.Projection{Name: "module"} declares a key shape.
			sel, ok := n.Type.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Projection" {
				return true
			}
			for _, elt := range n.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := kv.Key.(*ast.Ident)
				if !ok || key.Name != "Name" {
					continue
				}
				switch v := kv.Value.(type) {
				case *ast.BasicLit:
					if v.Kind == token.STRING {
						if s, err := strconv.Unquote(v.Value); err == nil {
							facts.declared[s] = true
						}
					}
				case *ast.Ident:
					if s, ok := consts[v.Name]; ok {
						facts.declared[s] = true
					}
				}
			}
		case *ast.CallExpr:
			// cache.CacheKey("module", ...) / dbcache.CacheKey("module", ...)
			// builds a key outside the declared projections.
			sel, ok := n.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "CacheKey" {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || (pkg.Name != "cache" && pkg.Name != "dbcache") {
				return true
			}
			if len(n.Args) == 0 {
				return true
			}
			switch arg := n.Args[0].(type) {
			case *ast.BasicLit:
				if arg.Kind == token.STRING {
					if s, err := strconv.Unquote(arg.Value); err == nil {
						facts.used[s] = true
					}
				}
			case *ast.Ident:
				if s, ok := consts[arg.Name]; ok {
					facts.used[s] = true
				}
			}
		}
		return true
	})
}
