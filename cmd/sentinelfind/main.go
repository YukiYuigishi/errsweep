package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/YukiYuigishi/errsweep/analyzer"
)

func main() {
	singlechecker.Main(analyzer.Analyzer)
}
