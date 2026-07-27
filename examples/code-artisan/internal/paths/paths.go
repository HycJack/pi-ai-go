package paths

import (
	"os"
	"path/filepath"
)

func AppRoot() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	exeDir := filepath.Dir(exePath)
	if isProjectRoot(exeDir) || isRuntimeRoot(exeDir) {
		return exeDir, nil
	}

	cwd, err := os.Getwd()
	if err == nil && (isProjectRoot(cwd) || isRuntimeRoot(cwd)) {
		return cwd, nil
	}

	return exeDir, nil
}

func ScriptsDir() (string, error) {
	root, err := AppRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "scripts"), nil
}

func RuntimeDir() (string, error) {
	root, err := AppRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "runtime"), nil
}

func RuntimeArchivePath() (string, error) {
	root, err := AppRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "resources", "runtime", "runtime.zip"), nil
}

func RuntimeTempDir() (string, error) {
	root, err := AppRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "runtime.tmp"), nil
}

func isProjectRoot(root string) bool {
	return fileExists(filepath.Join(root, "go.mod")) &&
		fileExists(filepath.Join(root, "wails.json"))
}

func isRuntimeRoot(root string) bool {
	return fileExists(filepath.Join(root, "resources", "runtime"))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
