// +build windows

package fileperm

func IsFileWideOpen(_ string) bool {
	// On Windows file permissions have no meaning. Always return success.
	return false
}
