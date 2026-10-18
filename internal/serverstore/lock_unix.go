//go:build !windows

package serverstore

import (
	"os"
	"syscall"
)

func lockSettings(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}
func unlockSettings(f *os.File) { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
