package gotojs

import (
	"fmt"
	"log"
	"net/http"
	"reflect"
)

// remoteBinder is a function type that will be invoked for a remote binding.
type remoteBinder func(c *HTTPContext, s *Session, i []interface{}) interface{}

// bindingInterface declare binding specific methods.
type bindingInterface interface {
	invokeI(Injections, []interface{}) interface{}
	argCount() int
	argType(int) reflect.Type
	signature([]reflect.Type) string
	base() *binding
}

// binding declares binding type independent attributes
type binding struct {
	elemName      string
	interfaceName string
	injections    map[int]reflect.Type
	singletons    Injections
	filters       []Filter
	container     *Container
}

type functionBinding struct {
	binding
	i interface{}
}

type attributeBinding struct {
	binding
	elemNum int
	i       interface{}
}

type methodBinding struct {
	binding
	elemNum int
	i       interface{}
}

type remoteBinding struct {
	binding
	remoteSignature string
	i               remoteBinder
}

type handlerBinding struct {
	binding
	handler http.HandlerFunc
	i       http.Handler
}

// :-D
func (b *binding) base() *binding {
	return b
}

//InterfaceName returns the name of the interface this binding is assigned to.
func (b *binding) InterfaceName() string {
	return b.interfaceName
}

//MEthodName returns the method name of this binding.
func (b *binding) MethodName() string {
	return b.elemName
}

// ValidationString generate a string that represents the signature of a method or function. It
// is used to perform a runtime validation when calling a JS proxy method.
func (b Binding) ValidationString() (ret string) {
	return b.signature(parameterTypeArray(b, false))
}
func (b *binding) signature(a []reflect.Type) (ret string) {
	ret = ""
	//TODO: check direct Mapping
	for _, v := range a {
		ret += string(kindMapping[v.Kind()])
	}
	return
}
func (b *remoteBinding) signature(a []reflect.Type) string {
	return b.remoteSignature
}
func (b *handlerBinding) signature(a []reflect.Type) string {
	return ""
}

// argType is an internally used method that returns the type of i-th binding parameter.
// The index i does not respect the receiver but includes the injected parameters.
func (b *attributeBinding) argType(i int) reflect.Type {
	return reflect.TypeOf(b.i)
}
func (b *remoteBinding) argType(i int) reflect.Type {
	return reflect.TypeOf(b.i).In(i)
}
func (b *methodBinding) argType(i int) reflect.Type {
	t := reflect.TypeOf(b.i)
	return t.Method(b.elemNum).Type.In(i + 1)
}
func (b *functionBinding) argType(i int) reflect.Type {
	return reflect.TypeOf(b.i).In(i)
}
func (b *handlerBinding) argType(i int) reflect.Type {
	return reflect.TypeOf("")
}

// argCount is an internally Function that returns the effective amount of parameters
// this binding needs. This includes injections and excludes the receiver.
func (b *attributeBinding) argCount() int {
	return 0
}
func (b *methodBinding) argCount() int {
	return reflect.TypeOf(b.i).Method(b.elemNum).Func.Type().NumIn() - 1
}
func (b *functionBinding) argCount() int {
	return reflect.TypeOf(b.i).NumIn()
}
func (b *remoteBinding) argCount() int {
	//TODO: check if this is correct ? Statically return 3 ?
	return reflect.TypeOf(b.i).NumIn()
}
func (b *handlerBinding) argCount() int {
	return 0
}

//NewRemoteBinding creates a new remote binding. All details are kept in the closure of the given proxy function.
func (b *Container) newRemoteBinding(i remoteBinder, sig, in, mn string) (ret Binding) {
	ret = Binding{bindingInterface: &remoteBinding{
		binding:         *b.newBinding(in, mn),
		remoteSignature: sig,
		i:               i,
	}}
	ret.addGlobalInjections()
	b.bindingContainer[in][mn] = ret
	return
}

// newBinding creates a new binding object that is associated with the given container.
// All existing global Injections will be added to this binding.
func (b *Container) newBinding(in, mn string) *binding {
	_, found := b.Binding(in, mn)
	if found {
		log.Printf("Binding \"%s\" already exposed for interface \"%s\". Overwriting.", mn, in)
	} else {
		if _, f := b.bindingContainer[in]; !f {
			im := make(map[string]Binding)
			b.bindingContainer[in] = im
		}

		if _, f := b.bindingContainer[in][mn]; !f {
			r := Binding{}
			b.bindingContainer[in][mn] = r
		}
	}
	p := &binding{
		elemName:      mn,
		interfaceName: in,
		container:     b,
		singletons:    make(Injections),
		injections:    make(map[int]reflect.Type)}
	return p
}

