package utils

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/apppath"
)

func CreateDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func RemoveDir(path string) error {
	return os.RemoveAll(path)
}

func DirExists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func GetDirNames(path string) []string {
	dir, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer dir.Close()
	names, _ := dir.Readdirnames(-1)
	return names
}

func GetLastDirName(path string) string {
	return filepath.Base(path)
}

func GetAppDataPath() string {
	return apppath.AppData()
}

func GetMinecraftGDKDataPath(ispreview bool) string {
	return apppath.MinecraftData(ispreview)
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func ZipDir(srcDir, destZip string) error {
	out, err := os.Create(destZip)
	if err != nil {
		return err
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	defer zw.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)

		if info.IsDir() {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = rel
		header.Method = zip.Deflate

		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(writer, f)
		return err
	})
}

func DirSize(path string) int64 {
	var total int64
	root := filepath.Clean(strings.TrimSpace(path))
	if root == "" {
		return 0
	}
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total
}

func SanitizeFilename(name string) string {
	s := strings.TrimSpace(name)
	if s == "" {
		return "world"
	}
	invalid := "<>:\"/\\|?*"
	var b strings.Builder
	for _, r := range s {
		if r < 32 || strings.ContainsRune(invalid, r) {
			b.WriteRune('_')
		} else {
			b.WriteRune(r)
		}
	}
	cleaned := strings.TrimRight(b.String(), " .")
	if cleaned == "" {
		return "world"
	}
	return cleaned
}

func CopyDir(src, dst string) error {
	src = filepath.Clean(strings.TrimSpace(src))
	dst = filepath.Clean(strings.TrimSpace(dst))
	if src == "" || dst == "" {
		return os.ErrInvalid
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return os.ErrInvalid
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return err
		}
		defer out.Close()
		if _, err := io.Copy(out, in); err != nil {
			return err
		}
		return nil
	})
}

func JsonCompatBytes(data []byte) []byte {
	src := data
	n := len(src)
	out := make([]byte, 0, n)
	inString := false
	quote := byte(0)
	esc := false
	inLineComment := false
	inBlockComment := false
	for i := 0; i < n; i++ {
		b := src[i]
		if inLineComment {
			if b == '\n' || b == '\r' {
				inLineComment = false
				out = append(out, b)
			}
			continue
		}
		if inBlockComment {
			if b == '*' && i+1 < n && src[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if inString {
			out = append(out, b)
			if esc {
				esc = false
				continue
			}
			if b == '\\' {
				esc = true
			} else if b == quote {
				inString = false
				quote = 0
			}
			continue
		}
		if b == '"' || b == '\'' {
			inString = true
			quote = b
			out = append(out, b)
			continue
		}
		if b == '/' && i+1 < n {
			nb := src[i+1]
			if nb == '/' {
				inLineComment = true
				i++
				continue
			}
			if nb == '*' {
				inBlockComment = true
				i++
				continue
			}
		}
		if b == ',' {
			j := i + 1
			for j < n {
				c := src[j]
				if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
					j++
					continue
				}
				if c == '}' || c == ']' {
					goto next
				}
				break
			}
			out = append(out, b)
		next:
			continue
		}
		out = append(out, b)
	}
	return out
}
