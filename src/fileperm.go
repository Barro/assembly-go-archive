// +build !windows

package fileperm

import (
	"os"
)

func IsFileWideOpen(filename string) bool {
	stats, err := os.Stat(filename)
	if err != nil {
		return false
	}
	if stats.Mode().Perm()&0044 != 0 {
		return true
	}
	return false
}
