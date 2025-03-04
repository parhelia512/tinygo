package reflectlite

import (
	"internal/itoa"
	"unsafe"
)

type Kind uint8

// Copied from reflect/type.go
// https://golang.org/src/reflect/type.go?s=8302:8316#L217
// These constants must match basicTypes and the typeKind* constants in
// compiler/interface.go
const (
	Invalid Kind = iota
	Bool
	Int
	Int8
	Int16
	Int32
	Int64
	Uint
	Uint8
	Uint16
	Uint32
	Uint64
	Uintptr
	Float32
	Float64
	Complex64
	Complex128
	String
	UnsafePointer
	Chan
	Interface
	Pointer
	Slice
	Array
	Func
	Map
	Struct
)

// Ptr is the old name for the Pointer kind.
const Ptr = Pointer

func (k Kind) String() string {
	switch k {
	case Invalid:
		return "invalid"
	case Bool:
		return "bool"
	case Int:
		return "int"
	case Int8:
		return "int8"
	case Int16:
		return "int16"
	case Int32:
		return "int32"
	case Int64:
		return "int64"
	case Uint:
		return "uint"
	case Uint8:
		return "uint8"
	case Uint16:
		return "uint16"
	case Uint32:
		return "uint32"
	case Uint64:
		return "uint64"
	case Uintptr:
		return "uintptr"
	case Float32:
		return "float32"
	case Float64:
		return "float64"
	case Complex64:
		return "complex64"
	case Complex128:
		return "complex128"
	case String:
		return "string"
	case UnsafePointer:
		return "unsafe.Pointer"
	case Chan:
		return "chan"
	case Interface:
		return "interface"
	case Pointer:
		return "ptr"
	case Slice:
		return "slice"
	case Array:
		return "array"
	case Func:
		return "func"
	case Map:
		return "map"
	case Struct:
		return "struct"
	default:
		return "kind" + itoa.Itoa(int(int8(k)))
	}
}

type Type interface {
	Name() string
	PkgPath() string
	Size() uintptr
	Kind() Kind
	Implements(u Type) bool
	AssignableTo(u Type) bool
	Comparable() bool
	String() string
	Elem() Type
}

// Constants for the 'meta' byte.
// These constants are also defined in the reflect package.
const (
	kindMask       = 31  // mask to apply to the meta byte to get the Kind value
	flagNamed      = 32  // flag that is set if this is a named type
	flagComparable = 64  // flag that is set if this type is comparable
	flagIsBinary   = 128 // flag that is set if this type uses the hashmap binary algorithm
)

// The below types (rawType, elemType, etc) are also defined in the reflect
// package and must match the compiler output.

type rawType struct {
	meta uint8
}

type elemType struct {
	rawType
	numMethod uint16
	ptrTo     *rawType
	elem      *rawType
}

type ptrType struct {
	rawType
	numMethod uint16
	elem      *rawType
}

type interfaceType struct {
	rawType
	ptrTo *rawType
}

type arrayType struct {
	rawType
	numMethod uint16
	ptrTo     *rawType
	elem      *rawType
	arrayLen  uintptr
	slicePtr  *rawType
}

type namedType struct {
	rawType
	numMethod uint16
	ptrTo     *rawType
	elem      *rawType
	pkg       *byte
	name      [1]byte
}

type structType struct {
	rawType
	numMethod uint16
	ptrTo     *rawType
	pkgpath   *byte
	size      uint32
	numField  uint16
	fields    [1]structField // the remaining fields are all of type structField
}

type structField struct {
	fieldType *rawType
	data      unsafe.Pointer // various bits of information, packed in a byte array
}

func TypeOf(i interface{}) Type {
	if i == nil {
		return nil
	}
	typecode, _ := decomposeInterface(i)
	return (*rawType)(typecode)
}

func (t *rawType) ptrtag() uintptr {
	return uintptr(unsafe.Pointer(t)) & 0b11
}

func (t *rawType) isNamed() bool {
	if tag := t.ptrtag(); tag != 0 {
		return false
	}

	return t.meta&flagNamed != 0
}

func (t *rawType) underlying() *rawType {
	if t.isNamed() {
		return (*elemType)(unsafe.Pointer(t)).elem
	}
	return t
}

