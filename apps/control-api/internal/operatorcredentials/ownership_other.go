//go:build !linux

package operatorcredentials

import (
	"errors"
	"os"
)

// The appliance is Linux-only; keep development builds portable while
// refusing to rotate files on hosts whose ownership semantics differ.
func preserveOwnership(_ *os.File, _ os.FileInfo) error {
	return errors.New("operator key rotation is only supported on the Linux appliance")
}
