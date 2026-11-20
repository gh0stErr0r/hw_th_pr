package fsio

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

var (
	ErrTooLarge     = errors.New("file exceeds the size limit")
	ErrNotRegular   = errors.New("путь не указывает на обычный файл")
	ErrOutputExists = errors.New("output already exists")
	ErrSameFile     = errors.New("input and output are the same file")
)

// CheckPaths проверяет выходной путь до чтения файла и запроса пароля.
func CheckPaths(in, out string, force bool) error {
	if in == out {
		return ErrSameFile
	}
	entry, err := os.Lstat(out)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	// Lstat замечает даже битую ссылку, а Stat позволяет сравнить сам файл.
	target := entry
	if entry.Mode()&os.ModeSymlink != 0 {
		target, _ = os.Stat(out)
	}
	source, err := os.Stat(in)
	if err == nil && target != nil && os.SameFile(source, target) {
		return ErrSameFile
	}
	if !force {
		return ErrOutputExists
	}
	return nil
}

// ReadFile принимает только обычные файлы и ограничивает объём чтения.
func ReadFile(path string, limit int64) ([]byte, error) {
	// Неблокирующее открытие позволяет сразу отклонить именованный канал.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrNotRegular
	}
	if info.Size() > limit {
		return nil, ErrTooLarge
	}

	// Выделяем буфер по известному размеру и отдельно проверяем,
	// появились ли новые байты после получения метаданных.
	data := make([]byte, info.Size())
	n, err := io.ReadFull(f, data)
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return data[:n], nil
	}
	if err != nil {
		return nil, err
	}

	var extra [1]byte
	n, err = f.Read(extra[:])
	if errors.Is(err, io.EOF) {
		return data, nil
	}
	if err != nil {
		return nil, err
	}
	if int64(len(data))+int64(n) > limit {
		return nil, ErrTooLarge
	}
	data = append(data, extra[:n]...)
	tail, err := io.ReadAll(io.LimitReader(f, limit-int64(len(data))+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data))+int64(len(tail)) > limit {
		return nil, ErrTooLarge
	}
	return append(data, tail...), nil
}

// WriteAtomic публикует полностью записанный файл. Без force существующий
// путь сохраняется даже при его появлении после предварительной проверки.
func WriteAtomic(path string, data []byte, force bool) error {
	// Временный файл создаётся в том же каталоге, что и результат.
	dir, _ := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	tmp, err := os.CreateTemp(dir, ".genc-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() {
		if name != "" {
			os.Remove(name)
		}
	}()

	// Синхронизируем и закрываем файл до публикации конечного имени.
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}

	if force {
		if err := os.Rename(name, path); err != nil {
			return err
		}
		name = ""
		return nil
	}
	// Link атомарно создаёт конечное имя и отказывает, если путь уже занят.
	if err := os.Link(name, path); errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%w: %v", ErrOutputExists, err)
	} else {
		return err
	}
}
