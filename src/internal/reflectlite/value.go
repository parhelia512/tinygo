package reflectlite

import "unsafe"

type valueFlags uint8

// These flags are shared with the reflect package.
const (
	valueFlagIndirect valueFlags = 1 << iota
	valueFlagExported
	valueFlagEmbedRO
	valueFlagStickyRO

	valueFlagRO = valueFlagEmbedRO | valueFlagStickyRO
)

type Value struct {
	typecode *rawType
	value    unsafe.Pointer
	flags    valueFlags
}

//go:linkname composeInterface runtime.composeInterface
func composeInterface(unsafe.Pointer, unsafe.Pointer) interface{}

//go:linkname decomposeInterface runtime.decomposeInterface
func decomposeInterface(i interface{}) (unsafe.Pointer, unsafe.Pointer)

func ValueOf(i interface{}) Value {
	typecode, value := decomposeInterface(i)
	return Value{
		typecode: (*rawType)(typecode),
		value:    value,
		flags:    valueFlagExported,
	}
}

func (v Value) isIndirect() bool {
	return v.flags&valueFlagIndirect != 0
}

func (v Value) isRO() bool {
	return v.flags&(valueFlagRO) != 0
}

func (v Value) checkAddressable() {
	if !v.isIndirect() {
		panic("reflect: value is not addressable")
	}
}

func (v Value) checkRO() {
	if v.isRO() {
		panic("reflect: value is not settable")
	}
}

func (v Value) pointer() unsafe.Pointer {
	if v.isIndirect() {
		return *(*unsafe.Pointer)(v.value)
	}
	return v.value
}

func valueIsNil(v Value) bool {
	switch v.Kind() {
	case Chan, Map, Ptr, UnsafePointer:
		return v.pointer() == nil
	case Func:
		if v.value == nil {
			return true
		}
		fn := (*funcHeader)(v.value)
		return fn.Code == nil
	case Slice:
		if v.value == nil {
			return true
		}
		slice := (*sliceHeader)(v.value)
		return slice.data == nil
	case Interface:
		val := *(*interface{})(v.value)
		return val == nil
	default:
		panic(&ValueError{Method: "IsNil", Kind: v.Kind()})
	}
}

func valueLen(v Value) int {
	switch v.typecode.Kind() {
	case Array:
		return int(v.typecode.arrayLen())
	case Chan:
		return chanlen(v.pointer())
	case Map:
		return maplen(v.pointer())
	case Slice:
		return int((*sliceHeader)(v.value).len)
	case String:
		return int((*stringHeader)(v.value).len)
	default:
		panic(&ValueError{Method: "Len", Kind: v.Kind()})
	}
}

func valueElem(v Value) Value {
	switch v.Kind() {
	case Ptr:
		ptr := v.pointer()
		if ptr == nil {
			return Value{}
		}
		// Don't copy RO flags
		flags := (v.flags & (valueFlagIndirect | valueFlagExported)) | valueFlagIndirect
		return Value{
			typecode: typeElem(v.typecode),
			value:    ptr,
			flags:    flags,
		}
	case Interface:
		typecode, value := decomposeInterface(*(*interface{})(v.value))
		return Value{
			typecode: (*rawType)(typecode),
			value:    value,
			flags:    v.flags &^ valueFlagIndirect,
		}
	default:
		panic(&ValueError{Method: "Elem", Kind: v.Kind()})
	}
}

func valueSet(v, x Value) {
	v.checkAddressable()
	v.checkRO()
	if !x.typecode.AssignableTo(v.typecode) {
		panic("reflect.Value.Set: value of type " + x.typecode.String() + " cannot be assigned to type " + v.typecode.String())
	}

	if v.typecode.Kind() == Interface && x.typecode.Kind() != Interface {
		// move the value of x back into the interface, if possible
		if x.isIndirect() && x.typecode.Size() <= unsafe.Sizeof(uintptr(0)) {
			x.value = unsafe.Pointer(loadValue(x.value, x.typecode.Size()))
		}

		intf := composeInterface(unsafe.Pointer(x.typecode), x.value)
		x = Value{
			typecode: v.typecode,
			value:    unsafe.Pointer(&intf),
		}
	}

	size := v.typecode.Size()
	if size <= unsafe.Sizeof(uintptr(0)) && !x.isIndirect() {
		storeValue(v.value, size, uintptr(x.value))
	} else {
		memcpy(v.value, x.value, size)
	}
}

func (v Value) Type() Type {
	return v.typecode
}

func (v Value) IsNil() bool {
	return valueIsNil(v)
}

func (v Value) Elem() Value {
	return valueElem(v)
}

func (v Value) Set(x Value) {
	valueSet(v, x)
}

func (v Value) Kind() Kind {
	return v.typecode.Kind()
}

func (v Value) Len() int {
	return valueLen(v)
}

type funcHeader struct {
	Context unsafe.Pointer
	Code    unsafe.Pointer
}

type sliceHeader struct {
	data unsafe.Pointer
	len  uintptr
	cap  uintptr
}

type stringHeader struct {
	data unsafe.Pointer
	len  uintptr
}

//go:linkname memcpy runtime.memcpy
func memcpy(dst, src unsafe.Pointer, size uintptr)

//go:linkname maplen runtime.hashmapLen
func maplen(p unsafe.Pointer) int

//go:linkname chanlen runtime.chanLen
func chanlen(p unsafe.Pointer) int
