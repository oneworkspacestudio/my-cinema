package app

import "os"

// Windows refuses to replace an existing directory (including an empty one).
// The entire title becomes visible in one rename; no playable partial file exists.
func publishUpload(root *os.Root, stage, title string) error {
	return root.Rename(stage, title)
}
