package filesystem

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	srcPath := filepath.Join(tmpDir, "src.txt")
	content := []byte("hello world")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	dstPath := filepath.Join(tmpDir, "dst.txt")

	if err := CopyFile(srcPath, dstPath); err != nil {
		t.Fatalf("CopyFile: %v", err)
	}

	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("read dest file: %v", err)
	}
	if !bytes.Equal(content, got) {
		t.Fatalf("content mismatch: got %q, want %q", got, content)
	}
}

func TestCopyDir(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.Mkdir(srcDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(srcDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "subdir", "subfile.txt"), []byte("subcontent"), 0644); err != nil {
		t.Fatal(err)
	}

	dstDir := filepath.Join(tmpDir, "dst")

	if err := CopyDir(srcDir, dstDir); err != nil {
		t.Fatalf("CopyDir: %v", err)
	}

	for _, rel := range []string{"file.txt", filepath.Join("subdir", "subfile.txt")} {
		p := filepath.Join(dstDir, rel)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file %s to exist: %v", rel, err)
		}
	}
}

func TestCreateSymlinkOrCopy(t *testing.T) {
	t.Run("symlink to directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		srcDir := filepath.Join(tmpDir, "srcdir")
		if err := os.Mkdir(srcDir, 0755); err != nil {
			t.Fatal(err)
		}

		linkPath := filepath.Join(tmpDir, "link")
		if err := CreateSymlinkOrCopy(srcDir, linkPath); err != nil {
			t.Fatalf("CreateSymlinkOrCopy: %v", err)
		}

		fi, err := os.Lstat(linkPath)
		if err != nil {
			t.Fatalf("Lstat: %v", err)
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			t.Fatal("expected link to be a symlink")
		}
	})

	t.Run("symlink to file", func(t *testing.T) {
		tmpDir := t.TempDir()
		srcFile := filepath.Join(tmpDir, "src.txt")
		if err := os.WriteFile(srcFile, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}

		linkPath := filepath.Join(tmpDir, "link.txt")
		if err := CreateSymlinkOrCopy(srcFile, linkPath); err != nil {
			t.Fatalf("CreateSymlinkOrCopy: %v", err)
		}

		fi, err := os.Lstat(linkPath)
		if err != nil {
			t.Fatalf("Lstat: %v", err)
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			t.Fatal("expected link to be a symlink")
		}
	})
}

func TestIsSymlink(t *testing.T) {
	t.Run("regular file", func(t *testing.T) {
		tmpDir := t.TempDir()
		p := filepath.Join(tmpDir, "regular.txt")
		if err := os.WriteFile(p, []byte("hi"), 0644); err != nil {
			t.Fatal(err)
		}
		if IsSymlink(p) {
			t.Fatal("expected IsSymlink to return false for regular file")
		}
	})

	t.Run("symlink", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "target.txt")
		if err := os.WriteFile(target, []byte("hi"), 0644); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(tmpDir, "link.txt")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		if !IsSymlink(link) {
			t.Fatal("expected IsSymlink to return true for symlink")
		}
	})

	t.Run("nonexistent path", func(t *testing.T) {
		if IsSymlink("/nonexistent/path/that/does/not/exist") {
			t.Fatal("expected IsSymlink to return false for nonexistent path")
		}
	})
}

