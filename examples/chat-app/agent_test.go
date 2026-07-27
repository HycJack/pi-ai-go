package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadTextFilePathTraversal(t *testing.T) {
	// Create a temp dir to simulate working directory with a "secret" outside.
	tmpDir, err := os.MkdirTemp("", "chat-app-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	workDir := filepath.Join(tmpDir, "workspace")
	outsideFile := filepath.Join(tmpDir, "secret.txt")
	insideFile := filepath.Join(workDir, "hello.txt")

	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outsideFile, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(insideFile, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	app := &App{
		settings: AppSettings{
			WorkingDir: workDir,
		},
	}

	t.Run("allow file inside working dir", func(t *testing.T) {
		content, err := app.ReadTextFile(insideFile)
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
		if content != "hello world" {
			t.Errorf("expected 'hello world', got %q", content)
		}
	})

	t.Run("deny file outside working dir via ..", func(t *testing.T) {
		_, err := app.ReadTextFile(outsideFile)
		if err == nil {
			t.Fatal("expected error for path outside working dir, got nil")
		}
	})

	t.Run("deny file outside via relative ..", func(t *testing.T) {
		// Construct a relative path like ../secret.txt
		relPath := filepath.Join(workDir, "..", "secret.txt")
		_, err := app.ReadTextFile(relPath)
		if err == nil {
			t.Fatal("expected error for relative path escaping working dir, got nil")
		}
	})

	t.Run("deny path traversal in deep subpath", func(t *testing.T) {
		relPath := filepath.Join(workDir, "subdir", "..", "..", "secret.txt")
		_, err := app.ReadTextFile(relPath)
		if err == nil {
			t.Fatal("expected error for deep path traversal, got nil")
		}
	})

	t.Run("empty working dir uses process CWD", func(t *testing.T) {
		app2 := &App{
			settings: AppSettings{
				WorkingDir: "",
			},
		}
		// When WorkingDir is empty, ReadTextFile uses os.Getwd() as the base.
		// So an absolute path outside CWD is expected to be denied.
		wd, _ := os.Getwd()
		relFromWD, _ := filepath.Rel(wd, outsideFile)
		// If the file isn't relative to CWD, access is denied — this is correct.
		_, err := app2.ReadTextFile(outsideFile)
		if len(relFromWD) >= 2 && relFromWD[:2] == ".." {
			// outsideFile is not under CWD — expect denial
			if err == nil {
				t.Error("expected error when file is outside process CWD, got nil")
			}
		} else {
			// outsideFile happens to be under CWD — expect success
			content, err := app2.ReadTextFile(outsideFile)
			if err != nil {
				t.Fatalf("expected success, got error: %v", err)
			}
			if content != "secret" {
				t.Errorf("expected 'secret', got %q", content)
			}
		}
	})
}
