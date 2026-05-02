package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

func main() {
	// ファイルをパースする
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "main.go", nil, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		return
	}

	// 構造体ごとにチェックする
	for _, decl := range node.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if structType, ok := typeSpec.Type.(*ast.StructType); ok {
						fmt.Printf("%s 構造体の情報:\n", typeSpec.Name)
						for _, field := range structType.Fields.List {
							if len(field.Names) > 0 {
								fmt.Printf("\tフィールド名: %s\n", field.Names[0].Name)
							} else {
								fmt.Println("\tフィールド名: (名前なし)")
							}
							fmt.Printf("\tフィールドの型: %s\n", field.Type)
							fmt.Printf("\t実装箇所: %s\n", fset.Position(field.Type.Pos()))
							if field.Tag != nil {
								fmt.Printf("\tタグ: %s\n", field.Tag.Value)
							}
							fmt.Println()
						}
					}
				}
			}
		}
	}

	// 関数宣言を反復処理する
	for _, decl := range node.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			fmt.Printf("関数名: %s\n", funcDecl.Name.Name)
			if funcDecl.Recv != nil && len(funcDecl.Recv.List) != 0 {
				fmt.Printf("レシーバー: %s\n", funcDecl.Recv.List[0].Type)
			}
			fmt.Printf("実装箇所: %s\n", fset.Position(funcDecl.Pos()))
			fmt.Println()
		}
	}
}

type Struct struct {
	Name    string
	Fields  []string
	Methods []string
}

type Method struct {
	Name   string
	Args   []string
	Return []string
}
