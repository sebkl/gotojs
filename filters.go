package gotojs

import(
	"log"
	"reflect"
)

// AutoInjectF returns a filter function whose parameters will be automatically injected based
// on their types. Besides explicitly announced Injections by SetupInjection, both the *Binding as 
// well as the full Injections container will be injected.
func AutoInjectF(f interface{}) Filter {
	fv := reflect.ValueOf(f)
	ft := reflect.TypeOf(f)
	if fv.Kind() != reflect.Func {
		log.Fatalf("Parameter is not a function. %s/%s",fv.Kind().String(),reflect.Func.String())
	}

	if ft.NumOut() != 1 || ft.Out(0) != reflect.TypeOf(true) {
		log.Fatal("Return parameter is not of type bool.")
	}

	return func (b *Binding,injo Injections) bool {
		ac := ft.NumIn()
		av := make([]reflect.Value,ac)
		inj := MergeInjections(injo,NewI(b,injo))

		for x := 0; x < ac; x++ {
			at := fv.Type().In(x)
			if v := inj[at]; v != nil {
				// Type found in Injections container
				av[x] = reflect.ValueOf(v)
			} else {
				// Not found
				log.Printf("Cannot fulfill injection uring AutoInject for type: \"%s\". Aborting filter chain.",at)
				return false
			}
		}

		return fv.Call(av)[0].Bool()
	}
}
