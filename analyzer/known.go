package analyzer

// knownErrorMap は標準ライブラリの関数名 → Sentinel Errorのマッピング。
// キーは "pkg.Func" または "(*pkg.Type).Method" 形式。
var knownErrorMap = map[string][]SentinelInfo{
	"(*os.File).Read":           {{PkgPath: "io", Name: "EOF"}},
	"(*os.File).ReadAt":         {{PkgPath: "io", Name: "EOF"}},
	"(*database/sql.Row).Scan":  {{PkgPath: "database/sql", Name: "ErrNoRows"}},
	"(*database/sql.Rows).Next": {{PkgPath: "database/sql", Name: "ErrNoRows"}},
	"io.ReadAll":                {{PkgPath: "io", Name: "EOF"}},
	"io.ReadFull":               {{PkgPath: "io", Name: "ErrUnexpectedEOF"}, {PkgPath: "io", Name: "EOF"}},
	"io.Copy":                   {{PkgPath: "io", Name: "EOF"}},
}
