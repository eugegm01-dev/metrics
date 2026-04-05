// Package staticlint implements a custom multichecker for the metrics project.
//
// # Usage
//
// To run the linter, execute:
//
//	go run cmd/staticlint/main.go ./...
//
// Or install and run:
//
//	go build -o staticlint cmd/staticlint/main.go
//	./staticlint ./...
//
// # Included analyzers
//
//  1. All standard analyzers from golang.org/x/tools/go/analysis/passes:
//     - asmdecl      : report mismatches between assembly files and Go declarations
//     - assign       : detect useless assignments
//     - atomic       : check for common mistakes using the sync/atomic package
//     - bools        : detect common mistakes involving boolean operators
//     - buildtag     : check that +build tags are valid
//     - cgocall      : detect violations of the cgo pointer passing rules
//     - composites   : check for unkeyed composite literals
//     - copylock     : detect locks erroneously passed by value
//     - errorsas     : report passing nil to errors.As
//     - framepointer : report assembly that clobbers the frame pointer
//     - httpresponse : check for mistakes using HTTP responses
//     - ifaceassert  : detect impossible interface-to-interface type assertions
//     - loopclosure  : check references to loop variables inside closures
//     - lostcancel   : detect failure to call cancel function returned by context.WithCancel
//     - nilfunc      : check for useless comparisons between functions and nil
//     - printf       : check consistency of Printf format strings and arguments
//     - shift        : check for shifts that exceed the width of an integer
//     - sortslice    : check for calls to sort.Slice that do not use a slice type
//     - stdmethods   : check signature of methods of well-known interfaces
//     - stringintconv: check for string(int) conversions
//     - structtag    : check struct field tags are well formed
//     - testinggoroutine: report calls to (*testing.T).Fatal from goroutines started by a test
//     - tests        : check for common mistaken usages of tests and examples
//     - timeformat   : check for calls to (time.Time).Format with a static format
//     - unmarshal    : report passing non-pointer or non-interface to unmarshal
//     - unreachable  : check for unreachable code
//     - unsafeptr    : check for misuse of unsafe.Pointer
//     - unusedresult : check for unused results of calls to pure functions
//
//  2. All SA class analyzers from staticcheck.io (staticcheck):
//     SA1xxx – various misuses of the standard library
//     SA2xxx – concurrency issues
//     SA3xxx – staticcheck's own checks
//     SA4xxx – correctness issues
//     SA5xxx – code that will not work correctly on all platforms
//     SA6xxx – performance issues
//     SA9xxx – bugs in the analysis itself
//
//  3. One additional analyzer from staticcheck.io (ST1000 – incorrect or missing package comment)
//     from the stylecheck category (ST).
//
//  4. Public analyzers:
//     - github.com/timakin/bodyclose  : checks that HTTP response bodies are closed
//     - github.com/kisielk/errcheck   : checks for unchecked errors
//
//  5. Custom analyzer: exitcheck
//     Forbids direct calls to os.Exit inside the main function of the main package.
//     Rationale: os.Exit terminates the program abruptly without running deferred functions,
//     which can lead to resource leaks and incomplete cleanup. In the main function,
//     returning an error and exiting with a non-zero status via a helper is preferred.
//
// # Running with custom analyzers
//
// The multichecker uses the `multichecker` package from `golang.org/x/tools/go/analysis/multichecker`
// and additionally registers custom analyzers.
//
// Example output when a violation is found:
//
//	cmd/server/main.go:15:2: direct call to os.Exit in main function is forbidden
//
// See each analyzer's documentation for detailed checks.
package main
