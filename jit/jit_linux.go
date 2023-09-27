package jit

import "syscall"

func compile(code []byte) error {
	// FIXME, it may be possible to use Go allocator (ie. make([]byte))
	// and just set the memory to be executable. In order to do this on
	// linux, the memory in question will need to be aligned to a page
	// boundary. This means we can use GC to free the memory when no
	// longer in-use.
	//fmt.Printf("%x\n", code)
	exec, err := syscall.Mmap(
		-1,
		0,
		len(code),
		syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC, syscall.MAP_PRIVATE|syscall.MAP_ANONYMOUS,
	)
	if err != nil {
		return err
	}
	copy(exec, code)
	//fmt.Priantf("%x\n", code)
	return nil
}
