package ffi

import (
	"errors"
	"fmt"
	"reflect"
	"unsafe"
)

//go:linkname funcPC runtime.funcPC
func funcPC(f interface{}) uintptr

//go:linkname cgocall runtime.cgocall
func cgocall(fn, arg unsafe.Pointer) int32

//go:linkname _Cgo_always_false runtime.cgoAlwaysFalse
var _Cgo_always_false bool

//go:linkname _Cgo_use runtime.cgoUse
func _Cgo_use(interface{})

type class int

const (
	classVoid = iota
	classInt
	classUint
	classFloat
)

type value struct {
	offset int
	c      class
	size   int
	align  int
}

// stackFields takes pointer to function variable and returns: pointer to set it, argument offsets and type, and return value and type
// Arguments in go are according to the following (from cmd/compile/internal/gc/align.go dowidth TFUNCARGS):
// 3 consecutive structures on the stack
// 1. struct: receiver argument(s)
// 2. struct (aligned to register width): parameters
// 3. struct (aligned to register width): return values
func stackFields(fun interface{}) (fptr unsafe.Pointer, arguments []value, ret value, err error) {
	v := reflect.ValueOf(fun)
	if v.Kind() != reflect.Ptr {
		err = errors.New("provided argument must be pointer to function variable")
		return
	}
	f := v.Elem().Type()
	if f.Kind() != reflect.Func {
		err = errors.New("provided argument must be pointer to function variable")
		return
	}
	if f.NumOut() > 1 {
		err = errors.New("only one or no return argument allowed")
		return
	}

	ret.c = classVoid

	offset := 0

	for i := 0; i < f.NumIn(); i++ {
		a := f.In(i)
		k := a.Kind()
		var v value

		skip := 0

		v.size = int(a.Size())
		v.align = a.Align()

		switch k {
		case reflect.Slice:
			v.size = int(unsafe.Sizeof(uintptr(0)))
			skip = int(unsafe.Sizeof(reflect.SliceHeader{})) - v.size
			v.c = classUint
		case reflect.Uintptr, reflect.Ptr, reflect.UnsafePointer, reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Bool:
			v.c = classUint
		case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
			v.c = classInt
		case reflect.Float32, reflect.Float64:
			v.c = classFloat
		default:
			err = fmt.Errorf("type %s of argument number %d not supported", k, i)
			return
		}

		offset = (offset + v.align - 1) &^ (v.align - 1)
		v.offset = offset

		arguments = append(arguments, v)

		offset += skip + v.size
	}

	if f.NumOut() == 1 {
		a := f.Out(0)
		k := a.Kind()

		ret.size = int(a.Size())
		ret.align = int(unsafe.Sizeof(uintptr(0))) // return values are aligned by register size - let's hope this is the same as the pointer size

		switch k {
		case reflect.Slice:
			ret.size = int(unsafe.Sizeof(uintptr(0)))
			ret.c = classUint
		case reflect.Uintptr, reflect.Ptr, reflect.UnsafePointer, reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Bool:
			ret.c = classUint
		case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
			ret.c = classInt
		case reflect.Float32, reflect.Float64:
			ret.c = classFloat
		default:
			err = fmt.Errorf("type %s of return value not supported", k)
			return
		}

		offset = (offset + ret.align - 1) &^ (ret.align - 1)
		ret.offset = offset
	}

	fptr = unsafe.Pointer(v.Pointer())

	return
}
