package analyzer_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"err-analyze/analyzer"
)

func TestAnalyzer_Basic(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer.Analyzer, "basic")
}

func TestAnalyzer_Wrapped(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer.Analyzer, "wrapped")
}

func TestAnalyzer_Phi(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer.Analyzer, "phi")
}

func TestAnalyzer_NilReturn(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer.Analyzer, "nilreturn")
}

func TestAnalyzer_Opaque(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer.Analyzer, "opaque")
}

func TestAnalyzer_Interprocedural(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer.Analyzer, "interprocedural")
}

func TestAnalyzer_CrossPackage(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer.Analyzer, "callee", "caller")
}
