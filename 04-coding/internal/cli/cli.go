package cli

import (
	"crypto/subtle"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"genc/04-coding/internal/core"
	"genc/04-coding/internal/fsio"
	"genc/04-coding/internal/pwd"
)

// Env задаёт ресурсы CLI. В тестах можно передать свой источник паролей,
// случайных байтов и параметры вывода ключа.
type Env struct {
	Stdin     *os.File
	Stdout    io.Writer
	Stderr    io.Writer
	Passwords pwd.Reader
	Random    io.Reader
	Params    core.Params
}

// Run выполняет команду и возвращает код завершения процесса.
func Run(args []string, env Env) int {
	const options = " -in <файл> -out <файл> [-force]"
	const usage = "Использование: genc <encrypt|decrypt>" + options
	stderr := env.Stderr
	if len(args) == 1 &&
		(args[0] == "-h" || args[0] == "--help" || args[0] == "-help") {
		fmt.Fprintln(stderr, usage)
		return 0
	}
	if len(args) == 0 || (args[0] != "encrypt" && args[0] != "decrypt") {
		fmt.Fprintln(stderr, "ошибка: укажите команду encrypt или decrypt")
		fmt.Fprintln(stderr, usage)
		return 2
	}

	// Свой FlagSet изолирует аргументы команды и русскоязычные сообщения.
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	in := flags.String("in", "", "")
	out := flags.String("out", "", "")
	force := flags.Bool("force", false, "")
	err := flags.Parse(args[1:])
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(stderr, "Использование: genc "+args[0]+options)
		return 0
	}
	if err != nil || *in == "" || *out == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "ошибка: неверные аргументы команды")
		fmt.Fprintln(stderr, "Использование: genc "+args[0]+options)
		return 2
	}

	if err := fsio.CheckPaths(*in, *out, *force); err != nil {
		return fail(stderr, "не удалось записать файл", err)
	}

	decrypting := args[0] == "decrypt"
	// Лимит контейнера включает 55 байт заголовка и тега.
	limit := int64(core.MaxInputSize)
	if decrypting {
		limit = core.MaxContainerSize
	}
	input, err := fsio.ReadFile(*in, limit)
	if err != nil {
		return fail(stderr, "не удалось прочитать файл", err)
	}

	// Повреждённый заголовок отклоняется до запроса пароля.
	var container *core.Container
	if decrypting {
		container, err = core.ParseContainer(input)
		if err != nil {
			return fail(stderr, "не удалось прочитать файл", err)
		}
	}

	reader := env.Passwords
	if reader == nil {
		reader = pwd.Terminal{In: env.Stdin, Err: stderr}
	}
	password, err := readPassword(reader, !decrypting)
	if err != nil {
		return fail(stderr, "", err)
	}
	defer clear(password)

	var output []byte
	if decrypting {
		output, err = container.Decrypt(password)
	} else {
		output, err = core.Encrypt(input, password, env.Params, env.Random)
	}
	// Пароль можно очистить сразу по завершении криптографической операции.
	clear(password)
	if err != nil {
		return fail(stderr, "не удалось обработать файл", err)
	}

	if err := fsio.WriteAtomic(*out, output, *force); err != nil {
		return fail(stderr, "не удалось записать файл", err)
	}
	if decrypting {
		fmt.Fprintf(stderr, "Готово: файл расшифрован в %s\n", *out)
	} else {
		fmt.Fprintf(stderr, "Готово: зашифрованный файл сохранён в %s\n", *out)
	}
	return 0
}

// readPassword требует непустой пароль и сравнивает подтверждение,
// если оно нужно. Буферы очищаются при каждом отказе.
func readPassword(reader pwd.Reader, confirm bool) (_ []byte, err error) {
	password, err := reader.ReadPassword("Пароль: ")
	defer func() {
		if err != nil {
			clear(password)
		}
	}()
	if errors.Is(err, pwd.ErrNoTerminal) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать пароль: %w", err)
	}
	if len(password) == 0 {
		return nil, errors.New("пароль не может быть пустым")
	}
	if confirm {
		confirmation, err := reader.ReadPassword("Повторите пароль: ")
		defer clear(confirmation)
		if errors.Is(err, pwd.ErrNoTerminal) {
			return nil, err
		}
		if err != nil {
			return nil, fmt.Errorf("не удалось прочитать пароль: %w", err)
		}
		if subtle.ConstantTimeCompare(password, confirmation) != 1 {
			return nil, errors.New("пароли не совпадают")
		}
	}
	return password, nil
}

// fail сопоставляет ошибки внутренних пакетов с сообщениями программы
// и возвращает код 1.
func fail(stderr io.Writer, operation string, err error) int {
	var message string
	switch {
	case errors.Is(err, fsio.ErrSameFile):
		message = "входной и выходной файлы совпадают"
	case errors.Is(err, fsio.ErrOutputExists):
		message = "выходной файл уже существует (используйте -force)"
	case errors.Is(err, fsio.ErrTooLarge),
		errors.Is(err, core.ErrInputTooLarge):
		message = "файл слишком большой (максимум 1 ГиБ)"
	case errors.Is(err, core.ErrTooShort), errors.Is(err, core.ErrBadMagic),
		errors.Is(err, core.ErrUnsupportedVersion),
		errors.Is(err, core.ErrBadParams):
		message = "неверный формат файла"
	case errors.Is(err, core.ErrAuthFailed):
		message = "неверный пароль или файл повреждён"
	default:
		message = err.Error()
		if operation != "" {
			message = operation + ": " + message
		}
	}
	fmt.Fprintln(stderr, "ошибка: "+message)
	return 1
}
