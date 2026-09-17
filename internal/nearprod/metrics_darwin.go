//go:build darwin

package nearprod

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func hostMemory() int64 {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	b, e := exec.CommandContext(ctx, "/usr/sbin/sysctl", "-n", "hw.memsize").Output()
	if e != nil {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	return n
}
func processRSS() any {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	b, e := exec.CommandContext(ctx, "/bin/ps", "-o", "rss=", "-p", strconv.Itoa(os.Getpid())).Output()
	if e != nil {
		return nil
	}
	n, e := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if e != nil {
		return nil
	}
	return n * 1024
}
