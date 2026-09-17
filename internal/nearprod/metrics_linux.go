//go:build linux

package nearprod

import (
	"os"
	"strconv"
	"strings"
)

func hostMemory() int64 {
	b, e := os.ReadFile("/proc/meminfo")
	if e != nil {
		return 0
	}
	return parseMeminfo(string(b))["MemTotal"]
}
func processRSS() any {
	b, e := os.ReadFile("/proc/self/statm")
	if e != nil {
		return nil
	}
	f := strings.Fields(string(b))
	if len(f) < 2 {
		return nil
	}
	n, e := strconv.ParseInt(f[1], 10, 64)
	if e != nil {
		return nil
	}
	return n * int64(os.Getpagesize())
}
