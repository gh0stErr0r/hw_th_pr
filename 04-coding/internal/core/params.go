package core

const (
	HeaderSize       = 39
	SaltSize         = 16
	NonceSize        = 12
	TagSize          = 16
	Overhead         = HeaderSize + TagSize
	MaxInputSize     = 1 << 30
	MaxContainerSize = MaxInputSize + Overhead
)

// Params хранит параметры Argon2id, записываемые в заголовок файла.
type Params struct {
	Time      uint8
	MemoryKiB uint32
	Threads   uint8
}

// DefaultParams задаёт стоимость вывода ключа для новых контейнеров.
var DefaultParams = Params{Time: 3, MemoryKiB: 64 * 1024, Threads: 4}

// valid ограничивает стоимость KDF до выделения памяти и учитывает
// минимум Argon2id в восемь блоков на поток.
func (p Params) valid() bool {
	return p.Time >= 1 && p.Time <= 10 &&
		p.Threads >= 1 && p.Threads <= 16 &&
		p.MemoryKiB >= 8*uint32(p.Threads) && p.MemoryKiB <= 1048576
}
