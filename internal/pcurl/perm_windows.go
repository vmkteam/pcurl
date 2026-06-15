//go:build windows

package pcurl

// checkPermissions is a no-op on Windows.
//
// Go derives a file's permission bits on Windows solely from the read-only
// attribute: os.Stat().Mode().Perm() returns 0666 for writable files and 0444
// for read-only ones — Unix-style group/other bits and ACLs are never exposed.
// That makes the Unix check (perm&0077 == 0, i.e. mode 0600) impossible to
// satisfy:
//
//	0666 & 0077 = 0066  -> always fails
//	0444 & 0077 = 0044  -> always fails
//
// chmod (including via Git Bash) cannot fix this, so the check would render
// pcurl unusable on Windows. Access is instead restricted by the ACLs of the
// per-user config directory (%USERPROFILE%\.config\pcurl). See perm_unix.go for
// the real check.
func checkPermissions(_ string) error {
	return nil
}
