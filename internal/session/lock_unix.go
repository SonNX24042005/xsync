//go:build !windows

package session

import (
	"os"
	"syscall"
)

func tryLock(f *os.File) error { return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) }
func unlock(f *os.File)        { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
