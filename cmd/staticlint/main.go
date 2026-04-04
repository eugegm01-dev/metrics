package main

import (
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/asmdecl"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/buildtag"
	"golang.org/x/tools/go/analysis/passes/cgocall"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/framepointer"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/ifaceassert"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/sortslice"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/stringintconv"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/testinggoroutine"
	"golang.org/x/tools/go/analysis/passes/tests"
	"golang.org/x/tools/go/analysis/passes/timeformat"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"

	"github.com/kisielk/errcheck/errcheck"
	"github.com/timakin/bodyclose/passes/bodyclose"
	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/stylecheck"
)

func main() {
	// 1. Standard analyzers
	stdAnalyzers := []*analysis.Analyzer{
		asmdecl.Analyzer, assign.Analyzer, atomic.Analyzer, bools.Analyzer,
		buildtag.Analyzer, cgocall.Analyzer, composite.Analyzer, copylock.Analyzer,
		errorsas.Analyzer, framepointer.Analyzer, httpresponse.Analyzer, ifaceassert.Analyzer,
		loopclosure.Analyzer, lostcancel.Analyzer, nilfunc.Analyzer, printf.Analyzer,
		shift.Analyzer, sortslice.Analyzer, stdmethods.Analyzer, stringintconv.Analyzer,
		structtag.Analyzer, testinggoroutine.Analyzer, tests.Analyzer, timeformat.Analyzer,
		unmarshal.Analyzer, unreachable.Analyzer, unsafeptr.Analyzer, unusedresult.Analyzer,
	}

	// 2. All SA class analyzers from staticcheck
	var saAnalyzers []*analysis.Analyzer
	for _, a := range staticcheck.Analyzers {
		if len(a.Analyzer.Name) >= 2 && a.Analyzer.Name[:2] == "SA" {
			saAnalyzers = append(saAnalyzers, a.Analyzer)
		}
	}

	// 3. One ST class analyzer (ST1000 – package comment)
	var stAnalyzers []*analysis.Analyzer
	for _, a := range stylecheck.Analyzers {
		if a.Analyzer.Name == "ST1000" {
			stAnalyzers = append(stAnalyzers, a.Analyzer)
			break
		}
	}

	// 4. Public analyzers (bodyclose, errcheck)
	publicAnalyzers := []*analysis.Analyzer{
		bodyclose.Analyzer,
		errcheck.Analyzer,
	}

	// 5. Custom analyzer (exitcheck)
	customAnalyzers := []*analysis.Analyzer{
		ExitCheckAnalyzer,
	}

	// Combine all
	allAnalyzers := append(stdAnalyzers, saAnalyzers...)
	allAnalyzers = append(allAnalyzers, stAnalyzers...)
	allAnalyzers = append(allAnalyzers, publicAnalyzers...)
	allAnalyzers = append(allAnalyzers, customAnalyzers...)

	multichecker.Main(allAnalyzers...)
}
