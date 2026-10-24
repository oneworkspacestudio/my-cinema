package app

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const RuntimeSHA256 = "64b81a7b01b4dfb4ad3380a367cdeaf59878d9070c0d5e0cf84025e58ed3e853"

// Extract only the pinned vendor archive, into private app-relative state.
// The publisher appends the exact vendor ZIP and a fixed-size footer to the PE.
// Windows permits a PE data overlay; the normal updater hashes the whole file.
// This keeps the installed launch contract to one atomic executable package.
const runtimeMagic = "MYCINEMA-RUNTIME"

func prepareRuntime(data, archive string) (string, error) {
	f, e := os.Open(archive)
	if e != nil {
		return "", errors.New("the My Cinema runtime payload is missing; restore the complete verified executable")
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return "", e
	}
	offset, size := int64(0), st.Size()
	if strings.HasSuffix(strings.ToLower(archive), ".exe") {
		const footerSize = 24
		if size < footerSize {
			return "", errors.New("the My Cinema executable has no bundled media runtime")
		}
		footer := make([]byte, footerSize)
		if _, e = f.ReadAt(footer, size-footerSize); e != nil {
			return "", e
		}
		if string(footer[:16]) != runtimeMagic {
			return "", errors.New("the My Cinema executable has no verified media payload; use the packaged release")
		}
		length := binary.LittleEndian.Uint64(footer[16:])
		if length > uint64(size-footerSize) || length > 512<<20 {
			return "", errors.New("invalid bundled media payload size")
		}
		offset = size - footerSize - int64(length)
		size = int64(length)
	}
	content := io.NewSectionReader(f, offset, size)
	sum := sha256.New()
	_, e = io.Copy(sum, content)
	if e != nil || hex.EncodeToString(sum.Sum(nil)) != RuntimeSHA256 {
		return "", errors.New("the media runtime archive failed its release checksum")
	}
	root := filepath.Join(data, "runtime", "jellyfin")
	marker := filepath.Join(root, ".mycinema-runtime")
	if b, e := os.ReadFile(marker); e == nil && string(b) == RuntimeSHA256 {
		for _, name := range []string{"jellyfin.exe", "ffmpeg.exe", "ffprobe.exe"} {
			if rejectLink(filepath.Join(root, name)) {
				return "", errors.New("the media runtime cannot contain links")
			}
			if _, e = os.Stat(filepath.Join(root, name)); e != nil {
				return "", errors.New("the extracted media runtime is incomplete")
			}
		}
		return root, nil
	}
	if _, e = os.Stat(root); e == nil {
		return "", errors.New("an incomplete media runtime exists; preserve it and extract into a fresh app data folder")
	}
	z, e := zip.NewReader(io.NewSectionReader(f, offset, size), size)
	if e != nil {
		return "", e
	}
	parent := filepath.Dir(root)
	if e = os.MkdirAll(parent, 0700); e != nil {
		return "", e
	}
	stage, e := os.MkdirTemp(parent, ".jellyfin-extract-")
	if e != nil {
		return "", e
	}
	defer os.RemoveAll(stage)
	var total uint64
	for _, entry := range z.File {
		name := strings.TrimPrefix(entry.Name, "jellyfin/")
		if name == "" {
			continue
		}
		if name == entry.Name || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") || entry.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("unexpected media runtime archive entry")
		}
		path := filepath.Join(stage, filepath.FromSlash(name))
		if !inside(stage, path) {
			return "", errors.New("media runtime entry escapes its folder")
		}
		total += entry.UncompressedSize64
		if total > 2<<30 {
			return "", errors.New("media runtime exceeds its extraction limit")
		}
		if entry.FileInfo().IsDir() {
			if e = os.MkdirAll(path, 0700); e != nil {
				return "", e
			}
			continue
		}
		if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			return "", e
		}
		src, e := entry.Open()
		if e != nil {
			return "", e
		}
		dst, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			src.Close()
			return "", e
		}
		_, e = io.Copy(dst, io.LimitReader(src, 2<<30))
		closeErr := dst.Close()
		src.Close()
		if e != nil {
			return "", e
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	if e = os.WriteFile(filepath.Join(stage, ".mycinema-runtime"), []byte(RuntimeSHA256), 0600); e != nil {
		return "", e
	}
	if e = os.Rename(stage, root); e != nil {
		return "", e
	}
	return root, nil
}
