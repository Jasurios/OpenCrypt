package backup

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

var zstdEnc *zstd.Encoder

func init() {
	zstdEnc, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	zip.RegisterCompressor(93, func(out io.Writer) (io.WriteCloser, error) {
		zstdEnc.Reset(out)
		return zstdEnc, nil
	})
}

func StartWatcher(userDir string) {
	go watch(userDir)
}

func watch(userDir string) {
	for {
		nowFile := filepath.Join(userDir, "now.now")
		if _, err := os.Stat(nowFile); err == nil {
			os.Remove(nowFile)
			log.Printf("Бэкап %s: запущен по now.now", userDir)
			if err := Do(userDir); err != nil {
				log.Printf("Бэкап %s: ошибка: %v", userDir, err)
			}
			time.Sleep(time.Minute)
			continue
		}

		interval := readInterval(userDir)
		if interval > 0 {
			time.Sleep(time.Duration(interval) * time.Minute)
			log.Printf("Бэкап %s: запущен по расписанию", userDir)
			if err := Do(userDir); err != nil {
				log.Printf("Бэкап %s: ошибка: %v", userDir, err)
			}
		} else {
			time.Sleep(30 * time.Second)
		}
	}
}

func readInterval(userDir string) int {
	data, err := os.ReadFile(filepath.Join(userDir, "backup_interval.txt"))
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func Do(userDir string) error {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	zipName := fmt.Sprintf("backup_%s.zip", timestamp)
	zipPath := filepath.Join(userDir, zipName)

	if err := createZip(userDir, zipPath); err != nil {
		os.Remove(zipPath)
		return fmt.Errorf("ошибка архивации: %w", err)
	}

	if err := cleanUserDir(userDir, zipName); err != nil {
		return fmt.Errorf("ошибка очистки: %w", err)
	}

	log.Printf("Бэкап %s: готово -> %s", userDir, zipName)
	return nil
}

func createZip(userDir string, destZip string) error {
	f, err := os.Create(destZip)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	return filepath.Walk(userDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(userDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		if strings.HasSuffix(rel, ".zip") || rel == "backup_interval.txt" {
			return nil
		}

		if info.IsDir() {
			_, err = w.Create(rel + "/")
			return err
		}

		header := &zip.FileHeader{
			Name:     rel,
			Method:   93,
			Modified: info.ModTime(),
		}
		writer, err := w.CreateHeader(header)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}

func cleanUserDir(userDir string, keepFile string) error {
	entries, err := os.ReadDir(userDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if name == keepFile || name == "backup_interval.txt" {
			continue
		}
		os.RemoveAll(filepath.Join(userDir, name))
	}
	return nil
}