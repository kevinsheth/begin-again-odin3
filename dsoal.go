package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const dsoalURL = "https://github.com/kcat/dsoal/releases/download/archive/DSOAL_r695.zip"

// installDSOAL puts DSOAL's 32-bit DLLs in a Root Builder mod. Wine's own
// DirectSound mixer uses most of a CPU core on the Odin; DSOAL needs a few
// percent.
func (in install) installDSOAL(zipPath string) (bool, error) {
	root := filepath.Join(in.manager, "mods", dsoalMod, "Root")
	if _, err := os.Stat(filepath.Join(root, "dsound.dll")); err == nil {
		return false, nil
	}

	var data []byte
	var err error
	if zipPath != "" {
		data, err = os.ReadFile(zipPath)
	} else {
		data, err = download(dsoalURL)
	}
	if err != nil {
		return false, fmt.Errorf("getting DSOAL: %w", err)
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
		if err := os.WriteFile(to, contents, 0o644); err != nil {
			return false, err
		}
	}
	return true, nil
}

func download(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// dsoalFiles pulls the Win32 DLLs and license files out of the release, which
// is a zip inside a zip.
func dsoalFiles(data []byte) (map[string][]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("DSOAL download isn't a zip: %w", err)
	}
	files := map[string][]byte{}
	for _, f := range r.File {
		contents, err := readZipFile(f)
		if err != nil {
			return nil, err
		}
		switch {
		case strings.HasSuffix(strings.ToLower(f.Name), ".zip"):
			inner, err := dsoalFiles(contents)
			if err != nil {
				return nil, err
			}
			for k, v := range inner {
				files[k] = v
			}
		case f.Name == "DSOAL/Win32/dsound.dll" || f.Name == "DSOAL/Win32/dsoal-aldrv.dll":
			files[path.Base(f.Name)] = contents
		case strings.HasPrefix(f.Name, "DSOAL/Documentation/") && strings.Contains(f.Name, "License"):
			files[path.Join("DSOAL-licenses", path.Base(f.Name))] = contents
		}
	}
	if files["dsound.dll"] == nil || files["dsoal-aldrv.dll"] == nil {
		return nil, fmt.Errorf("DSOAL download is missing its Win32 DLLs")
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
