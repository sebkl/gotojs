package gotojs

import (
	"reflect"
	"log"
	"strings"
	"regexp"
	"fmt"
	"strconv"
	"encoding/json"
	"time"
)

const (
	DefaultInterfaceName string = "main"
	DefaultFunctionName string = "f"
	DefaultInternalInterfaceName string = "gotojs"
)

// Filter is a filter function that receives, the binding which is currently being invoked and the 
// Injection objects of the environment. It returns true if the call and filter chain may proceed.
// If it returns false, the request has been prohabited or already answered and neither a further
// filter nor the real method call will be invoked.
type Filter func (Binding,Injections) bool

// remoteBinder is a function type that will be invoked for a remote binding.
type remoteBinder func (c *HTTPContext,s *Session, i []interface{}) interface{}

// bindingInterface declare binding specific methods.
type bindingInterface interface {
	invokeI(Injections,[]interface{}) interface{}
	argCount() int
	argType(int) reflect.Type
	signature([]reflect.Type) string
	base() *binding
}

// backend consits of the binding container as well as some administrative attributes like revision and injection references.
type backend struct {
	bindingContainer
	globalInjections Injections
	revision uint64
	converterRegistry map[reflect.Type]Converter
}

// binding declares binding type independent attributes
type binding struct {
	elemName string
	interfaceName string
	injections map[int]reflect.Type
	singletons Injections
	filters []Filter
	backend *backend
}

type functionBinding struct {
	binding
	i interface{}
}

type attributeBinding struct {
	binding
	elemNum int
	i interface{}
}

type methodBinding struct {
	binding
	elemNum int
	i interface{}
}

type remoteBinding struct {
	binding
	remoteSignature string
	i remoteBinder
}

// Binding is a concrete method binding. It maps a interface and method name to a go object's method.
// The receiver is stored and in case of a method invocation, the original receiver will be 
// passed while the method is called. Besides this the a holds the information, which filter are
// active, which parameter needs to be injected by the InvokeI call or need to be registered as singletons..
type Binding struct {
	bindingInterface
}

//Bindings is a list of concrete method bindings.
type Bindings []Binding

// Interface represents an interface binding which consists of a set of methods or functions.
type Interface map[string]Binding

// Interfaces represents a list or slice of Interfaces including all its bindings.
type Interfaces []Interface

// bindingContainer represents a container which consists of a set of interface and its bindings
type bindingContainer map[string]Interface

// Injections is a container of injection objects sorted by their type.
type Injections map[reflect.Type]interface{}


// Mapping of kind to char for method/function signature validation string.
var kindMapping = map[reflect.Kind]byte{
	reflect.Bool: 'i',
	reflect.Int: 'i',
	reflect.Int8: 'i',
	reflect.Int16: 'i',
	reflect.Int32: 'i',
	reflect.Int64: 'i',
	reflect.Uint: 'i',
	reflect.Uint8: 'i',
	reflect.Uint16: 'i',
	reflect.Uint32: 'i',
	reflect.Uint64: 'i',
	reflect.Uintptr: 'i',
	reflect.Float32: 'f',
	reflect.Float64: 'f',
	reflect.Complex64: '_',
	reflect.Complex128: '_',
	reflect.Array: 'a',
	reflect.Chan: '_',
	reflect.Func: '_',
	reflect.Interface: 'o',
	reflect.Map: 'm',
	reflect.Ptr: 'o',
	reflect.Slice: 'a',
	reflect.String: 's',
	reflect.Struct: 'o',
	reflect.UnsafePointer: 'i' }

//newBackend is a constructor for the bindingContainer data structure.
func newBackend() (backend) {
	ret := backend{
		bindingContainer: make(bindingContainer),
		globalInjections: make(Injections),
		converterRegistry: make(map[reflect.Type]Converter),
	}

	ret.RegisterConverter("",StringConverter)
	ret.RegisterConverter(time.Now(),TimeConverter)

	return ret
}

//RegisterConverter defines the given converter function for the assigned type.
func (b backend) RegisterConverter(t interface{}, c Converter) {
	log.Printf("Registering converter for type %s",reflect.TypeOf(t))
	b.converterRegistry[reflect.TypeOf(t)] = c
}

