package cpu

import (
	"runtime"
	"unsafe"

	"runtime.link/lib/internal/cgo/dyncall"
)

func (p *Program) pinCallArch(reg RegistersArch) RegistersArch {
	var ptrs = make([]unsafe.Pointer, 0, len(p.Pins)) // pins unsafe pointers to prevent them from being garbage collected.
	for _, pin := range p.Pins {
		switch pin {
		case R0:
			ptrs = append(ptrs, reg.R0().UnsafePointer())
		case R1:
			ptrs = append(ptrs, reg.R1().UnsafePointer())
		case R2:
			ptrs = append(ptrs, reg.R2().UnsafePointer())
		case R3:
			ptrs = append(ptrs, reg.R3().UnsafePointer())
		case R4:
			ptrs = append(ptrs, reg.R4().UnsafePointer())
		case R5:
			ptrs = append(ptrs, reg.R5().UnsafePointer())
		case R6:
			ptrs = append(ptrs, reg.R6().UnsafePointer())
		case R7:
			ptrs = append(ptrs, reg.R7().UnsafePointer())
		case R8:
			ptrs = append(ptrs, reg.R8().UnsafePointer())
		case R9:
			ptrs = append(ptrs, reg.R9().UnsafePointer())
		case R10:
			ptrs = append(ptrs, reg.R10().UnsafePointer())
		case R11:
			ptrs = append(ptrs, reg.R11().UnsafePointer())
		case R12:
			ptrs = append(ptrs, reg.R12().UnsafePointer())
		case R13:
			ptrs = append(ptrs, reg.R13().UnsafePointer())
		case R14:
			ptrs = append(ptrs, reg.R14().UnsafePointer())
		case R15:
			ptrs = append(ptrs, reg.R15().UnsafePointer())
		}
	}
	defer runtime.KeepAlive(ptrs)
	return p.callArch(reg)
}