func TestCopyFile_SymlinkSource(t *testing.T) {
	tmpDir := t.TempDir()

	realFile := filepath.Join(tmpDir, "real.txt")
	if err := os.WriteFile(realFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	symlinkSrc := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(realFile, symlinkSrc); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(tmpDir, "dst.txt")
	err := CopyFile(symlinkSrc, dst)
	if err == nil {
		t.Fatal("expected error when source is a symlink")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected error about symlink, got: %v", err)
	}
}

func TestCopyDir_SkipsSymlinks(t *testing.T) {
	tmpDir := t.TempDir()

	srcDir := filepath.Join(tmpDir, "src")
	if err := os.Mkdir(srcDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Normal file
	if err := os.WriteFile(filepath.Join(srcDir, "normal.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	// Symlink inside srcDir pointing to the normal file
	if err := os.Symlink(filepath.Join(srcDir, "normal.txt"), filepath.Join(srcDir, "link.txt")); err != nil {
		t.Fatal(err)
	}

	dstDir := filepath.Join(tmpDir, "dst")
	if err := CopyDir(srcDir, dstDir); err != nil {
		t.Fatalf("CopyDir: %v", err)
	}

	// Normal file should be copied
	if _, err := os.Stat(filepath.Join(dstDir, "normal.txt")); err != nil {
		t.Fatal("expected normal.txt to be copied")
	}

	// Symlink should be skipped
	if _, err := os.Lstat(filepath.Join(dstDir, "link.txt")); err == nil {
		t.Fatal("expected symlink link.txt to be skipped, but it exists in destination")
	}
}

func TestCopyDir_DepthLimit(t *testing.T) {
	tmpDir := t.TempDir()

	// maxCopyDepth is 20; copyDirRecursive checks depth > maxCopyDepth,
	// so we need 21 levels of nesting to trigger the error.
	srcDir := filepath.Join(tmpDir, "src")
	current := srcDir
	for i := 0; i <= maxCopyDepth; i++ {
		current = filepath.Join(current, fmt.Sprintf("d%d", i))
	}
	if err := os.MkdirAll(current, 0755); err != nil {
		t.Fatal(err)
	}

	dstDir := filepath.Join(tmpDir, "dst")
	err := CopyDir(srcDir, dstDir)
	if err == nil {
		t.Fatal("expected recursion limit error")
	}
	if !strings.Contains(err.Error(), "recursion limit") {
		t.Fatalf("expected recursion limit error, got: %v", err)
	}
}

func TestCopyDir_SymlinkSourceRoot(t *testing.T) {
	tmpDir := t.TempDir()

	realDir := filepath.Join(tmpDir, "realdir")
	if err := os.Mkdir(realDir, 0755); err != nil {
		t.Fatal(err)
	}

	symlinkDir := filepath.Join(tmpDir, "linkdir")
	if err := os.Symlink(realDir, symlinkDir); err != nil {
		t.Fatal(err)
	}

	dstDir := filepath.Join(tmpDir, "dst")
	err := CopyDir(symlinkDir, dstDir)
	if err == nil {
		t.Fatal("expected error when source root is a symlink")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection error, got: %v", err)
	}
}

func TestCopyDir_NonexistentSource(t *testing.T) {
	tmpDir := t.TempDir()
	err := CopyDir(filepath.Join(tmpDir, "no_such_dir"), filepath.Join(tmpDir, "dst"))
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
}

func TestCopyFile_PreservesPermissions(t *testing.T) {
	tmpDir := t.TempDir()

	srcPath := filepath.Join(tmpDir, "script.sh")
	if err := os.WriteFile(srcPath, []byte("#!/bin/sh\necho hi"), 0755); err != nil {
		t.Fatal(err)
	}

	dstPath := filepath.Join(tmpDir, "script_copy.sh")
	if err := CopyFile(srcPath, dstPath); err != nil {
		t.Fatalf("CopyFile: %v", err)
	}

	fi, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("stat dest: %v", err)
	}
	// CopyFile masks with 0755, so owner execute bit should be preserved
	mode := fi.Mode().Perm()
	if mode&0100 == 0 {
		t.Errorf("expected owner execute bit to be set, got mode %o", mode)
	}
	// Group/other write bits should be stripped
	if mode&0022 != 0 {
		t.Errorf("expected group/other write bits to be stripped, got mode %o", mode)
	}
}

func TestAtomicWriteFile_Basic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile.txt")
	data := []byte("hello atomic write")

	err := AtomicWriteFile(path, data, 0644)
	if err != nil {
		t.Fatalf("AtomicWriteFile failed: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("Expected content %q, got %q", string(data), string(got))
	}
}

func TestAtomicWriteFile_Permissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "permfile.txt")

	err := AtomicWriteFile(path, []byte("perm test"), 0600)
	if err != nil {
		t.Fatalf("AtomicWriteFile failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestAtomicWriteFile_Overwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overwrite.txt")

	// Write initial content
	err := AtomicWriteFile(path, []byte("original"), 0644)
	if err != nil {
		t.Fatalf("First write failed: %v", err)
	}

	// Overwrite with new content
	err = AtomicWriteFile(path, []byte("replaced"), 0644)
	if err != nil {
		t.Fatalf("Overwrite failed: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(got) != "replaced" {
		t.Errorf("Expected content %q, got %q", "replaced", string(got))
	}
}

func TestAtomicWriteFile_InvalidDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "file.txt")

	err := AtomicWriteFile(path, []byte("data"), 0644)
	if err == nil {
		t.Fatal("Expected error for non-existent directory, got nil")
	}
}

func TestCopyFile_NonexistentSource(t *testing.T) {
	tmpDir := t.TempDir()
	err := CopyFile(filepath.Join(tmpDir, "nonexistent.txt"), filepath.Join(tmpDir, "dst.txt"))
	if err == nil {
		t.Fatal("expected error for nonexistent source file")
	}
}

func TestCreateSymlinkOrCopy_NonexistentSource(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a target path inside a read-only directory so symlink creation fails,
	// then the fallback stat will also fail because source doesn't exist.
	badSource := filepath.Join(tmpDir, "does_not_exist")
	roDir := filepath.Join(tmpDir, "readonly")
	if err := os.Mkdir(roDir, 0555); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(roDir, "link")

	err := CreateSymlinkOrCopy(badSource, target)
	if err == nil {
		t.Fatal("expected error when source does not exist and symlink fails")
	}
	if !strings.Contains(err.Error(), "failed to stat source") {
		t.Fatalf("expected 'failed to stat source' error, got: %v", err)
	}
	// Restore permissions so TempDir cleanup works
	_ = os.Chmod(roDir, 0755)
}
