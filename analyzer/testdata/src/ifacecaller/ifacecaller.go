package ifacecaller

import (
	"ifacecallee"
)

// TagFinder は interface DI 用のローカル interface。
type TagFinder interface {
	FindTag(id int) error
}

// compile-time assertion: *TagRepo は明示的に宣言する。
// *TagRepoBusy は宣言を書かないが、オートディスカバリで拾われる。
var _ TagFinder = (*ifacecallee.TagRepo)(nil)

// Lookup は interface 経由で FindTag を呼ぶ。
// ifacecallee の 2 つの具象が両方 TagFinder を満たすため、
// アナライザは union の合算ラインに加えて concrete ごとの
// 内訳ラインも emit する。
func Lookup(repo TagFinder, id int) error { // want `Lookup returns sentinels: ifacecallee\.ErrTag, ifacecallee\.ErrTagBusy` `Lookup returns sentinels via \*ifacecallee\.TagRepo: ifacecallee\.ErrTag` `Lookup returns sentinels via \*ifacecallee\.TagRepoBusy: ifacecallee\.ErrTagBusy` Lookup:`SentinelFact\(ifacecallee\.ErrTag, ifacecallee\.ErrTagBusy\)`
	return repo.FindTag(id)
}