func (p *Program) callArch(reg RegistersArch) RegistersArch {
	//println(error(&err))
	//fmt.Println(p.Text, reg.r0.Int32())
	//p.Dump()
	var (
		pc int                   // program counter
		g  FloatingPointRegister // used to backup runtime.g on architectures that require it.

		normal Register
		length Register
		assert Register
		result Register

		out = reg // write-only output registers

		heap [][]byte       // heap allocator
		pins runtime.Pinner // pins unsafe pointers to prevent them from being garbage collected.

		vm *dyncall.VM
	)
	if p.Size == 0 {
		switch runtime.GOARCH {
		case "amd64":
			GrowStack() // assert stack has at-least 1MB of space.
		case "arm64":
			prepare := Prepare
			g.SetUintptr((*(*func() uintptr)(unsafe.Pointer(&prepare)))()) // save runtime.g, and grow stack, as above.
		}
	} else {
		vm = dyncall.NewVM(p.Size)
	}
	// intepreter loop, executes each instruction one-by-one.
	// the long register switch cases hopefully mean the Go
	// compiler can optimize this into a jump table. ?
	for ; pc < len(p.Text); pc++ {
		mode, data := p.Text[pc].Decode()
		switch mode {
		case Bits:
			normal.SetUint8(uint8(data))
		case Slow:
			switch SlowFunc(data) {
			case PushBytes1:
				vm.PushChar(normal.Int8())
			case PushBytes2:
				vm.PushShort(normal.Int16())
			case PushBytes4:
				vm.PushSignedInt(normal.Int32())
			case PushBytes8:
				vm.PushSignedLong(normal.Int())
			case PushFloat4:
				vm.PushFloat(normal.Float32())
			case PushFloat8:
				vm.PushDouble(normal.Float64())
			case PushMemory:
				vm.PushPointer(normal.UnsafePointer())
			case PushSizing:
				vm.PushSignedLong(normal.Int())
			case CallBytes1:
				normal.SetInt8(vm.CallChar(p.Call))
			case CallBytes2:
				normal.SetInt16(vm.CallShort(p.Call))
			case CallBytes4:
				normal.SetInt32(vm.CallInt(p.Call))
			case CallBytes8:
				normal.SetInt(vm.CallLong(p.Call))
			case CallFloat4:
				normal.SetFloat32(vm.CallFloat(p.Call))
			case CallFloat8:
				normal.SetFloat64(vm.CallDouble(p.Call))
			case CallMemory:
				normal.SetUnsafePointer(vm.CallPointer(p.Call))
			case CallSizing:
				normal.SetInt(vm.CallLong(p.Call))
			case CallIgnore:
				vm.Call(p.Call)
			default:
				panic("not implemented")
			}
		case Math:
			switch MathFunc(data) {
			case Flip:
				result.SetBool(!result.Bool())
			case Less:
				result.SetBool(normal.Uint() < assert.Uint())
			case More:
				result.SetBool(normal.Uint() > assert.Uint())
			case Same:
				result.SetBool(normal.Uint() == assert.Uint())
			case Add:
				result.SetUint(normal.Uint() + length.Uint())
			case Sub:
				result.SetUint(normal.Uint() - length.Uint())
			case Mul:
				result.SetUint(normal.Uint() * length.Uint())
			case Div:
				result.SetUint(normal.Uint() / length.Uint())
			case Mod:
				result.SetUint(normal.Uint() % length.Uint())
			case Addi:
				result.SetInt(normal.Int() + length.Int())
			case Subi:
				result.SetInt(normal.Int() - length.Int())
			case Muli:
				result.SetInt(normal.Int() * length.Int())
			case Divi:
				result.SetInt(normal.Int() / length.Int())
			case Modi:
				result.SetInt(normal.Int() % length.Int())
			case And:
				result.SetUint(normal.Uint() & length.Uint())
			case Or:
				result.SetUint(normal.Uint() | length.Uint())
			case Xor:
				result.SetUint(normal.Uint() ^ length.Uint())
			case Shl:
				result.SetUint(normal.Uint() << length.Uint())
			case Shr:
				result.SetUint(normal.Uint() >> length.Uint())
			case Addf:
				result.SetFloat64(normal.Float64() + length.Float64())
			case Subf:
				result.SetFloat64(normal.Float64() - length.Float64())
			case Mulf:
				result.SetFloat64(normal.Float64() * length.Float64())
			case Divf:
				result.SetFloat64(normal.Float64() / length.Float64())
			case Conv:
				result.SetFloat64(float64(normal.Int()))
			case Convf:
				result.SetInt(int(normal.Float64()))
			default:
				panic("not implemented")
			}
		case Func:
			switch FuncName(data) {
			case Noop:
			case Jump:
				if assert.Uint64() != 0 {
					pc = int(normal.Uint())
				}
			case Call:
				switch runtime.GOARCH {
				case "amd64":
					callfunc := CallFunc
					out.r0.SetUnsafePointer(p.Call)
					reg = (*(*func(RegistersArch) RegistersArch)(unsafe.Pointer(&callfunc)))(out)
				case "arm64":
					closure := &p.Call
					restore := Restore
					reg = (*(*func(RegistersArch) RegistersArch)(unsafe.Pointer(&closure)))(out)
					(*(*func(g uintptr))(unsafe.Pointer(&restore)))(g.Uintptr())
				}
				out = reg
			case SwapLength:
				length, normal = normal, length
			case SwapAssert:
				assert, normal = normal, assert
			case SwapResult:
				result, normal = normal, result
			case HeapMake:
				heap = append(heap, nil)
			case HeapPush8:
				heap[len(heap)-1] = append(heap[len(heap)-1], normal.Uint8())
			case HeapPush16:
				i := normal.Uint16()
				heap[len(heap)-1] = append(heap[len(heap)-1], byte(i), byte(i>>8))
			case HeapPush32:
				i := normal.Uint32()
				heap[len(heap)-1] = append(heap[len(heap)-1], byte(i), byte(i>>8), byte(i>>16), byte(i>>24))
			case HeapPush64:
				i := normal.Uint64()
				heap[len(heap)-1] = append(heap[len(heap)-1], byte(i), byte(i>>8), byte(i>>16), byte(i>>24), byte(i>>32), byte(i>>40), byte(i>>48), byte(i>>56))
			case HeapCopy:
				data := unsafe.Slice((*byte)(normal.UnsafePointer()), length.Uintptr())
				heap[len(heap)-1] = append(heap[len(heap)-1], data...)
			case HeapLoad:
				ptr := unsafe.Pointer(unsafe.SliceData(heap[len(heap)-1]))
				pins.Pin(ptr)
				normal.SetUnsafePointer(ptr)
				length.SetInt(len(heap[len(heap)-1]))
				heap = heap[:len(heap)-1]
			case StackCaller:
				panic("not implemented")
			case StackCallee:
				panic("not implemented")
			case Stack:
				panic("not implemented")
			case Stack8:
				panic("not implemented")
			case Stack16:
				panic("not implemented")
			case Stack32:
				panic("not implemented")
			case Stack64:
				panic("not implemented")
			case PointerMake:
				ptr := new(Pointer)
				ptr.addr = normal.Uintptr()
				ptr.free = p.pin(ptr)
				normal.SetUnsafePointer(unsafe.Pointer(ptr))
			case PointerFree:
				ptr := (*Pointer)(normal.UnsafePointer())
				ptr.free()
			case PointerKeep:
				pins.Pin(normal.UnsafePointer())
			case PointerLoad:
				ptr := (*Pointer)(normal.UnsafePointer())
				normal.SetUintptr(ptr.addr)
			case StringSize:
				for i := 0; ; i++ {
					if *(*byte)(unsafe.Add(normal.UnsafePointer(), uintptr(i))) == 0 {
						length.SetInt(i)
						break
					}
				}
			case StringCopy: // null terminated string copy (unknown length)
				var buf []byte
				for i := 0; ; i++ {
					b := *(*byte)(unsafe.Add(normal.UnsafePointer(), uintptr(i)))
					if b == 0 {
						break
					}
					buf = append(buf, b)
				}
				normal.SetUnsafePointer(unsafe.Pointer(unsafe.SliceData(buf)))
			case StringMake:
				ptr := normal.UnsafePointer()
				siz := length.Uintptr()
				s := unsafe.Slice((*byte)(ptr), siz)
				if len(s) > 0 && s[len(s)-1] != 0 {
					s = append(s, 0)
				}
				pins.Pin(unsafe.SliceData(s))
				normal.SetUnsafePointer(unsafe.Pointer(unsafe.SliceData(s)))
			case ErrorMake:
				if result == 0 {
					var err error = Error(normal)
					var ptr = *(*unsafe.Pointer)(unsafe.Pointer(&err))
					normal.SetUnsafePointer(ptr)
					length.SetUnsafePointer(unsafe.Add(ptr, unsafe.Sizeof(uintptr(0))))
					pins.Pin(&err)
				} else {
					length.SetUintptr(0)
					normal.SetUintptr(0)
				}
			case Wrap:
				normal = p.Func[length.Uint()](normal)
			case Bool:
				normal.SetBool(normal.Uint() != 0)
			case AssertArgs:
				panic("not implemented")
			default:
				panic("not implemented")
			}
		case Load:
			switch Location(data) {
			case R0:
				normal = *reg.R0()
			case R1:
				normal = *reg.R1()
			case R2:
				normal = *reg.R2()
			case R3:
				normal = *reg.R3()
			case R4:
				normal = *reg.R4()
			case R5:
				normal = *reg.R5()
			case R6:
				normal = *reg.R6()
			case R7:
				normal = *reg.R7()
			case R8:
				normal = *reg.R8()
			case R9:
				normal = *reg.R9()
			case R10:
				normal = *reg.R10()
			case R11:
				normal = *reg.R11()
			case R12:
				normal = *reg.R12()
			case R13:
				normal = *reg.R13()
			case R14:
				normal = *reg.R14()
			case R15:
				normal = *reg.R15()
			case X0:
				normal.SetUint(reg.X0().Uint())
			case X1:
				normal.SetUint(reg.X1().Uint())
			case X2:
				normal.SetUint(reg.X2().Uint())
			case X3:
				normal.SetUint(reg.X3().Uint())
			case X4:
				normal.SetUint(reg.X4().Uint())
			case X5:
				normal.SetUint(reg.X5().Uint())
			case X6:
				normal.SetUint(reg.X6().Uint())
			case X7:
				normal.SetUint(reg.X7().Uint())
			case X8:
				normal.SetUint(reg.X8().Uint())
			case X9:
				normal.SetUint(reg.X9().Uint())
			case X10:
				normal.SetUint(reg.X10().Uint())
			case X11:
				normal.SetUint(reg.X11().Uint())
			case X12:
				normal.SetUint(reg.X12().Uint())
			case X13:
				normal.SetUint(reg.X13().Uint())
			case X14:
				normal.SetUint(reg.X14().Uint())
			case X15:
				normal.SetUint(reg.X15().Uint())
			}
		case Move:
			switch Location(data) {
			case R0:
				*out.R0() = normal
			case R1:
				*out.R1() = normal
			case R2:
				*out.R2() = normal
			case R3:
				*out.R3() = normal
			case R4:
				*out.R4() = normal
			case R5:
				*out.R5() = normal
			case R6:
				*out.R6() = normal
			case R7:
				*out.R7() = normal
			case R8:
				*out.R8() = normal
			case R9:
				*out.R9() = normal
			case R10:
				*out.R10() = normal
			case R11:
				*out.R11() = normal
			case R12:
				*out.R12() = normal
			case R13:
				*out.R13() = normal
			case R14:
				*out.R14() = normal
			case R15:
				*out.R15() = normal
			case X0:
				out.X0().SetUint(normal.Uint())
			case X1:
				out.X1().SetUint(normal.Uint())
			case X2:
				out.X2().SetUint(normal.Uint())
			case X3:
				out.X3().SetUint(normal.Uint())
			case X4:
				out.X4().SetUint(normal.Uint())
			case X5:
				out.X5().SetUint(normal.Uint())
			case X6:
				out.X6().SetUint(normal.Uint())
			case X7:
				out.X7().SetUint(normal.Uint())
			case X8:
				out.X8().SetUint(normal.Uint())
			case X9:
				out.X9().SetUint(normal.Uint())
			case X10:
				out.X10().SetUint(normal.Uint())
			case X11:
				out.X11().SetUint(normal.Uint())
			case X12:
				out.X12().SetUint(normal.Uint())
			case X13:
				out.X13().SetUint(normal.Uint())
			case X14:
				out.X14().SetUint(normal.Uint())
			case X15:
				out.X15().SetUint(normal.Uint())
			}
		}
	}
	if pins != (runtime.Pinner{}) {
		pins.Unpin()
	}
	return out
}
