package engineer_test

import "os"

func writeBytes(path string, b []byte) error {
	return os.WriteFile(path, b, 0o600)
}

func mkdirAll(p string) error { return os.MkdirAll(p, 0o755) }
