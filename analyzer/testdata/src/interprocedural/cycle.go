package interprocedural

// cycleA と cycleB は互いに呼び合うが、無限ループしないことを確認するケース。
// どちらも直接 Sentinel を返さないため診断は出ない。
func cycleA() error {
	return cycleB()
}

func cycleB() error {
	return cycleA()
}
