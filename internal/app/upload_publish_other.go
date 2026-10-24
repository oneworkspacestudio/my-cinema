//go:build !windows

package app

import (
	"errors"
	"os"
)

// This distribution targets Windows. Avoid weaker directory rename semantics on
// other platforms until their no-replace publish operation has been certified.
func publishUpload(root *os.Root, stage, title string) error {
	return errors.New("video uploads require the Windows distribution")
}
