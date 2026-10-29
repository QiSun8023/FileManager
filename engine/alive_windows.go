//go:build windows

package main

import "syscall"

func alive(pid int) bool {
	h, err := syscall.OpenProcess(0x1000, false, uint32(pid))
	if err != nil {
		return false
	}
	syscall.CloseHandle(h)
	return true
}
