package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	dirname := "./"

	if len(os.Args) > 1 {
		dirname = os.Args[1]
	}

	fmt.Printf("指定されたディレクトリ: %s\n", dirname)

	paths := []string{}

	if ferr := filepath.Walk(dirname, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			fmt.Println(path)
			paths = append(paths, path)
		}
		return nil
	}); ferr != nil {
		fmt.Printf("error walking the path %v: %v\n", dirname, ferr)
		return
	}

	for _, path := range paths {
		fmt.Printf("path: %s\n", path)
		CheckEmbeddedPointerStructs(path)
	}
}

func CheckEmbeddedPointerStructs(
	filename string,
) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		return
	}

	ast.Inspect(f, func(n ast.Node) bool {
		t, ok := n.(*ast.TypeSpec)
		if !ok || t.Type == nil {
			return true
		}

		s, ok := t.Type.(*ast.StructType)
		if !ok {
			return true
		}

		for _, field := range s.Fields.List {
			if len(field.Names) == 0 { // embedded field
				switch typ := field.Type.(type) {
				case *ast.Ident:
					fmt.Printf("struct %s has embedded struct %s\n", t.Name.Name, typ.Name)
				case *ast.StarExpr: // pointer to something
					if ident, ok := typ.X.(*ast.Ident); ok {
						fmt.Printf("struct %s has embedded pointer to struct %s\n", t.Name.Name, ident.Name)
					}
				default:
					fmt.Printf("unknown type %T\n", typ)
				}
			}
		}

		return true
	})
}
