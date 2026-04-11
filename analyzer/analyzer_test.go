package analyzer_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/YukiYuigishi/errsweep/analyzer"
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

func TestAnalyzer_Deferred(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer.Analyzer, "deferred")
}

func TestAnalyzer_Opaque(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer.Analyzer, "opaque")
}

func TestAnalyzer_Interprocedural(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer.Analyzer, "interprocedural")
}

func TestAnalyzer_Stdlib(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer.Analyzer, "stdlib")
}

func TestAnalyzer_CrossPackage(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer.Analyzer, "callee", "caller")
}

func TestAnalyzer_FuncVar(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer.Analyzer, "funcvar")
}

func TestAnalyzer_IfaceImpl(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer.Analyzer, "ifacecallee", "ifacecaller")
}

func TestAnalyzer_CustomType(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer.Analyzer, "customtype")
}

func TestAnalyzer_NonWrapFmtErrorf(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer.Analyzer, "nonwrap")
}

func TestAnalyzer_IfaceExternal(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer.Analyzer, "ifaceexternal", "ifacecallee", "ifacecallerext")
}

func TestAnalyzer_IfaceParamFlow(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer.Analyzer, "ifaceparam")
}

func TestAnalyzer_RealWorld(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, analyzer.Analyzer, "realworld")
}
