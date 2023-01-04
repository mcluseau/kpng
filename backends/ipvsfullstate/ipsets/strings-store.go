package ipsets

import "sigs.k8s.io/kpng/client/diffstore"

type stringLeaf = diffstore.AnyLeaf[string]
type stringStore = diffstore.Store[string, *stringLeaf]

func newStringStore() *stringStore {
	return diffstore.NewAnyStore[string](func(a, b string) bool { return a == b })
}
