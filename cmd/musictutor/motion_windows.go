package main

import (
	"syscall"
	"unsafe"
)

// systemReducesMotion reports Windows' "Show animations in Windows" being
// off (SPI_GETCLIENTAREAANIMATION), the default for Reduce motion.
func systemReducesMotion() bool {
	const spiGetClientAreaAnimation = 0x1042
	proc := syscall.NewLazyDLL("user32.dll").NewProc("SystemParametersInfoW")
	if proc.Find() != nil {
		return false
	}
	var on int32
	r, _, _ := proc.Call(spiGetClientAreaAnimation, 0, uintptr(unsafe.Pointer(&on)), 0)
	return r != 0 && on == 0
}
