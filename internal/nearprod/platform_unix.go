//go:build darwin || linux

package nearprod

import (
	"os"
	"os/exec"
	"syscall"
)

func ownedByUser(st os.FileInfo) bool {
	s, ok := st.Sys().(*syscall.Stat_t)
	return !ok || int(s.Uid) == os.Getuid()
}
func childProcessGroup(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func killGroup(pid int)               { _ = syscall.Kill(-pid, syscall.SIGKILL) }
func detachProcess(cmd *exec.Cmd)     { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
