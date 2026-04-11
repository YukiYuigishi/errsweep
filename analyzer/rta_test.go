package analyzer

import (
	"testing"

	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/ssa"
)

func TestSafeAnalyzeRTA_RecoversPanic(t *testing.T) {
	prev := analyzeRTA
	t.Cleanup(func() { analyzeRTA = prev })

	analyzeRTA = func(_ []*ssa.Function, _ bool) *rta.Result {
		panic("boom")
	}

	if got := safeAnalyzeRTA(nil); got != nil {
		t.Fatalf("safeAnalyzeRTA() = %v, want nil on panic", got)
	}
}
