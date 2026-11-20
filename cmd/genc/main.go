package main

import (
	"crypto/rand"
	"os"

	"genc/internal/cli"
	"genc/internal/core"
)

// main связывает CLI с терминалом, криптографическим генератором
// случайных чисел и штатными параметрами Argon2id.
func main() {
	os.Exit(cli.Run(os.Args[1:], cli.Env{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Random: rand.Reader,
		Params: core.DefaultParams,
	}))
}
