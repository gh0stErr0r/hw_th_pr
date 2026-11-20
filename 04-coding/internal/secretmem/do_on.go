//go:build goexperiment.runtimesecret

package secretmem

import "runtime/secret"

// Do передаёт f экспериментальному runtime/secret. На поддерживаемых
// платформах среда Go очищает временную память, использованную f.
func Do(f func()) {
	secret.Do(f)
}
