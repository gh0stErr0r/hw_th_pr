//go:build !goexperiment.runtimesecret

package secretmem

// Do вызывает f напрямую в обычной сборке. Явная очистка буферов
// выполняется в коде, который владеет секретными данными.
func Do(f func()) {
	f()
}
