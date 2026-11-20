package fsio

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input")
	if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile(path, 4)
	if string(got) != "data" || err != nil {
		t.Fatalf("ReadFile = %q, %v", got, err)
	}
	if _, err := ReadFile(path, 3); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("size check failed: %v", err)
	}
	if _, err := ReadFile(dir, 1<<20); !errors.Is(err, ErrNotRegular) {
		t.Fatalf("directory check failed: %v", err)
	}
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(fifo, 1<<20); !errors.Is(err, ErrNotRegular) {
		t.Fatalf("FIFO check failed: %v", err)
	}
}

func TestWriteAtomic(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	if err := WriteAtomic(out, []byte("old"), false); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Fatalf("mode = %o", fi.Mode().Perm())
	}
	if err := WriteAtomic(out, []byte("new"), false); !errors.Is(err, ErrOutputExists) {
		t.Fatalf("wanted output collision, got %v", err)
	}
	if got, err := os.ReadFile(out); string(got) != "old" || err != nil {
		t.Fatalf("original = %q, %v", got, err)
	}
	if err := WriteAtomic(out, []byte("new"), true); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(out); string(got) != "new" || err != nil {
		t.Fatalf("replacement = %q, %v", got, err)
	}
	destination := filepath.Join(dir, "destination")
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(destination, []byte("new"), true); err == nil {
		t.Fatal("replaced directory with regular file")
	}
	if names, err := filepath.Glob(filepath.Join(dir, ".genc-*")); err != nil || len(names) != 0 {
		t.Fatalf("temporary files = %v, %v", names, err)
	}
}

func TestCheckPaths(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "in")
	if err := os.WriteFile(source, nil, 0600); err != nil {
		t.Fatal(err)
	}
	hard := filepath.Join(dir, "hard")
	symlink := filepath.Join(dir, "symlink")
	dangling := filepath.Join(dir, "dangling")
	if err := os.Link(source, hard); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, symlink); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "missing"), dangling); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{source, hard, symlink} {
		if err := CheckPaths(source, path, true); !errors.Is(err, ErrSameFile) {
			t.Errorf("path %q, expected same-file error, got %v", path, err)
		}
	}
	if err := CheckPaths(source, dangling, false); !errors.Is(err, ErrOutputExists) {
		t.Errorf("expected dangling symlink to occupy output path, got %v", err)
	}
}
