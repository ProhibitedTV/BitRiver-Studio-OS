//go:build linux

package service

import "syscall"

func diskUsage(path string) (int, int, int, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0, err
	}

	total := gbFromBlocks(stat.Blocks, stat.Bsize)
	free := gbFromBlocks(stat.Bavail, stat.Bsize)
	used := maxInt(total-free, 0)
	return total, used, free, nil
}

func gbFromBlocks(blocks uint64, blockSize int64) int {
	bytes := float64(blocks) * float64(blockSize)
	return int(bytes / (1024 * 1024 * 1024))
}
