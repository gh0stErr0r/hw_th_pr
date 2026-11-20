package core

import (
	"fmt"
	"io"

	"genc/04-coding/internal/secretmem"
)

// Encrypt формирует контейнер GENC v1 для переданных байтов и пароля.
// Из rnd добирается ровно столько случайных байтов, сколько требует формат.
func Encrypt(
	plaintext, password []byte, p Params, rnd io.Reader,
) ([]byte, error) {
	if len(plaintext) > MaxInputSize {
		return nil, ErrInputTooLarge
	}
	if !p.valid() {
		return nil, ErrBadParams
	}

	// Один буфер вмещает и заголовок, и результат Seal.
	header := make([]byte, HeaderSize, len(plaintext)+Overhead)
	pack(header, p)
	salt, nonce := header[11:27], header[27:HeaderSize]
	if _, err := io.ReadFull(rnd, header[11:]); err != nil {
		return nil, fmt.Errorf("random salt and nonce: %w", err)
	}

	// Вывод ключа и шифрование входят в одну область секретных вычислений.
	var blob []byte
	var err error
	secretmem.Do(func() {
		key := deriveKey(password, salt, p)
		defer clear(key)
		blob, err = seal(header, key, nonce, plaintext, header)
	})
	return blob, err
}

// ParseContainer проверяет формат контейнера и готовит его к расшифрованию.
// Разбор выполняется до чтения пароля и вычисления ключа.
func ParseContainer(blob []byte) (*Container, error) {
	return unpack(blob)
}

// Decrypt проверяет тег и возвращает исходные байты в случае успеха.
func (c *Container) Decrypt(password []byte) ([]byte, error) {
	// Неподготовленный контейнер отклоняется до вычисления размера буфера.
	if c == nil || len(c.sealed) < TagSize {
		return nil, ErrTooShort
	}

	// Буфер результата выделяется вне secretmem.Do, ключ остаётся внутри.
	dst := make([]byte, 0, len(c.sealed)-TagSize)
	var plaintext []byte
	var err error
	secretmem.Do(func() {
		key := deriveKey(password, c.salt, c.params)
		defer clear(key)
		plaintext, err = open(dst, key, c.nonce, c.sealed, c.header)
	})
	return plaintext, err
}