// AddInjection adds a singleton injection object for the given binding and declares its type
// as injection object. It can calso be used to declare a type and in the same step define a 
// default singleton which will be injected in case no further object of this type will is 
// provided for InvokeI calls.
func (b Binding) AddInjection(i interface{}) Binding{
	bb:= b.base()
	it:= reflect.TypeOf(i)
	bb.singletons[it] = i

	pta := parameterTypeArray(b,true)

	for ii,t := range pta {
		if t == it {
			bb.injections[ii] = it
		}
	}

	return  b
}

// :-D
func (b *binding) base() *binding{
	return b
}

//InterfaceName returns the name of the interface this binding is assigned to.
func (b *binding) InterfaceName() string{
	return b.interfaceName
}

//MEthodName returns the method name of this binding.
func (b* binding) MethodName() string{
	return b.elemName
}

//S method returns an one element array of this binding.
func (b Binding) S() (ret Bindings) {
	ret = make(Bindings,1)
	ret[0] = b
	return
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

// parameterTypeArray is an internally used method to get an
// array of the method parameter types.
func parameterTypeArray(b bindingInterface ,includeInjections bool) []reflect.Type {
	argCount := b.argCount()
	ret := make([]reflect.Type,argCount)

	ri := 0 //result arrach index
	for n:=0;n < argCount;n++ {
		if _,found := b.base().injections[n]; !found || includeInjections {
			ret[ri] = b.argType(n)
			ri++
		}
	}
	return ret[:ri]
}

// ValidationString generate a string that represents the signature of a method or function. It
// is used to perform a runtime validation when calling a JS proxy method.
func (b Binding) ValidationString() (ret string){
	return b.signature(parameterTypeArray(b,false))
}
func (b *binding) signature(a []reflect.Type) (ret string) {
	ret = ""
	//TODO: check direct Mapping
	for _,v := range a {
		ret += string(kindMapping[v.Kind()])
	}
	return
}
func (b *remoteBinding) signature(a []reflect.Type) string{
	return b.remoteSignature
}

func receivesBinaryContent(b bindingInterface) bool {
	return countParameterType(b,&BinaryContent{}) > 0
}

//Signature returns a tring representation of the binding signature.
func (b Binding) Signature() (ret string) {
	ret = b.ValidationString()
	if receivesBinaryContent(b) {
		ret = ":" + ret
	}
	return
}

// countParameterType counts the amount of paremter this method
// accepts. Usually used to determine whether it takes a certain
// argument type.
func countParameterType(b bindingInterface, i interface{}) (ret int) {
	t := reflect.TypeOf(i)
	ret = 0
	a := parameterTypeArray(b,true)
	for _,v := range a {
		if v == t {
			ret++
		}
	}
	return
}

// AddInjection is a convenience method to AddInjection of type Binding.
func (bs Bindings) AddInjection(i interface{}) Bindings {
	for _,b := range bs {
		b.AddInjection(i)
	}
	return bs
}

// SetupGlobaleIjection declares a type that will always be injected.
// This applies for both existing bindings as well as new bindings.
func (b backend) SetupGlobalInjection(i interface{}) {
	t := reflect.TypeOf(i)
	b.globalInjections[t] = i
	b.Bindings().AddInjection(i) // Add Injection for all existing bindings.
}

// Match filters the list of Bindings and only returns those bindings whose
// name matches the given regex pattern.
// The interface is alwas placed in front of the method name seperated
// by a ".".
func (bs Bindings) Match(pattern string) Bindings {
	re,err := regexp.Compile(pattern)

	if err != nil {
		log.Printf("Compilation of regexp patter \"%s\" failed: %s",pattern,err.Error())
		return make(Bindings,0)
	}

	ret := make(Bindings,len(bs))
	i := 0
	for _,b := range bs {
		n := b.Name()
		if re.MatchString(n) {
			ret[i] = b
			i++
		}
	}
	return ret[0:i]
}

// addGlobalInjection adds the global injection types to the given binding.
func (b Binding) addGlobalInjections() {
	for _,v := range b.base().backend.globalInjections {
		b.AddInjection(v)
	}
}

// newBinding creates a new binding object that is associated with the given backend.
// All existing global Injections will be added to this binding.
func (b *backend) newBinding(in,mn string) (*binding) {
	_,found := b.Binding(in,mn)
	if found {
		log.Printf("Binding \"%s\" already exposed for interface \"%s\". Overwriting.",mn,in)
	} else {
		if _,f := b.bindingContainer[in]; !f {
			im := make(map[string]Binding)
			b.bindingContainer[in] = im
		}

		if _,f := b.bindingContainer[in][mn]; !f {
			r := Binding{}
			b.bindingContainer[in][mn] = r
		}
	}
	p := &binding{
		elemName: mn,
		interfaceName: in,
		backend: b,
		singletons: make(Injections),
		injections: make(map[int]reflect.Type)}
	return p
}

//NewRemoteBinding creates a new remote binding. All details are kept in the closure of the given proxy function.
func (b *backend) newRemoteBinding(i remoteBinder,sig,in,mn string) (ret Binding) {
	ret = Binding{bindingInterface: &remoteBinding{
		binding: *b.newBinding(in,mn),
		remoteSignature: sig,
		i: i,
	}}
	ret.addGlobalInjections()
	b.bindingContainer[in][mn] = ret
	return
}

//NewMethodBinding creates a new method binding with the given interface and method name, whereby
// x specifies the x-th method of the given object.
func (b *backend) newMethodBinding(i interface{},x int,in,mn string) (ret Binding) {
	ret = Binding{bindingInterface: &methodBinding{
		binding: *b.newBinding(in,mn),
		elemNum: x,
		i: i,
	}}
	ret.addGlobalInjections()
	b.bindingContainer[in][mn] = ret
	return
}

//NewFunctionBinding creates a new function binding with the given interface and method name.
func (b *backend) newFunctionBinding(i interface{},in,mn string) (ret Binding) {
	ret =  Binding{bindingInterface: &functionBinding{
		binding: *b.newBinding(in,mn),
		i: i,
	}}
	ret.addGlobalInjections()
	b.bindingContainer[in][mn] = ret
	return
}

//NewAttributeBinding creates a new attribute getter binding whereby x specifies the x-th
// field of there referenced object.
func (b *backend) newAttributeBinding(i interface{},x int,in,mn string) (ret Binding) {
	ret = Binding{bindingInterface: &attributeBinding{
		binding: *b.newBinding(in,mn),
		elemNum: x,
		i: i,
	}}
	ret.addGlobalInjections()
	b.bindingContainer[in][mn] = ret
	return
}

// Expose an entire interface. All methods of the given interface will be exposed. THe name of the
// exposed interface is either taken from type name or could be specified as additional name parameter.
func (b *backend) ExposeInterface(i interface{},name ...string) (ret Bindings) {
	return b.ExposeMethods(i,"",name...)
}

// ExposeMethod is a convenience method to ExposeMethods for exposing a single method of an interface.
func (b *backend) ExposeMethod(i interface{}, name string,target_name ...string) (ret Bindings) {
	return b.ExposeMethods(i,"^" + name + "$",target_name...)
}

// resolvePointer ensures that the returned value is a pointer to the 
// origin object.
func resolvePointer(i interface{}) interface{}{
	// Try to get object/receiver name from interface.
	k:=reflect.ValueOf(i).Type().Kind();

	// If interface type is not a pointer, take it.
	if k != reflect.Ptr{
		ptr := reflect.New(reflect.TypeOf(i))
		temp := ptr.Elem()
		temp.Set(reflect.ValueOf(i))
		i = ptr.Interface()
	}

	return i
}

// findInterfaceName tries to retrieve the interface name from the named interface object.
// If the object is unnamed, the default name will be returned.
func findInterfaceName(i interface{}) (in string){
	in = DefaultFunctionName

	// TODO: Fix this crap.
	name := reflect.ValueOf(i).Type().String()
	elems := strings.Split(name,".")
	iname :=elems[len(elems)-1]
	if len(iname) > 0 {
		in = iname
	}
	return
}

//ExposeAllAttributes is a convenience method to ExposeAttribute which exposes
// all public attributes of the given object.
func (b* backend) ExposeAllAttributes(i interface{}, name... string) (Bindings) {
	return b.ExposeAttributes(i,"",name...)
}

// ExposeAttributes exposes getter function to all public attributes of the given object.
func (b* backend) ExposeAttributes(i interface{}, pattern string, name ...string) (ret Bindings) {
	i = resolvePointer(i)

	// Find name either by args or by interface type. 
	var in string
	if len(name) > 0 {
		in = name[0]
	} else {
		in = findInterfaceName(i)
	}

	t := reflect.TypeOf(i)
	c := 0
	ret = make(Bindings,t.Elem().NumField())
	for x:=0; x < t.Elem().NumField(); x++ {
		an := t.Elem().Field(x).Name

		if f,_ := regexp.Match(pattern,[]byte(an)); len(pattern) == 0 || f {
			pm := b.newAttributeBinding(i,x,in,an)
			ret[c] = pm
			c++
		}
	}

	if c > 0 {
		b.revision++
	}
	return ret[:c]
}

// ExposeMethods exposes the methods of a interface that do match the given regex pattern.
func (b *backend) ExposeMethods(i interface{},pattern string, name ...string) (ret Bindings) {
	i = resolvePointer(i)

	// Find name either by args or by interface type. 
	var n string
	if len(name) > 0 {
		n = name[0]
	} else {
		n = findInterfaceName(i)
	}

	t := reflect.TypeOf(i)

	c:= 0 // final amount of exposed methods.
	ret = make(Bindings,t.NumMethod())
	for x:=0;x < t.NumMethod();x++ {
		mt := t.Method(x)
		/* Sanity check on method signature: */
		if (mt.Type.NumOut() > 1) {
			/* Only up to one return argument is allowed. */
			log.Printf("Ignoring method \"%s\" due to invalid amount of return parameters. %d / %d",mt.Name,mt.Type.NumOut(),1);
			continue
		}

		mn := mt.Name
		/* Sanity check if method is Accessible: */
		if matched,_ := regexp.Match("^[A-Z]",[]byte(mn)); !matched {
			log.Printf("Ignoring internal method \"%s\".",mn)
			continue
		}

		// If a pattern is given, check the method name first.
		if  matched,_ := regexp.Match(pattern,[]byte(mn)); len(pattern) == 0 || matched {
			pm := b.newMethodBinding(i,x,n,mn)
			//Compile return slice
			ret[c] = pm
			c++;
		}

	}

	if c > 0 {
		b.revision++
	}
	return ret[:c]
}

// ExposeFunction exposes a single function. No receiver is required for this binding.
func (b *backend) ExposeFunction(f interface{}, name ...string) Bindings {
	v:= reflect.ValueOf(f)
	if v.Kind() != reflect.Func {
		panic(fmt.Errorf("Parameter is not a function. %s/%s",v.Kind().String(),reflect.Func.String()))
	}

	iname := DefaultInterfaceName
	fname := DefaultFunctionName
	l := len(name)
	if l > 0 {
		iname = name[0]
	}

	if l > 1 {
		fname = name[1]
	}

	//TODO: make clean and move to ExportFunction method of bindingContainer
	pm:= b.newFunctionBinding(f,iname,fname)
	b.revision++
	return pm.S()
}

///ExposeYourself exposes some administrative and discovery methods of the gotojs backend functionality.
func (b *backend) ExposeYourself(args ...string) (ret Bindings) {
	in := DefaultInternalInterfaceName
	if len(args) > 0 {
		in = args[0]
	}
	ret = make(Bindings,2)
	ret[0] = b.ExposeFunction(func (b *backend) map[string]string {
		bs := b.Bindings()
		ret := make(map[string]string)
		for _,b := range bs {
			ret[b.Name()] = b.ValidationString()
		}
		return ret
	},in,"Bindings").AddInjection(b)[0]

	ret[1] = b.ExposeFunction(func (b *backend) []string{
		return b.InterfaceNames()
	},in,"Interfaces").AddInjection(b)[0]
	return
}

// If sets a filter for the given binding. See type Filter for more information.
func (b Binding) If(f Filter) Binding {
	bb := b.base()
	bb.filters = append(bb.filters,f)
	return b
}

// ClearFilter removes all filters for the given binding.
func (b Binding) ClearFilter() Binding {
	b.base().filters = make([]Filter,0)
	return b
}

// Remove removes the binding from the binding container.
func (b Binding) Remove() {
	bb := b.base()
	bb.backend.RemoveBinding(bb.interfaceName,bb.elemName)
}

// If sets a filter for all given binding. See type Filter for more information.
func (bs Bindings) If(f Filter) Bindings{
	for _,b := range bs {
		b.If(f)
	}
	return bs
}

// ClearFilter remove all filters from the given bindings.
func (bs Bindings) ClearFilter() Bindings {
	for _,b := range bs {
		b.ClearFilter()
	}
	return bs
}

// Remove removes all bindings from their binding container.
func (bs Bindings) Remove() {
	for _,b := range bs {
		b.Remove()
	}
}

// Name returns the name of the given Binding. This is a concatenation of 
// the interface name, a "." seperator and the method name.
func (b Binding) Name() string {
	bb := b.base()
	return bb.interfaceName + "." + bb.elemName
}

// Binding searches a concrete binding by the given interface and method name.
func (b bindingContainer) Binding(i string, mn string) (ret Binding,found bool) {
	if _,found = b[i];!found {
		return
	}

	ret,found = b[i][mn]
	return
}

// Binding is a convenience method to retrieve the method of a interface. It panics if the method does not exist.
func (i Interface) Binding(n string) (r Binding) {
	r, found := i[n]
	if !found{
		panic(fmt.Errorf("Binding \"%s\" not found for interface.",n))
	}
	return
}

// Remove an entire interface from the binding container identified by the interface name.
func (b bindingContainer) RemoveInterface(i string) {
	delete(b,i)
}

// RemoveBinding removes a single method from the binding container identified by the interface and method name.
func (b bindingContainer) RemoveBinding(i,m string) {
	delete(b[i],m)
}

// InterfaceNames retrieves all bound interface names.
func (b bindingContainer) InterfaceNames() (keys []string) {
	//TODO: use Interfaces() here.
	keys = make([]string,len(b))
	i:=0
	for k,_ := range ( b ) {
		keys[i] = k
		i++
	}
	return
}

// Interfaces returns a list of all interface including its bindings.
func (b bindingContainer) Interfaces() (ret Interfaces) {
	ret = make(Interfaces,len(b))
	i:=0
	for _,v := range b {
		ret[i] = v
		i++
	}
	return
}

//Interface is a convenience method to retrieve an interface. It panics if the interface does not exist.
func (b bindingContainer) Interface(name string) (ret Interface) {
	ret,found := b[name]
	if !found {
		panic(fmt.Errorf("Interface \"%s\" does not exist.",name))
	}
	return
}

// BindingNames retreives all bound methods or functions names of the given interface.
func (b bindingContainer) BindingNames(i string) (methods []string) {
	mmap, found :=  b[i]
	if found {
		methods = make([]string,len(mmap))
		i:=0
		for k,_ := range mmap {
			methods[i] = k
			i++
		}
	}
	return
}

// Bindings returns all method bindings of the given container.
func (i Interface) Bindings() (ret Bindings) {
	ret = make(Bindings,len(i))
	c := 0
	for _,v := range i {
		ret[c] = v
		c++
	}
	return
}

// Bindings returns all method bindings of the given container.
func (b bindingContainer) Bindings() (ret Bindings) {
	is := b.Interfaces()
	for _,v := range is {
		bdns := v.Bindings()
		ret = append(ret,bdns...)
	}
	return
}

// Invoke a bound method or function of the given interface and method name.
func (b bindingContainer) Invoke(i,m string, args ...interface{}) interface{} {
	return b.InvokeI(i,m,nil,args...)
}

// InvokeI is a convenience method for invoking methods/function without prior discovery.
func (b bindingContainer) InvokeI(i,m string,inj Injections, args ...interface{}) interface{} {
	if r,found := b.Binding(i,m);found {
		return r.InvokeI(inj,args...)
	} else {
		panic(fmt.Errorf("Binding \"%s.%s\" not found.",i,m))
	}
	return nil
}

// Invoke a bound method or function with the given parameters.
func (b Binding) Invoke(args ...interface{}) (ret interface{}) {
	return b.InvokeI(nil,args...)
}

//Invoke the first bound method or function with the given parameters. 
func (r Bindings) Invoke(args ...interface{}) interface{} {
	return r.Invoke(args...)
}

//InvokeI the first bound method or function with the given parameters. 
func (r Bindings) InvokeI(inj Injections,args ...interface{}) interface{} {
	if len(r) > 0 {
		return r[0].InvokeI(inj,args...)
	} else {
		panic(fmt.Errorf("Empty Binding set. Invocation not possible."))
	}
}

// NewI constructs a new Injections container. Each parameter is part of the Injections container.
func NewI(args ...interface{}) (Injections) {
	ret := make(Injections)
	for _,v := range args {
		ret[reflect.TypeOf(v)] = v
	}
	return ret
}

// Add adds an injection object to the list of injections.
func (inj Injections) Add(i interface{}) {
	inj[reflect.TypeOf(i)] = i
}

// MergeInjections merges multiple Injetions. The later ones overwrite the prveiouse ones.
func MergeInjections(inja ...Injections) (ret Injections) {
	ret = make(Injections)
	for _,is := range inja{
		for it,io := range is {
			ret[it] = io
		}
	}
	return
}

//convertParameterValue tries to convert the value of the given parameter to the target type.
//More hi-level calls like strconv may be involved here.
func (b *backend) convertParameterValue(av reflect.Value,at reflect.Type) reflect.Value{
	tk := at.Kind()
	sk := av.Kind()

	//Check first if direct converter is registered.
	if converter,ok := b.converterRegistry[at]; ok {
		rv, err := converter(av.Interface(),at)
		if err == nil {
			return reflect.ValueOf(rv)
		}
		log.Printf("Converter failed for type '%s': %s",at,err)
	}

	switch sk {
		case reflect.String:
			skv := av.String()
			var err error
			var v interface{}
			switch tk {
				case reflect.Float64,reflect.Float32:
					v,err = strconv.ParseFloat(skv,64)
				case reflect.Int,reflect.Int8,reflect.Int16,reflect.Int32,reflect.Uint,reflect.Uint32,reflect.Uint8,reflect.Uint16:
					v,err = strconv.Atoi(skv)
				case reflect.Int64,reflect.Uint64:
					v,err = strconv.ParseInt(skv,10,64)
				case reflect.String:
					return av
				default:
					err = fmt.Errorf("No conversion type found for %s",tk)
			}
			if err == nil {
				av = reflect.ValueOf(v)
			} else {
				log.Printf("%s",err)
			}
		case reflect.Map, reflect.Struct:
			b,err := json.Marshal(av.Interface())
			if err != nil {
				panic(fmt.Errorf("Could not json encode parameter: %s (P1)",err))
			}

			rv := reflect.New(at)
			i := rv.Interface()
			err = json.Unmarshal(b,i)
			if err != nil {
				panic(fmt.Errorf("Could not json decode parameter: %s (P2)",err))
			}
			return reflect.Indirect(reflect.ValueOf(i))
		default:
			if tk == sk {
				return av
			}
			if tk == reflect.String {
				if iv,err := StringConverter(av.Interface(),reflect.TypeOf("")); err != nil {
					av = reflect.ValueOf(iv)
				}
			}
	}
	return av.Convert(at) // Try to convert automatically.
}

//callValuesI compiles the final function or methoad call parameters using the given injections
// with respect to the underlying binding type.
func callValuesI(b bindingInterface,inj Injections, args []interface{}) (ret []reflect.Value) {
	targetArgCount := b.argCount()
	ret = make([]reflect.Value,targetArgCount)
	ic := 0 // count of found injections
	iai := 0
	for ai := 0; ai < targetArgCount; ai++ {
		at := b.argType(ai)
		var av reflect.Value

		// Check if this parameter needs to be injected
		if _,ok:= b.base().injections[ai]; ok {
			if in,ok := inj[at]; ok { // a object of type at is provided by InvokeI call
				av = reflect.ValueOf(in).Convert(at)
			} else {
				panic(fmt.Errorf("Injection for type \"%s\" not found.",at))
			}

			ic++ // skip one input param
		} else {
			if iai >= len(args) {
				panic(fmt.Errorf("Invalid parameter count: %d/%d (%d injections applied)",iai,len(args),ic))
			}
			av = reflect.ValueOf(args[iai]) // Value object of the current parameter
			iai++ //procede to next input argument
		}

		// Assign final value to final call vectob.
		ret[ai] = b.base().backend.convertParameterValue(av,at)
	}

	if targetArgCount != (iai+ic) {
		panic(fmt.Errorf("Argument count does not match for method \"%s\". %d/%d. (%d injections applied)",b.base().elemName,targetArgCount,(iai+ic),ic))
	}

	return
}

func convertReturnValue(iret []reflect.Value) interface{} {
	/* Check if return argument exists or not. If not nil is returned as interface{} */
	switch len(iret) {
		case 0:
			return nil
		default:
			log.Printf("Too many return arguments %d/%d. Ignoring.",len(iret),1)
			fallthrough
		case 1:
			return iret[0].Interface() // Convert return argument to interface{}
	}
}

//invokeI is an internally used method to invoke a proxy type binding
// using the given unjections opjects and binding parameters
func (b *methodBinding) invokeI(inj Injections, args []interface{}) interface{} {
	val := reflect.ValueOf(b.i)

	//Sanity check whether binding object is not of kind function.
	if val.Kind() == reflect.Func {
		panic(fmt.Errorf("MethodBinding for \"%s.%s\" does not bind an object.",b.interfaceName,b.elemName))
	}

	av := callValuesI(b,inj,args)

	// Prepend the receiver
	cav := make([]reflect.Value,len(av)+1)
	cav[0] = val
	for i,v := range av {
		cav[i+1] = v
	}

	meth := reflect.TypeOf(b.i).Method(b.elemNum).Func
	return convertReturnValue(meth.Call(cav)) // Call with receiver and consider injected objects.
}

//invokeI is an internally used method to invoke a function type binding
// using the given unjections opjects and binding parameters
func (b *functionBinding) invokeI(inj Injections, args []interface{}) interface{} {
	val := reflect.ValueOf(b.i)

	// Sanity check whether binding object is actually of kind function
	if val.Kind() != reflect.Func {
		panic(fmt.Errorf("FunctionBinding for \"%s.%s\"  does not bind a function.",b.interfaceName,b.elemName))
	}
	meth := reflect.ValueOf(b.i)

	av := callValuesI(b,inj,args)
	return convertReturnValue(meth.Call(av)) // Call with receiver and consider injected objects.
}

//invokeI is an internally used method to invoke a proxy type binding
// using the given unjections opjects and binding parameters
func (b *remoteBinding) invokeI(inj Injections, args []interface{}) interface{} {
	meth := reflect.ValueOf(b.i)

	//wrap parameters in an []interface{} array
	aa := make([]interface{},1)
	aa[0] = args

	av := callValuesI(b,inj,aa) //Imporatent: use args as array parameter (NOT exploded !)
	return convertReturnValue(meth.Call(av))
}

//invokeI returns the value of the attribute this binding is refereing.
func (b *attributeBinding) invokeI(inj Injections, args []interface{}) interface{} {
	return reflect.ValueOf(b.i).Elem().Field(b.elemNum).Interface()
}

func (b Binding) InvokeI(ri Injections,args ...interface{}) interface{} {
	//Merge Injections. Runtime objects overwrite singletons.
	inj := MergeInjections(b.base().singletons,ri)

	//Execute filters
	for _,f := range b.base().filters {
		if !f(Binding{b},inj) {
			return nil
		}
	}

	return b.invokeI(inj,args)
}
