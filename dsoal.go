package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// dsoalURL is a variable so tests can serve their own archive.
var dsoalURL = "https://github.com/kcat/dsoal/releases/download/archive/DSOAL_r695.zip"

// installDSOAL puts DSOAL's 32-bit DLLs and licenses in a Root Builder mod.
// On the Odin, Wine's own DirectSound mixer takes most of a CPU core.
func (in install) installDSOAL() (bool, error) {
	root := filepath.Join(in.manager, "mods", dsoalMod, "Root")
	if _, err := os.Stat(filepath.Join(root, "dsound.dll")); err == nil {
		return false, nil
	}
	data, err := download(dsoalURL)
	if err != nil {
		return false, err
	}
	files, err := dsoalFiles(data)
	if err != nil {
		return false, err
	}
	for name, contents := range files {
		to := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return false, err
		}
		if err := writeFile(to, contents); err != nil {
			return false, err
		}
	}
	return true, nil
}

func download(url string) ([]byte, error) {
	client := http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// dsoalFiles extracts the Win32 DLLs and license files from the release
// archive, which is a zip inside a zip.
func dsoalFiles(data []byte) (map[string][]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("reading DSOAL archive: %w", err)
	}
	files := map[string][]byte{}
	for _, f := range r.File {
		name := f.Name
		isZip := strings.HasSuffix(strings.ToLower(name), ".zip")
		isDLL := name == "DSOAL/Win32/dsound.dll" || name == "DSOAL/Win32/dsoal-aldrv.dll"
		isLicense := strings.HasPrefix(name, "DSOAL/Documentation/") && strings.Contains(name, "License")
		if !isZip && !isDLL && !isLicense {
			continue
		}
		contents, err := readZipFile(f)
		if err != nil {
			return nil, err
		}
		switch {
		case isZip:
			inner, err := dsoalFiles(contents)
			if err != nil {
				return nil, err
			}
			for k, v := range inner {
				files[k] = v
			}
		case isDLL:
			files[path.Base(name)] = contents
		case isLicense:
			files[path.Join("DSOAL-licenses", path.Base(name))] = contents
		}
	}
	if files["dsound.dll"] == nil || files["dsoal-aldrv.dll"] == nil {
		return nil, errors.New("DSOAL archive is missing its Win32 DLLs")
	}
	return files, nil
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
