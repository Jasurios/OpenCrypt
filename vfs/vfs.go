package vfs

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
)

type FS struct {
	root string
}

func New(root string) sftp.Handlers {
	fs := &FS{root: filepath.Clean(root)}
	return sftp.Handlers{
		FileGet:  fs,
		FilePut:  fs,
		FileCmd:  fs,
		FileList: fs,
	}
}

// Проверяем что путь внутри root
func (fs *FS) safePath(p string) string {
	full := filepath.Join(fs.root, p)
	full = filepath.Clean(full)
	if !strings.HasPrefix(full, fs.root) {
		return fs.root
	}
	return full
}

func (fs *FS) Fileread(r *sftp.Request) (io.ReaderAt, error) {
	return os.Open(fs.safePath(r.Filepath))
}

func (fs *FS) Filewrite(r *sftp.Request) (io.WriterAt, error) {
	return os.OpenFile(fs.safePath(r.Filepath), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
}

func (fs *FS) Filecmd(r *sftp.Request) error {
	switch r.Method {
	case "Rename":
		return os.Rename(fs.safePath(r.Filepath), fs.safePath(r.Target))
	case "Remove":
		return os.Remove(fs.safePath(r.Filepath))
	case "Mkdir":
		return os.MkdirAll(fs.safePath(r.Filepath), 0755)
	case "Rmdir":
		return os.Remove(fs.safePath(r.Filepath))
	case "Setstat":
		return nil
	}
	return nil
}

func (fs *FS) Filelist(r *sftp.Request) (sftp.ListerAt, error) {
	switch r.Method {
	case "List":
		entries, err := os.ReadDir(fs.safePath(r.Filepath))
		if err != nil {
			return nil, err
		}
		infos := make([]os.FileInfo, 0, len(entries))
		for _, e := range entries {
			info, err := e.Info()
			if err == nil {
				infos = append(infos, info)
			}
		}
		return listerat(infos), nil

	case "Stat":
		info, err := os.Stat(fs.safePath(r.Filepath))
		if err != nil {
			return nil, err
		}
		return listerat([]os.FileInfo{info}), nil
	}
	return nil, nil
}

type listerat []os.FileInfo

func (l listerat) ListAt(ls []os.FileInfo, offset int64) (int, error) {
	if offset >= int64(len(l)) {
		return 0, io.EOF
	}
	n := copy(ls, l[offset:])
	if n < len(ls) {
		return n, io.EOF
	}
	return n, nil
}