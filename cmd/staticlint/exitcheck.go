package main

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var ExitCheckAnalyzer = &analysis.Analyzer{
	Name: "exitcheck",
	Doc:  "check for direct os.Exit calls in main function of main package",
	Run:  runExitCheck,
}

func runExitCheck(pass *analysis.Pass) (interface{}, error) {
	if pass.Pkg.Name() != "main" {
		return nil, nil
	}
	for _, file := range pass.Files {
		var mainFunc *ast.FuncDecl
		ast.Inspect(file, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "main" {
				mainFunc = fn
				return false
			}
			return true
		})
		if mainFunc == nil {
			continue
		}
		ast.Inspect(mainFunc.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "os" && sel.Sel.Name == "Exit" {
					pass.Reportf(call.Pos(), "direct call to os.Exit in main function is forbidden")
				}
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "log" && strings.HasPrefix(sel.Sel.Name, "Fatal") {
					pass.Reportf(call.Pos(), "call to log.Fatal* in main function is forbidden (indirect os.Exit)")
				}
			}
			return true
		})
	}
	return nil, nil
}
