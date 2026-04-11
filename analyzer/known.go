package analyzer

// knownErrorMap は標準ライブラリの関数 → Sentinel Errorのマッピング。
// キーは RelString(nil) の形式（"pkg.Func" または "(*pkg.Type).Method"）。
//
// 注意: io.Copy / io.ReadAll は内部で io.EOF を吸収して nil を返すため含めない。
var knownErrorMap = map[string][]SentinelInfo{
	// os: ファイル読み込みは EOF で終端を通知する
	"(*os.File).Read":   {{PkgPath: "io", Name: "EOF"}},
	"(*os.File).ReadAt": {{PkgPath: "io", Name: "EOF"}},
	"os.ReadFile": {
		{PkgPath: "io/fs", Name: "ErrNotExist"},
		{PkgPath: "io/fs", Name: "ErrPermission"},
	},
	"os.Stat": {
		{PkgPath: "io/fs", Name: "ErrNotExist"},
		{PkgPath: "io/fs", Name: "ErrPermission"},
	},

	// bufio: デリミタ/バイト単位読み込みは EOF を返しうる
	"(*bufio.Reader).Read":       {{PkgPath: "io", Name: "EOF"}},
	"(*bufio.Reader).ReadString": {{PkgPath: "io", Name: "EOF"}},
	"(*bufio.Reader).ReadBytes":  {{PkgPath: "io", Name: "EOF"}},
	"(*bufio.Reader).ReadByte":   {{PkgPath: "io", Name: "EOF"}},
	"(*bufio.Reader).ReadRune":   {{PkgPath: "io", Name: "EOF"}},
	"(*bufio.Reader).ReadLine":   {{PkgPath: "io", Name: "EOF"}},

	// io: ReadFull は短い読み込みを区別して返す
	"io.ReadFull": {
		{PkgPath: "io", Name: "EOF"},
		{PkgPath: "io", Name: "ErrUnexpectedEOF"},
	},

	// io: SectionReader は範囲末尾で EOF を返す
	"(*io.SectionReader).Read":   {{PkgPath: "io", Name: "EOF"}},
	"(*io.SectionReader).ReadAt": {{PkgPath: "io", Name: "EOF"}},
	"(*io.LimitedReader).Read":   {{PkgPath: "io", Name: "EOF"}},
	"(*io.PipeReader).Read":      {{PkgPath: "io", Name: "EOF"}},

	// database/sql: 行が存在しない場合に ErrNoRows を返す
	"(*database/sql.Row).Scan": {{PkgPath: "database/sql", Name: "ErrNoRows"}},
	"(*database/sql.Row).Err":  {{PkgPath: "database/sql", Name: "ErrNoRows"}},

	// net/http: リクエスト補助APIで返りうる sentinel
	"(*net/http.Request).Cookie":   {{PkgPath: "net/http", Name: "ErrNoCookie"}},
	"(*net/http.Request).FormFile": {{PkgPath: "net/http", Name: "ErrMissingFile"}},
}
