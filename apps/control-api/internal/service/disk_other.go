//go:build !linux

package service

func diskUsage(string) (int, int, int, error) {
	return 0, 0, 0, nil
}
