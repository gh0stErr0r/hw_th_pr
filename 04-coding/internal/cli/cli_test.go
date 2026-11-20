package cli

import (
	"bytes"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"genc/04-coding/internal/core"
)

type testReader struct {
	passwords [][]byte
	calls     int
	onRead    func()
}

func (r *testReader) ReadPassword(_ string) ([]byte, error) {
	r.calls++
	if r.onRead != nil {
		r.onRead()
	}
	if r.calls > len(r.passwords) {
		return nil, errors.New("unexpected password prompt")
	}
	return r.passwords[r.calls-1], nil
}

func assertRun(t *testing.T, args []string, reader *testReader, code int, message string, calls int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env := Env{
		Stdout: &stdout, Stderr: &stderr, Random: rand.Reader,
		Params: core.Params{Time: 1, MemoryKiB: 64, Threads: 1},
	}
	if reader != nil {
		env.Passwords = reader
	}
	if got := Run(args, env); got != code {
		t.Fatalf("exit = %d, want %d, stderr = %q", got, code, stderr.String())
	}
	if stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), message) {
		t.Fatalf("stdout = %q, stderr = %q; expected stderr prefix %q", stdout.String(), stderr.String(), message)
	}
	if reader != nil {
		if reader.calls != calls {
			t.Fatalf("password prompts = %d, want %d", reader.calls, calls)
		}
		for _, p := range reader.passwords[:calls] {
			if !bytes.Equal(p, make([]byte, len(p))) {
				t.Fatal("password was not cleared")
			}
		}
	}
}

func TestCLIRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in, encrypted := filepath.Join(dir, "in"), filepath.Join(dir, "data.genc")
	out, wrong := filepath.Join(dir, "out"), filepath.Join(dir, "wrong")
	data := []byte("привет\x00\xff")
	if err := os.WriteFile(in, data, 0600); err != nil {
		t.Fatal(err)
	}
	encrypt := []string{"encrypt", "-in", in, "-out", encrypted}
	passwords := func() *testReader {
		return &testReader{passwords: [][]byte{[]byte("pw"), []byte("pw")}}
	}
	success := "Готово: зашифрованный файл сохранён в " + encrypted + "\n"
	assertRun(t, encrypt, passwords(), 0, success, 2)
	assertRun(t, encrypt, &testReader{}, 1, "ошибка: выходной файл уже существует (используйте -force)\n", 0)
	assertRun(t, append(encrypt, "-force"), passwords(), 0, success, 2)
	assertRun(t, []string{"decrypt", "-in", encrypted, "-out", out},
		&testReader{passwords: [][]byte{[]byte("pw")}}, 0, "Готово: файл расшифрован в "+out+"\n", 1)
	if got, err := os.ReadFile(out); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("output = %q, %v", got, err)
	}
	assertRun(t, []string{"decrypt", "-in", encrypted, "-out", wrong},
		&testReader{passwords: [][]byte{[]byte("wrong")}}, 1, "ошибка: неверный пароль или файл повреждён\n", 1)
	assertRun(t, []string{"decrypt", "-in", encrypted, "-out", out, "-force"},
		&testReader{passwords: [][]byte{[]byte("wrong")}}, 1, "ошибка: неверный пароль или файл повреждён\n", 1)
	if _, err := os.Lstat(wrong); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("wrong-password output exists or could not be checked: %v", err)
	}
	if got, err := os.ReadFile(out); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("existing output changed after bad password: %q, %v", got, err)
	}
}

func TestCLIErrors(t *testing.T) {
	dir := t.TempDir()
	in, out := filepath.Join(dir, "in"), filepath.Join(dir, "out")
	if err := os.WriteFile(in, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	large := filepath.Join(dir, "large")
	if err := os.WriteFile(large, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(large, core.MaxContainerSize+1); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		command  string
		in, out  string
		password [][]byte
		calls    int
		msg      string
	}{
		{"empty password", "encrypt", in, out, [][]byte{nil}, 1, "пароль не может быть пустым\n"},
		{"mismatch", "encrypt", in, out, [][]byte{[]byte("pw"), []byte("other")}, 2, "пароли не совпадают\n"},
		{"same file", "encrypt", in, in, nil, 0, "входной и выходной файлы совпадают\n"},
		{"large plaintext", "encrypt", large, out, nil, 0, "файл слишком большой (максимум 1 ГиБ)\n"},
		{"large container", "decrypt", large, out, nil, 0, "файл слишком большой (максимум 1 ГиБ)\n"},
		{"format before password", "decrypt", in, out, nil, 0, "неверный формат файла\n"},
		{"read error", "encrypt", filepath.Join(dir, "missing"), out, nil, 0, "не удалось прочитать файл: "},
		{"write error", "encrypt", in, filepath.Join(dir, "missing", "out"), [][]byte{[]byte("pw"), []byte("pw")}, 2, "не удалось записать файл: "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertRun(t, []string{tc.command, "-in", tc.in, "-out", tc.out},
				&testReader{passwords: tc.password}, 1, "ошибка: "+tc.msg, tc.calls)
			if tc.out != in {
				if _, err := os.Lstat(tc.out); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("unexpected output state: %v", err)
				}
			}
		})
	}
	assertRun(t, []string{"encrypt", "-in", in, "-out", out}, nil, 1, "ошибка: ввод пароля требует терминал\n", 0)
}

func TestCLIUsage(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"encrypt", "-h"}, {"decrypt", "-h"}} {
		assertRun(t, args, &testReader{}, 0, "Использование: ", 0)
	}
	for _, args := range [][]string{nil, {"unknown"}, {"encrypt"}, {"decrypt", "-other"}, {"encrypt", "-in", "a", "-out", "b", "extra"}} {
		assertRun(t, args, &testReader{}, 2, "ошибка: ", 0)
	}
}

func TestCLIOutputAppearsDuringPasswordPrompt(t *testing.T) {
	dir := t.TempDir()
	in, out := filepath.Join(dir, "in"), filepath.Join(dir, "out")
	if err := os.WriteFile(in, []byte("input"), 0600); err != nil {
		t.Fatal(err)
	}
	r := &testReader{passwords: [][]byte{[]byte("pw"), []byte("pw")}}
	r.onRead = func() {
		if r.calls == 1 {
			if err := os.WriteFile(out, []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	assertRun(t, []string{"encrypt", "-in", in, "-out", out}, r, 1,
		"ошибка: выходной файл уже существует (используйте -force)\n", 2)
	if got, err := os.ReadFile(out); err != nil || string(got) != "keep" {
		t.Fatalf("newly created output = %q, %v", got, err)
	}
}
