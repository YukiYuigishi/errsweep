package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"err-analyze/analyzer"
)

func main() {
	singlechecker.Main(analyzer.Analyzer)
}
