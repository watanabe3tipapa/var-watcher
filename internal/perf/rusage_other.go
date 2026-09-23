//go:build !darwin

package perf

import "syscall"

// secVal は Rusage のユーザ+システム CPU 秒を返す(linux 等は Timeval)。
func secVal(ru *syscall.Rusage) float64 {
	return float64(ru.Utime.Sec) + float64(ru.Utime.Usec)/1e6 +
		float64(ru.Stime.Sec) + float64(ru.Stime.Usec)/1e6
}
