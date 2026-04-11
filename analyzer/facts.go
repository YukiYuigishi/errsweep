package analyzer

import "fmt"

// SentinelKind は SentinelInfo の種類を表す。
// KindVar: var Err* = ... のパッケージレベル Sentinel 変数
// KindType: error interface を実装したカスタム型（例: *NotFoundError）
type SentinelKind uint8

const (
	KindVar SentinelKind = iota
	KindType
)

// SentinelInfo は検出されたSentinel Errorの識別情報を保持する。
// Kind == KindVar なら Name はパッケージレベル変数名、
// Kind == KindType なら Name は型名で、Pointer が true の場合は *Name 相当。
type SentinelInfo struct {
	PkgPath string // e.g. "io"
	Name    string // e.g. "EOF" or "NotFoundError"
	Kind    SentinelKind
	Pointer bool // KindType 時のみ有効（pointer receiver の型か）
}

func (s SentinelInfo) String() string {
	pkg := shortPkgName(s.PkgPath)
	prefix := ""
	if s.Kind == KindType && s.Pointer {
		prefix = "*"
	}
	if pkg == "" {
		return prefix + s.Name
	}
	return fmt.Sprintf("%s%s.%s", prefix, pkg, s.Name)
}

// shortPkgName はインポートパスの最後のセグメントを返す。
// 例: "database/sql" → "sql", "io" → "io", "" → ""
func shortPkgName(pkgPath string) string {
	for i := len(pkgPath) - 1; i >= 0; i-- {
		if pkgPath[i] == '/' {
			return pkgPath[i+1:]
		}
	}
	return pkgPath
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
