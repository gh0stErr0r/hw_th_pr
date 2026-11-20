"""Функциональный прогон готового genc через командную строку."""
import argparse
import json
import os
from pathlib import Path
import pty
import select
import stat
import subprocess
import tempfile
import termios
import time


def run(binary, args, passwords=(), terminal=True):
    command = [binary, *map(str, args)]
    if not terminal:
        result = subprocess.run(command, input=b"", capture_output=True, timeout=20)
        assert not result.stdout, result.stdout
        return result.returncode, result.stderr.decode()

    master, slave = pty.openpty()
    transcript = bytearray()
    try:
        with subprocess.Popen(command, stdin=slave, stdout=subprocess.PIPE, stderr=slave) as process:
            def receive():
                if select.select([master], [], [], 0.05)[0]:
                    transcript.extend(os.read(master, 65536))

            try:
                offset = 0
                prompts = ("Пароль: ", "Повторите пароль: ")
                for prompt, password in zip(prompts, passwords):
                    deadline = time.monotonic() + 20
                    encoded_prompt = prompt.encode()
                    while True:
                        position = transcript.find(encoded_prompt, offset)
                        echo = termios.tcgetattr(slave)[3] & termios.ECHO
                        if position >= 0 and not echo:
                            offset = position + len(encoded_prompt)
                            break
                        assert process.poll() is None, transcript.decode()
                        assert time.monotonic() < deadline, transcript.decode()
                        receive()
                    os.write(master, password + b"\n")
                status = process.wait(timeout=20)
                while select.select([master], [], [], 0)[0]:
                    receive()
                assert not process.stdout.read(), transcript
                assert transcript.count(prompts[0].encode()) == bool(passwords), transcript
                assert transcript.count(prompts[1].encode()) == (len(passwords) == 2), transcript
                assert all(not password or password not in transcript for password in passwords)
                return status, transcript.decode()
            finally:
                if process.poll() is None:
                    process.kill()
    finally:
        os.close(slave)
        os.close(master)


def suite(binary, root):
    password = b"genc-functional-check"
    wrong = b"wrong-functional-pass"
    source = root / "input.txt"
    encrypted = root / "encrypted.genc"
    restored = root / "restored.txt"
    source_data = "Привет, мир!\nЁж.\n".encode()
    source.write_bytes(source_data)

    def invoke(action, src, dst, passwords=(), force=False, terminal=True):
        flags = ["-force"] if force else []
        return run(binary, [action, "-in", src, "-out", dst, *flags], passwords, terminal)

    def expect(outcome, status, message):
        assert outcome[0] == status and message in outcome[1], outcome

    expect(invoke("encrypt", source, encrypted, (password, password)), 0, "Готово")
    assert source.read_bytes() == source_data
    assert encrypted.stat().st_size == len(source_data) + 55
    assert stat.S_IMODE(encrypted.stat().st_mode) == 0o600
    expect(invoke("decrypt", encrypted, restored, (password,)), 0, "Готово")
    assert restored.read_bytes() == source_data
    yield "F1", "0 при шифровании и расшифровании, байты совпали, источник цел, разница 55 байт, ввода на экране нет"

    invalid_password = root / "invalid-password.txt"
    expect(invoke("decrypt", encrypted, invalid_password, (wrong,)), 1, "неверный пароль или файл повреждён")
    assert not invalid_password.exists()
    expect(invoke("decrypt", encrypted, restored, (wrong,), force=True), 1, "неверный пароль или файл повреждён")
    assert restored.read_bytes() == source_data
    yield "F2", "1, ошибка аутентификации, новый файл отсутствует, существующий сохранён"

    container = encrypted.read_bytes()
    damaged = root / "damaged.genc"
    damaged.write_bytes(container[:-1] + bytes([container[-1] ^ 1]))
    recovered = root / "recovered.txt"
    expect(invoke("decrypt", damaged, recovered, (password,)), 1, "неверный пароль или файл повреждён")
    assert not recovered.exists()
    yield "F3", "1, ошибка аутентификации, выходного файла нет"

    damaged.write_bytes(container[:54])
    expect(invoke("decrypt", damaged, recovered), 1, "неверный формат файла")
    assert not recovered.exists()
    yield "F4", "1, ошибка формата до запроса пароля, выходного файла нет"

    expect(invoke("encrypt", source, recovered, (password, wrong)), 1, "пароли не совпадают")
    assert not recovered.exists()
    expect(invoke("encrypt", source, recovered, (b"",)), 1, "пароль не может быть пустым")
    assert not recovered.exists()
    expect(invoke("encrypt", source, recovered, terminal=False), 1, "ввод пароля требует терминал")
    assert not recovered.exists()
    yield "F5", "1 для каждого варианта, три ожидаемых сообщения, выходного файла нет"

    expect(invoke("encrypt", source, encrypted), 1, "выходной файл уже существует")
    assert encrypted.read_bytes() == container
    expect(invoke("encrypt", source, encrypted, (password, password), force=True), 0, "Готово")
    assert encrypted.read_bytes() != container
    expect(invoke("encrypt", source, source, force=True), 1, "входной и выходной файлы совпадают")
    assert source.read_bytes() == source_data
    yield "F6", "1 без -force, 0 с -force, 1 при совпадении путей, источник цел"

    empty = root / "empty.txt"
    empty.touch()
    expect(invoke("encrypt", empty, encrypted, (password, password), force=True), 0, "Готово")
    assert encrypted.stat().st_size == 55
    expect(invoke("decrypt", encrypted, restored, (password,), force=True), 0, "Готово")
    assert restored.read_bytes() == b""
    yield "F7", "0 для обеих команд, контейнер 55 байт, восстановлено 0 байт"

    expect(run(binary, ["-h"], terminal=False), 0, "Использование")
    expect(run(binary, ["decrypt", "-unknown"], terminal=False), 2, "неверные аргументы")
    yield "F8", "0 для справки, 2 для неизвестного флага"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path)
    parser.add_argument("--json", type=Path, help="Записать фактические результаты для отчёта")
    args = parser.parse_args()
    results = []
    with tempfile.TemporaryDirectory() as directory:
        for case, observed in suite(args.binary.resolve(), Path(directory)):
            results.append({"case": case, "observed": observed})
            print(f"{case} PASS — {observed}")
    if args.json:
        args.json.write_text(json.dumps(results, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
