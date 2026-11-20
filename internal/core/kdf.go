package core

import "golang.org/x/crypto/argon2"

const keySize = 32

// deriveKey вычисляет 32-байтовый ключ из пароля, соли и проверенных
// параметров. Вызывающая функция отвечает за очистку ключа.
func deriveKey(password, salt []byte, p Params) []byte {
	return argon2.IDKey(
		password, salt, uint32(p.Time), p.MemoryKiB, p.Threads, keySize,
	)
}
