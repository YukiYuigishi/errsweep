package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"errsweep/analyzer"
)

func main() {
	singlechecker.Main(analyzer.Analyzer)
}
