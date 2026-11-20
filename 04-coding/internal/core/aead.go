package core

import (
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
)

// seal дописывает шифртекст и тег к dst. Заголовок передаётся в aad
// и участвует в вычислении тега.
func seal(dst, key, nonce, plaintext, aad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	return aead.Seal(dst, nonce, plaintext, aad), nil
}

// open возвращает данные только после успешной проверки тега библиотекой.
// CLI сообщает об ошибке аутентификации одинаково для неверного пароля
// и изменённых данных.
func open(dst, key, nonce, sealed, aad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	plaintext, err := aead.Open(dst, nonce, sealed, aad)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAuthFailed, err)
	}
	return plaintext, nil
}
