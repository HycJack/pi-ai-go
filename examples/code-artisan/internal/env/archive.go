package env

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"code-artisan/internal/paths"
)

func (m *Manager) ensureRuntimeExtracted(onProgress func(Progress)) error {
	reportProgress(onProgress, Progress{
		Stage:   "checking",
		Message: "检查运行环境",
		Percent: 12,
	})

	status, err := m.Status()
	if err != nil {
		return err
	}
	if status.Ready {
		reportProgress(onProgress, Progress{
			Stage:   "ready",
			Message: "运行环境已就绪",
			Percent: 100,
		})
		return nil
	}

	archivePath, err := paths.RuntimeArchivePath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(archivePath); err != nil {
		return fmt.Errorf("runtime 压缩包未找到: %s", archivePath)
	}

	reportProgress(onProgress, Progress{
		Stage:   "extracting",
		Message: "正在解压运行环境",
		Percent: 18,
	})

	runtimeDir, err := paths.RuntimeDir()
	if err != nil {
		return err
	}

	tempDir, err := paths.RuntimeTempDir()
	if err != nil {
		return err
	}

	if err := os.RemoveAll(tempDir); err != nil {
		return err
	}
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return err
	}

	if err := extractArchive(archivePath, tempDir, onProgress); err != nil {
		_ = os.RemoveAll(tempDir)
		return err
	}

	reportProgress(onProgress, Progress{
		Stage:   "installing",
		Message: "正在安装运行环境",
		Percent: 92,
	})

	if err := os.RemoveAll(runtimeDir); err != nil {
		_ = os.RemoveAll(tempDir)
		return err
	}
	if err := os.Rename(tempDir, runtimeDir); err != nil {
		_ = os.RemoveAll(tempDir)
		return err
	}

	status, err = m.Status()
	if err != nil {
		return err
	}
	if !status.Ready {
		return fmt.Errorf("runtime 解压后仍不完整")
	}

	reportProgress(onProgress, Progress{
		Stage:   "ready",
		Message: "运行环境已就绪",
		Percent: 100,
	})

	return nil
}

func extractArchive(archivePath string, targetDir string, onProgress func(Progress)) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	totalFiles := 0
	for _, file := range reader.File {
		if !file.FileInfo().IsDir() {
			totalFiles++
		}
	}

	processedFiles := 0
	for _, file := range reader.File {
		relativePath := normalizeArchivePath(file.Name)
		if relativePath == "" {
			continue
		}

		targetPath := filepath.Join(targetDir, filepath.FromSlash(relativePath))
		cleanTargetPath := filepath.Clean(targetPath)
		cleanRoot := filepath.Clean(targetDir) + string(os.PathSeparator)
		if cleanTargetPath != filepath.Clean(targetDir) && !strings.HasPrefix(cleanTargetPath, cleanRoot) {
			return fmt.Errorf("非法的 runtime 压缩包路径: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(cleanTargetPath, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(cleanTargetPath), 0o755); err != nil {
			return err
		}

		src, err := file.Open()
		if err != nil {
			return err
		}

		dst, err := os.OpenFile(cleanTargetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			src.Close()
			return err
		}

		_, copyErr := io.Copy(dst, src)
		if closeErr := dst.Close(); closeErr != nil {
			src.Close()
			return closeErr
		}
		src.Close()
		if copyErr != nil {
			return copyErr
		}

		processedFiles++
		if totalFiles > 0 {
			progressPercent := 18 + int(float64(processedFiles)/float64(totalFiles)*68.0)
			reportProgress(onProgress, Progress{
				Stage:   "extracting",
				Message: "正在解压运行环境",
				Percent: progressPercent,
			})
		}
	}

	return nil
}

func normalizeArchivePath(name string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.TrimPrefix(normalized, "/")
	if normalized == "" {
		return ""
	}

	// Skip the top-level "runtime" directory entry itself
	if normalized == "runtime" {
		return ""
	}
	if strings.HasPrefix(normalized, "runtime/") {
		return strings.TrimPrefix(normalized, "runtime/")
	}

	return normalized
}