func (t *rawType) arrayLen() uintptr {
	return (*arrayType)(unsafe.Pointer(t.underlying())).arrayLen
}

// Return the size (in bytes) of the given type.
func typeSize(t *rawType) uintptr {
	switch t.Kind() {
	case Bool, Int8, Uint8:
		return 1
	case Int16, Uint16:
		return 2
	case Int32, Uint32:
		return 4
	case Int64, Uint64:
		return 8
	case Int, Uint:
		return unsafe.Sizeof(int(0))
	case Uintptr:
		return unsafe.Sizeof(uintptr(0))
	case Float32:
		return 4
	case Float64:
		return 8
	case Complex64:
		return 8
	case Complex128:
		return 16
	case String:
		return unsafe.Sizeof("")
	case UnsafePointer, Chan, Map, Pointer:
		return unsafe.Sizeof(uintptr(0))
	case Slice:
		return unsafe.Sizeof([]int{})
	case Interface:
		return unsafe.Sizeof(interface{}(nil))
	case Func:
		var f func()
		return unsafe.Sizeof(f)
	case Array:
		return typeElem(t).Size() * t.arrayLen()
	case Struct:
		u := t.underlying()
		return uintptr((*structType)(unsafe.Pointer(u)).size)
	default:
		panic("unimplemented: size of type")
	}
}

// Return the type kind of the given type.
func typeKind(t *rawType) Kind {
	if t == nil {
		return Invalid
	}

	if tag := t.ptrtag(); tag != 0 {
		return Pointer
	}

	return Kind(t.meta & kindMask)
}

// Return the element type given a type. Panics if this type doesn't have an
// element type.
func typeElem(t *rawType) *rawType {
	if tag := t.ptrtag(); tag != 0 {
		return (*rawType)(unsafe.Add(unsafe.Pointer(t), -1))
	}

	underlying := t.underlying()
	switch underlying.Kind() {
	case Pointer:
		return (*ptrType)(unsafe.Pointer(underlying)).elem
	case Chan, Slice, Array, Map:
		return (*elemType)(unsafe.Pointer(underlying)).elem
	default:
		panic(errTypeElem)
	}
}

func typeNumMethod(t *rawType) int {
	if t.isNamed() {
		return int((*namedType)(unsafe.Pointer(t)).numMethod)
	}

	switch t.Kind() {
	case Pointer:
		return int((*ptrType)(unsafe.Pointer(t)).numMethod)
	case Struct:
		return int((*structType)(unsafe.Pointer(t)).numMethod)
	case Interface:
		//FIXME: Use len(methods)
		return typeNumMethod((*interfaceType)(unsafe.Pointer(t)).ptrTo)
	}

	// Other types have no methods attached.  Note we don't panic here.
	return 0
}

func typeAssignableTo(t, u *rawType) bool {
	if t == u {
		return true
	}

	if t.underlying() == u.underlying() && (!t.isNamed() || !u.isNamed()) {
		return true
	}

	if u.Kind() == Interface && typeNumMethod(u) == 0 {
		return true
	}

	if u.Kind() == Interface {
		panic("reflect: unimplemented: AssignableTo with interface")
	}
	return false
}

func (t *rawType) Name() string    { panic("todo: internal/reflectlite.Type.Name") }
func (t *rawType) PkgPath() string { panic("todo: internal/reflectlite.Type.PkgPath") }

func (t *rawType) Size() uintptr {
	return typeSize(t)
}

func (t *rawType) Kind() Kind {
	return typeKind(t)
}

func (t *rawType) Implements(u Type) bool {
	uraw := u.(*rawType)
	if uraw.Kind() != Interface {
		panic("reflect: non-interface type passed to Type.Implements")
	}
	return typeAssignableTo(t, uraw)
}

func (t *rawType) AssignableTo(u Type) bool {
	return typeAssignableTo(t, u.(*rawType))
}

func (t *rawType) Comparable() bool {
	return (t.meta & flagComparable) == flagComparable
}

func (t *rawType) String() string { panic("todo: internal/reflectlite.Type.String") }

func (t *rawType) Elem() Type {
	return typeElem(t)
}
