package notify

type FakeErrorContexter struct {
	I int
}

func (f FakeErrorContexter) ErrorContext() map[string]interface{} {
	foo := make(map[string]interface{}, 5)
	foo["test"] = f.I
	return foo
}
