package ifacecallerext

import (
	"ifacecallee"
	"ifaceexternal"
)

var _ = ifacecallee.NewTagRepo

func LookupExternal(repo ifaceexternal.ExternalFinder, id int) error { // want `LookupExternal returns sentinels: ifacecallee\.ErrTag, ifacecallee\.ErrTagBusy` `LookupExternal returns sentinels via \*ifacecallee\.TagRepo: ifacecallee\.ErrTag` `LookupExternal returns sentinels via \*ifacecallee\.TagRepoBusy: ifacecallee\.ErrTagBusy` LookupExternal:`SentinelFact\(ifacecallee\.ErrTag, ifacecallee\.ErrTagBusy\)`
	return repo.FindTag(id)
}
