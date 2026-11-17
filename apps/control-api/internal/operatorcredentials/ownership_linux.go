//go:build linux

package operatorcredentials

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func preserveOwnership(temp *os.File, original os.FileInfo) error {
	stat, ok := original.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("cannot determine credentials file ownership")
	}
	uid, gid := int(stat.Uid), int(stat.Gid)
	if uid != os.Geteuid() || gid != os.Getegid() {
		if err := temp.Chown(uid, gid); err != nil {
			return fmt.Errorf("cannot preserve credentials ownership: %w", err)
		}
	}
	return nil
}
