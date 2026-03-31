package nilreturn

func AlwaysNil() error {
	return nil
}

func MaybeNil(ok bool) error {
	if ok {
		return nil
	}
	return nil
}
