package core

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
)

// DefaultExecutionEnv is a basic ExecutionEnv implementation that
// interacts with the real filesystem and shell.
type DefaultExecutionEnv struct {
	workingDir string
}

// NewDefaultExecutionEnv creates a new DefaultExecutionEnv with the
// current working directory.
func NewDefaultExecutionEnv() *DefaultExecutionEnv {
	wd, _ := os.Getwd()
	return &DefaultExecutionEnv{workingDir: wd}
}

// NewDefaultExecutionEnvWithDir creates a new DefaultExecutionEnv with
// the specified working directory.
func NewDefaultExecutionEnvWithDir(workingDir string) *DefaultExecutionEnv {
	return &DefaultExecutionEnv{workingDir: workingDir}
}

func (e *DefaultExecutionEnv) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(e.ExpandPath(path))
}

func (e *DefaultExecutionEnv) WriteFile(path string, content []byte, perm uint32) error {
	return os.WriteFile(e.ExpandPath(path), content, os.FileMode(perm))
}

func (e *DefaultExecutionEnv) AppendFile(path string, content []byte) error {
	f, err := os.OpenFile(e.ExpandPath(path), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(content)
	return err
}

func (e *DefaultExecutionEnv) ListDir(path string) ([]string, error) {
	entries, err := os.ReadDir(e.ExpandPath(path))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, nil
}

func (e *DefaultExecutionEnv) Mkdir(path string, perm uint32) error {
	return os.MkdirAll(e.ExpandPath(path), os.FileMode(perm))
}

func (e *DefaultExecutionEnv) Remove(path string) error {
	return os.RemoveAll(e.ExpandPath(path))
}

func (e *DefaultExecutionEnv) Exec(cmd string, args []string, workingDir string) (string, string, error) {
	c := exec.Command(cmd, args...)
	if workingDir != "" {
		c.Dir = e.ExpandPath(workingDir)
	} else {
		c.Dir = e.workingDir
	}

	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr

	err := c.Run()
	return stdout.String(), stderr.String(), err
}

func (e *DefaultExecutionEnv) GetWorkingDir() string {
	return e.workingDir
}

func (e *DefaultExecutionEnv) SetWorkingDir(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	e.workingDir = absPath
	return nil
}

func (e *DefaultExecutionEnv) ExpandPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(e.workingDir, path)
}
