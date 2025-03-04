package reflectlite

type TypeError struct {
	Method string
}

func (e *TypeError) Error() string {
	return "reflect: call of reflect.Type." + e.Method + " on invalid type"
}

var (
	errTypeElem = &TypeError{"Elem"}
)

type ValueError struct {
	Method string
	Kind   Kind
}

func (e *ValueError) Error() string {
	if e.Kind == 0 {
		return "reflect: call of " + e.Method + " on zero Value"
	}
	return "reflect: call of " + e.Method + " on " + e.Kind.String() + " Value"
}
