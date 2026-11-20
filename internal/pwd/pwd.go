package pwd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

var ErrNoTerminal = errors.New("ввод пароля требует терминал")

// Reader передаёт владение буфером пароля вызывающему коду для очистки.
type Reader interface {
	ReadPassword(prompt string) ([]byte, error)
}

// Terminal читает пароль с In и отправляет приглашения в Err.
type Terminal struct {
	In  *os.File
	Err io.Writer
}

// ReadPassword проверяет терминал до вывода приглашения. Пакет term
// отключает эхо на время чтения и восстанавливает настройки при выходе.
func (t Terminal) ReadPassword(prompt string) ([]byte, error) {
	if t.In == nil || !term.IsTerminal(int(t.In.Fd())) {
		return nil, ErrNoTerminal
	}
	fmt.Fprint(t.Err, prompt)
	password, err := term.ReadPassword(int(t.In.Fd()))
	fmt.Fprintln(t.Err)
	return password, err
}