func (b *Container) newHandlerBinding(handler http.Handler, in, mn string) (ret Binding) {
	ret = Binding{
		bindingInterface: &handlerBinding{
			binding: *b.newBinding(in, mn),
			handler: nil,
			i:       handler,
		}}
	// No Injections needed here
	b.bindingContainer[in][mn] = ret
	return
}

func (b *Container) newHandlerFuncBinding(handler http.HandlerFunc, in, mn string) (ret Binding) {
	ret = Binding{
		bindingInterface: &handlerBinding{
			binding: *b.newBinding(in, mn),
			handler: handler,
			i:       nil,
		}}
	// No Injections needed here
	b.bindingContainer[in][mn] = ret
	return
}

//newMethodBinding creates a new method binding with the given interface and method name, whereby
// x specifies the x-th method of the given object.
func (b *Container) newMethodBinding(i interface{}, x int, in, mn string) (ret Binding) {
	ret = Binding{bindingInterface: &methodBinding{
		binding: *b.newBinding(in, mn),
		elemNum: x,
		i:       i,
	}}
	ret.addGlobalInjections()
	b.bindingContainer[in][mn] = ret
	return
}

//newFunctionBinding creates a new function binding with the given interface and method name.
func (b *Container) newFunctionBinding(i interface{}, in, mn string) (ret Binding) {
	ret = Binding{bindingInterface: &functionBinding{
		binding: *b.newBinding(in, mn),
		i:       i,
	}}
	ret.addGlobalInjections()
	b.bindingContainer[in][mn] = ret
	return
}

//newAttributeBinding creates a new attribute getter binding whereby x specifies the x-th
// field of there referenced object.
func (b *Container) newAttributeBinding(i interface{}, x int, in, mn string) (ret Binding) {
	ret = Binding{bindingInterface: &attributeBinding{
		binding: *b.newBinding(in, mn),
		elemNum: x,
		i:       i,
	}}
	ret.addGlobalInjections()
	b.bindingContainer[in][mn] = ret
	return
}

//invokeI is an internally used method to invoke a proxy type binding
// using the given injection objects and binding parameters
func (b *methodBinding) invokeI(inj Injections, args []interface{}) interface{} {
	val := reflect.ValueOf(b.i)

	//Sanity check whether binding object is not of kind function.
	if val.Kind() == reflect.Func {
		panic(fmt.Errorf("MethodBinding for \"%s.%s\" does not bind an object.", b.interfaceName, b.elemName))
	}

	av := callValuesI(b, inj, args)

	// Prepend the receiver
	cav := make([]reflect.Value, len(av)+1)
	cav[0] = val
	for i, v := range av {
		cav[i+1] = v
	}

	meth := reflect.TypeOf(b.i).Method(b.elemNum).Func
	return convertReturnValue(meth.Call(cav)) // Call with receiver and consider injected objects.
}

//invokeI is an internally used method to invoke a function type binding
// using the given injection objects and binding parameters
func (b *functionBinding) invokeI(inj Injections, args []interface{}) interface{} {
	val := reflect.ValueOf(b.i)

	// Sanity check whether binding object is actually of kind function
	if val.Kind() != reflect.Func {
		panic(fmt.Errorf("FunctionBinding for \"%s.%s\"  does not bind a function.", b.interfaceName, b.elemName))
	}
	meth := reflect.ValueOf(b.i)

	av := callValuesI(b, inj, args)
	return convertReturnValue(meth.Call(av)) // Call with receiver and consider injected objects.
}

//invokeI is an internally used method to invoke a proxy type binding
// using the given injection objects and binding parameters
func (b *remoteBinding) invokeI(inj Injections, args []interface{}) interface{} {
	meth := reflect.ValueOf(b.i)

	//wrap parameters in an []interface{} array
	aa := make([]interface{}, 1)
	aa[0] = args

	av := callValuesI(b, inj, aa) //Important: use args as array parameter (NOT exploded !)
	return convertReturnValue(meth.Call(av))
}

//invokeI returns the value of the attribute this binding is referring.
func (b *attributeBinding) invokeI(inj Injections, args []interface{}) interface{} {
	return reflect.ValueOf(b.i).Elem().Field(b.elemNum).Interface()
}

//invokeI returns invokes the associated Handler
func (b *handlerBinding) invokeI(inj Injections, args []interface{}) interface{} {
	o, ok := inj[reflect.TypeOf(&HTTPContext{})]
	if !ok {
		panic(fmt.Errorf("No web context found: %s", inj))
	}

	httpContext, ok := o.(*HTTPContext)
	if !ok {
		panic(fmt.Errorf("No http context found."))
	}

	if b.i != nil {
		b.i.ServeHTTP(httpContext.Response, httpContext.Request)
		return nil
	}

	if b.handler != nil {
		b.handler(httpContext.Response, httpContext.Request)
		return nil
	}

	return nil
}
