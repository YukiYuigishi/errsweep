package ifaceexternal

type ExternalFinder interface {
	FindTag(id int) error
}
