package analyzer

import "fmt"

// SentinelInfo は検出されたSentinel Errorの識別情報を保持する。
type SentinelInfo struct {
	PkgPath string // e.g. "io"
	Name    string // e.g. "EOF"
}

func (s SentinelInfo) String() string {
	if s.PkgPath == "" {
		return s.Name
	}
	// パッケージパスの最後のセグメントを使う（e.g. "io" → "io", "database/sql" → "sql"）
	pkg := s.PkgPath
	for i := len(pkg) - 1; i >= 0; i-- {
		if pkg[i] == '/' {
			pkg = pkg[i+1:]
			break
		}
	}
	return fmt.Sprintf("%s.%s", pkg, s.Name)
}

// SentinelFact は関数が返しうるSentinel Errorのセット。
// analysis.Fact として関数オブジェクトにエクスポートされる。
type SentinelFact struct {
	Errors []SentinelInfo
}

func (*SentinelFact) AFact() {}

func (f *SentinelFact) String() string {
	if len(f.Errors) == 0 {
		return "SentinelFact()"
	}
	s := "SentinelFact("
	for i, e := range f.Errors {
		if i > 0 {
			s += ", "
		}
		s += e.String()
	}
	return s + ")"
}
