package ifacecallee

import "errors"

var (
	ErrTag     = errors.New("tag not found")
	ErrTagBusy = errors.New("tag busy")
)

// TagRepo は ifacecaller パッケージが参照する具象型。
type TagRepo struct{}

// FindTag は Sentinel を返す実装。
// 別パッケージから interface 経由で呼ばれるため、
// Fact がエクスポートされている必要がある。
func (r *TagRepo) FindTag(id int) error { // want `FindTag returns sentinels: ifacecallee\.ErrTag` FindTag:`SentinelFact\(ifacecallee\.ErrTag\)`
	if id <= 0 {
		return ErrTag
	}
	return nil
}

// NewTagRepo はコンストラクタ。
func NewTagRepo() *TagRepo {
	return &TagRepo{}
}

// TagRepoBusy は TagFinder を満たす別の具象実装。
// compile-time assertion は書かないが、オートディスカバリで拾われるはず。
type TagRepoBusy struct{}

// FindTag は別種の Sentinel を返す。
func (r *TagRepoBusy) FindTag(id int) error { // want `FindTag returns sentinels: ifacecallee\.ErrTagBusy` FindTag:`SentinelFact\(ifacecallee\.ErrTagBusy\)`
	return ErrTagBusy
}
