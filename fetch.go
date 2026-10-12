package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

// source is a folder to copy from: a local path, or host:path over SSH.
type source struct {
	host, path string
}

func parseSource(s string) source {
	if i := strings.Index(s, ":"); i > 0 && !strings.Contains(s[:i], "/") {
		return source{host: s[:i], path: s[i+1:]}
	}
	return source{path: s}
}

func (s source) String() string {
	if s.host == "" {
		return s.path
	}
	return s.host + ":" + s.path
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func (s source) ssh(command string) *exec.Cmd {
	return exec.Command("ssh", "-o", "BatchMode=yes", s.host, command)
}

// list returns every file's size, keyed by its path relative to the source.
// Top-level folders in skip are left out.
func (s source) list(skip []string) (map[string]int64, error) {
	files := map[string]int64{}
	keep := func(rel string) bool {
		top, _, _ := strings.Cut(rel, "/")
		for _, k := range skip {
			if strings.EqualFold(top, k) {
				return false
			}
		}
		return true
	}
	if s.host == "" {
		err := filepath.WalkDir(s.path, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.Type().IsRegular() {
				return err
			}
			rel, _ := filepath.Rel(s.path, path)
			if rel = filepath.ToSlash(rel); keep(rel) {
				info, err := d.Info()
				if err != nil {
					return err
				}
				files[rel] = info.Size()
			}
			return nil
		})
		return files, err
	}
	out, err := s.ssh("find " + shellQuote(s.path) + ` -type f -printf '%s %P\0'`).Output()
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", s, err)
	}
	for _, entry := range bytes.Split(out, []byte{0}) {
		size, rel, ok := strings.Cut(string(entry), " ")
		if !ok {
			continue
		}
		var n int64
		fmt.Sscan(size, &n)
		if keep(rel) {
			files[rel] = n
		}
	}
	return files, nil
}

// fetch copies src into dest, skipping files that are already there with the
// right size, so an interrupted copy picks up where it stopped.
func fetch(src source, dest string, skip []string, verify bool) error {
	want, err := src.list(skip)
	if err != nil {
		return err
	}
	if len(want) == 0 {
		return fmt.Errorf("%s is empty or missing", src)
	}
	have, _ := source{path: dest}.list(nil)

	var need []string
	var total int64
	for rel, size := range want {
		if got, ok := have[rel]; !ok || got != size {
			need = append(need, rel)
			total += size
		}
	}
	sort.Strings(need)
	if len(need) > 0 {
		fmt.Printf("Copying %d files (%s) from %s\n", len(need), gigabytes(total), src)
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return err
		}
		if err := copyFiles(src, dest, need, total); err != nil {
			return err
		}
	}

	have, err = source{path: dest}.list(nil)
	if err != nil {
		return err
	}
	for rel, size := range want {
		if got, ok := have[rel]; !ok || got != size {
			return fmt.Errorf("%s didn't copy completely. Run setup again to resume", rel)
		}
	}
	if verify {
		return compareHashes(src, dest, want)
	}
	return nil
}

func copyFiles(src source, dest string, files []string, total int64) error {
	var done atomic.Int64
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		tick := time.NewTicker(10 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				fmt.Printf("  %s of %s\n", gigabytes(done.Load()), gigabytes(total))
			}
		}
	}()

	if src.host == "" {
		for _, rel := range files {
			if err := copyFile(filepath.Join(src.path, rel), filepath.Join(dest, rel)); err != nil {
				return err
			}
			info, _ := os.Stat(filepath.Join(dest, rel))
			if info != nil {
				done.Add(info.Size())
			}
		}
		return nil
	}

	var list bytes.Buffer
	for _, rel := range files {
		list.WriteString(rel)
		list.WriteByte(0)
	}
	send := src.ssh("tar -C " + shellQuote(src.path) + " --null -T - -cf -")
	send.Stdin = &list
	send.Stderr = os.Stderr
	stream, err := send.StdoutPipe()
	if err != nil {
		return err
	}
	receive := exec.Command("tar", "-C", dest, "-xf", "-")
	receive.Stdin = &countingReader{r: stream, n: &done}
	receive.Stderr = os.Stderr
	if err := send.Start(); err != nil {
		return err
	}
	if err := receive.Run(); err != nil {
		send.Process.Kill()
		send.Wait()
		return fmt.Errorf("copy from %s stopped: %w. Run setup again to resume", src, err)
	}
	if err := send.Wait(); err != nil {
		return fmt.Errorf("copy from %s stopped: %w. Run setup again to resume", src, err)
	}
	return nil
}

type countingReader struct {
	r io.Reader
	n *atomic.Int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n.Add(int64(n))
	return n, err
}

// compareHashes checks every copied file's SHA-256 against the source.
func compareHashes(src source, dest string, files map[string]int64) error {
	fmt.Printf("Checking %d files against %s. This takes a while\n", len(files), src)
	want := map[string]string{}
	if src.host == "" {
		for rel := range files {
			sum, err := hashFile(filepath.Join(src.path, rel))
			if err != nil {
				return err
			}
			want[rel] = sum
		}
	} else {
		var list bytes.Buffer
		for rel := range files {
			list.WriteString(rel)
			list.WriteByte(0)
		}
		cmd := src.ssh("cd " + shellQuote(src.path) + " && xargs -0 sha256sum --")
		cmd.Stdin = &list
		out, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("hashing %s: %w", src, err)
		}
		scan := bufio.NewScanner(bytes.NewReader(out))
		scan.Buffer(nil, 1<<20)
		for scan.Scan() {
			sum, rel, ok := strings.Cut(scan.Text(), "  ")
			if ok {
				want[rel] = sum
			}
		}
	}
	for rel := range files {
		got, err := hashFile(filepath.Join(dest, rel))
		if err != nil {
			return err
		}
		if got != want[rel] {
			return fmt.Errorf("%s doesn't match the source. Delete it and run setup again", rel)
		}
	}
	fmt.Println("Every file matches")
	return nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// fetchOnce copies src into dest unless an earlier run already finished
// copying from the same place. Setup edits the copy afterward, so copying
// again would undo that.
func fetchOnce(from, dest string, skip []string, verify bool) error {
	if from == "" {
		return nil
	}
	src := parseSource(from)
	marker := filepath.Join(dest, ".begin-again-odin3-copied")
	if data, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(data)) == src.String() {
		fmt.Printf("Already copied %s\n", src)
		return nil
	}
	if err := fetch(src, dest, skip, verify); err != nil {
		return err
	}
	return os.WriteFile(marker, []byte(src.String()+"\n"), 0o644)
}

func gigabytes(n int64) string { return fmt.Sprintf("%.1f GB", float64(n)/1e9) }
