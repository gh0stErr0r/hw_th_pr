package core

import "encoding/binary"

const magic = "GENC"
const version = 1

// Container ссылается на байты исходного буфера. Вызывающий код
// сохраняет их неизменными до завершения работы с контейнером.
type Container struct {
	params Params
	header []byte
	salt   []byte
	nonce  []byte
	sealed []byte
}

// pack записывает сигнатуру, версию и параметры KDF по смещениям GENC v1.
// Вызывающий код заполняет оставшиеся байты заголовка солью и nonce.
func pack(header []byte, p Params) {
	copy(header[:4], magic)
	header[4] = version
	header[5] = p.Time
	binary.LittleEndian.PutUint32(header[6:10], p.MemoryKiB)
	header[10] = p.Threads
}

// unpack проверяет заголовок до нарезки буфера и вызова Argon2id.
// Заголовок и шифртекст остаются срезами исходного файла.
func unpack(blob []byte) (*Container, error) {
	if len(blob) < Overhead {
		return nil, ErrTooShort
	}
	if string(blob[:4]) != magic {
		return nil, ErrBadMagic
	}
	if blob[4] != version {
		return nil, ErrUnsupportedVersion
	}
	p := Params{
		Time:      blob[5],
		MemoryKiB: binary.LittleEndian.Uint32(blob[6:10]),
		Threads:   blob[10],
	}
	if !p.valid() {
		return nil, ErrBadParams
	}
	return &Container{
		params: p,
		header: blob[:HeaderSize],
		salt:   blob[11:27],
		nonce:  blob[27:HeaderSize],
		sealed: blob[HeaderSize:],
	}, nil
}
