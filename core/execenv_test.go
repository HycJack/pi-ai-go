package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultExecutionEnv(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pi-ai-go-execenv-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	env := NewDefaultExecutionEnvWithDir(tmpDir)

	t.Run("GetWorkingDir", func(t *testing.T) {
		if got := env.GetWorkingDir(); got != tmpDir {
			t.Errorf("GetWorkingDir() = %q, want %q", got, tmpDir)
		}
	})

	t.Run("SetWorkingDir", func(t *testing.T) {
		newDir := filepath.Join(tmpDir, "subdir")
		if err := env.SetWorkingDir(newDir); err != nil {
			t.Fatal(err)
		}
		if got := env.GetWorkingDir(); got != newDir {
			t.Errorf("SetWorkingDir() = %q, want %q", got, newDir)
		}
	})

	t.Run("ExpandPath", func(t *testing.T) {
		env.SetWorkingDir(tmpDir)
		if got := env.ExpandPath("test.txt"); got != filepath.Join(tmpDir, "test.txt") {
			t.Errorf("ExpandPath() = %q, want %q", got, filepath.Join(tmpDir, "test.txt"))
		}
		if got := env.ExpandPath("/absolute/path.txt"); got != "/absolute/path.txt" {
			t.Errorf("ExpandPath(absolute) = %q, want %q", got, "/absolute/path.txt")
		}
	})

	t.Run("Mkdir", func(t *testing.T) {
		env.SetWorkingDir(tmpDir)
		if err := env.Mkdir("testdir", 0755); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(tmpDir, "testdir")); os.IsNotExist(err) {
			t.Error("Mkdir() failed to create directory")
		}
	})

	t.Run("WriteFile and ReadFile", func(t *testing.T) {
		env.SetWorkingDir(tmpDir)
		content := []byte("test content")
		if err := env.WriteFile("test.txt", content, 0644); err != nil {
			t.Fatal(err)
		}
		readContent, err := env.ReadFile("test.txt")
		if err != nil {
			t.Fatal(err)
		}
		if string(readContent) != string(content) {
			t.Errorf("ReadFile() = %q, want %q", readContent, content)
		}
	})

	t.Run("AppendFile", func(t *testing.T) {
		env.SetWorkingDir(tmpDir)
		if err := env.WriteFile("append.txt", []byte("first"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := env.AppendFile("append.txt", []byte("second")); err != nil {
			t.Fatal(err)
		}
		readContent, err := env.ReadFile("append.txt")
		if err != nil {
			t.Fatal(err)
		}
		if string(readContent) != "firstsecond" {
			t.Errorf("AppendFile() = %q, want %q", readContent, "firstsecond")
		}
	})

	t.Run("ListDir", func(t *testing.T) {
		env.SetWorkingDir(tmpDir)
		if err := env.WriteFile("file1.txt", []byte("1"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := env.WriteFile("file2.txt", []byte("2"), 0644); err != nil {
			t.Fatal(err)
		}
		files, err := env.ListDir(".")
		if err != nil {
			t.Fatal(err)
		}
		if len(files) < 2 {
			t.Errorf("ListDir() returned %d files, expected at least 2", len(files))
		}
	})

	t.Run("Remove", func(t *testing.T) {
		env.SetWorkingDir(tmpDir)
		if err := env.WriteFile("toremove.txt", []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		if err := env.Remove("toremove.txt"); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(tmpDir, "toremove.txt")); !os.IsNotExist(err) {
			t.Error("Remove() failed to remove file")
		}
	})
}

func TestExecutionEnvContext(t *testing.T) {
	env := NewDefaultExecutionEnv()

	t.Run("WithExecutionEnv and GetExecutionEnv", func(t *testing.T) {
		ctx := WithExecutionEnv(context.Background(), env)
		got := GetExecutionEnv(ctx)
		if got != env {
			t.Error("GetExecutionEnv() returned wrong ExecutionEnv")
		}
	})

	t.Run("GetExecutionEnv with nil context", func(t *testing.T) {
		got := GetExecutionEnv(nil)
		if got != nil {
			t.Error("GetExecutionEnv(nil) should return nil")
		}
	})

	t.Run("GetExecutionEnv without ExecutionEnv", func(t *testing.T) {
		ctx := context.Background()
		got := GetExecutionEnv(ctx)
		if got != nil {
			t.Error("GetExecutionEnv() should return nil when no ExecutionEnv is set")
		}
	})
}
