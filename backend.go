package gotojs

import (
	"reflect"
	"log"
	"strings"
	"regexp"
	"fmt"
)

const (
	DefaultInterfaceName string = "main"
	DefaultFunctionName string = "f"
	DefaultInternalInterfaceName string = "gotojs"
)

//Binding types
const (
	FunctionBinding = 1
	MethodBinding = 2
	AttributeBinding = 3
	RemoteBinding = 4
)

// Filter is a filter function that receives, the binding which is currently being invoked and the 
// Injection objects of the environment. It returns true if the call and filter chain may proceed.
// If it returns false, the request has been prohabited or already answered and neither a further
// filter nor the real method call will be invoked.
type Filter func (*Binding,Injections) bool

// Binding is a concrete method binding. It maps a interface and method name to a go object's method.
// The receiver is stored and in case of a method invocation, the original receiver will be 
// passed while the method is called. Besides this the a holds the information, which filter are
// active, which parameter needs to be injected by the InvokeI call or need to be registered as singletons..
// TODO: Refactor to interface with multiple umplementing types.
type Binding struct {
	t int
	elemName string
	interfaceName string
	elemNum int
	injections map[int]reflect.Type
	singletons Injections
	i interface{}
	filters []Filter
	backend *Backend
}

//Bindings is a list of concrete method bindings.
type Bindings []*Binding

// Interface represents an interface binding which consists of a set of methods or functions.
type Interface map[string]*Binding

// Interfaces represents a list or slice of Interfaces including all its bindings.
type Interfaces []Interface

// BindingContainer represents a container which consists of a set of interface and its bindings
type BindingContainer map[string]Interface

// Injections is a container of injection objects sorted by their type.
type Injections map[reflect.Type]interface{}

// Backend consits of the binding container as well as some administrative attributes like revision and injection references.
type Backend struct {
	BindingContainer
	globalInjections Injections
	revision uint64
}

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
	reflect.Ptr: 'i',
	reflect.Slice: 'a',
	reflect.String: 's',
	reflect.Struct: 'o',
	reflect.UnsafePointer: 'i' }

//NewBackend is a constructor for the BindingContainer data structure.
func NewBackend() (Backend) {
	return Backend{
		BindingContainer: make(BindingContainer),
		globalInjections: make(Injections)}
}

// AddInjection adds a singleton injection object for the given binding and declares its type
// as injection object. It can calso be used to declare a type and in the same step define a 
// default singleton which will be injected in case no further object of this type will is 
// provided for InvokeI calls.
func (m *Binding) AddInjection(i interface{}) *Binding{
	it:= reflect.TypeOf(i)
	m.singletons[it] = i

	pta := m.parameterTypeArray(true)

	for ii,t := range pta {
		if t == it {
			//log.Printf("Setting injecton (%s.%s) : idx: %d, type: %s",m.interfaceName,m.elemName,ii,it)
			m.injections[ii] = it
		}
	}
	return m
}

//S method returns an one element array of this binding.
func (m *Binding) S() (ret Bindings) {
	ret = make(Bindings,1)
	ret[0] = m
	return
}

// argType is an internally used method that returns the type of i-th binding parameter.
// The index i does not respect the receiver but includes the injected parameters.
func (r *Binding) argType(i int) reflect.Type {
	t := reflect.TypeOf(r.i)
	switch r.t {
		case MethodBinding:
			return t.Method(r.elemNum).Type.In(i + 1)
		case FunctionBinding,RemoteBinding:
			return t.In(i)
		case AttributeBinding:
			return t
		default:
			panic(fmt.Errorf("Unknown binding type: %d",r.t))
	}
}


// argCount is an internally Function that returns the effective amount of parameters
// this binding needs. This includes injections and excludes the receiver.
func (r* Binding) argCount() int {
	t := reflect.TypeOf(r.i)
	switch r.t {
		case MethodBinding:
			return t.Method(r.elemNum).Func.Type().NumIn() - 1
		case FunctionBinding,RemoteBinding:
			return t.NumIn()
		case AttributeBinding:
			return 0
		default:
			panic(fmt.Errorf("Unknown binding type: %d",r.t))
	}
}

// parameterTypeArray is an internally used method to get an
// array of the method parameter types.
func (r *Binding) parameterTypeArray(includeInjections bool) []reflect.Type {
	argCount := r.argCount()
	ret := make([]reflect.Type,argCount)

	ri := 0 //result arrach index
	for n:=0;n < argCount;n++ {
		if _,found := r.injections[n]; !found || includeInjections {
			ret[ri] = r.argType(n)
			ri++
		}
	}
	return ret[:ri]
}

// ValidationString generate a string that represents the signature of a method or function. It
// is used to perform a runtime validation when calling a JS proxy method.
func (r *Binding) ValidationString() (ret string){
	a := r.parameterTypeArray(false)
	for _,v := range a {
		ret += string(kindMapping[v.Kind()])
	}
	return
}


func (r *Binding) receivesBinaryContent() bool {
	return r.countParameterType(&BinaryContent{}) > 0
}

func (r *Binding) Signature() (ret string) {
	ret = r.ValidationString()
	if r.receivesBinaryContent() {
		ret = ":" + ret
	}
	return
}

// countParameterType counts the amount of paremter this method
// accepts. Usually used to determine whether it takes a certain
// argument type.
func (r *Binding) countParameterType(i interface{}) (ret int) {
	t := reflect.TypeOf(i)
	ret = 0
	a := r.parameterTypeArray(true)
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
func (b Backend) SetupGlobalInjection(i interface{}) {
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
func (b *Binding) addGlobalInjections() {
	for _,v := range b.backend.globalInjections {
		b.AddInjection(v)
	}
}

// newBinding creates a new binding object that is associated with the given backend.
// All existing global Injections will be added to this binding.
func (b *Backend) newBinding(i interface{},t,x int,in,mn string) (*Binding) {
	bind := b.Binding(in,mn)
	if bind != nil {
		log.Printf("Binding \"%s\" already exposed for interface \"%s\". Overwriting.",mn,in)
	} else {
		if _,f := b.BindingContainer[in]; !f {
			im := make(map[string]*Binding)
			b.BindingContainer[in] = im
		}

		if _,f := b.BindingContainer[in][mn]; !f {
			r := &Binding{}
			b.BindingContainer[in][mn] = r
		}
	}
	p := &Binding{
		i: i,
		t: t,
		elemNum: x,
		elemName: mn,
		interfaceName: in,
		backend: b,
		singletons: make(Injections),
		injections: make(map[int]reflect.Type)}
	p.addGlobalInjections()
	b.BindingContainer[in][mn] = p
	return p
}

//NewRemoteBinding creates a new remote binding. All details are kept in the closure of the given proxy function.
func (b *Backend) newRemoteBinding(i interface{},in,mn string) (*Binding) {
	return b.newBinding(i,RemoteBinding,-1,in,mn)
}

//NewMethodBinding creates a new method binding with the given interface and method name, whereby
// x specifies the x-th method of the given object.
func (b *Backend) newMethodBinding(i interface{},x int,in,mn string) (*Binding) {
	return b.newBinding(i,MethodBinding,x,in,mn)
}

//NewFunctionBinding creates a new function binding with the given interface and method name.
func (b *Backend) newFunctionBinding(i interface{},in,mn string) (*Binding) {
	return b.newBinding(i,FunctionBinding,-1,in,mn)
}

//NewAttributeBinding creates a new attribute getter binding whereby x specifies the x-th
// field of there referenced object.
func (b *Backend) newAttributeBinding(i interface{},x int,in,mn string) (*Binding) {
	return b.newBinding(i,AttributeBinding,x,in,mn)
}

// Expose an entire interface. All methods of the given interface will be exposed. THe name of the
// exposed interface is either taken from type name or could be specified as additional name parameter.
func (b *Backend) ExposeInterface(i interface{},name ...string) (ret Bindings) {
	return b.ExposeMethods(i,"",name...)
}

// ExposeMethod is a convenience method to ExposeMethods for exposing a single method of an interface.
func (b *Backend) ExposeMethod(i interface{}, name string,target_name ...string) (ret Bindings) {
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
func (b* Backend) ExposeAllAttributes(i interface{}, name... string) (Bindings) {
	return b.ExposeAttributes(i,"",name...)
}

// ExposeAttributes exposes getter function to all public attributes of the given object.
func (b* Backend) ExposeAttributes(i interface{}, pattern string, name ...string) (ret Bindings) {
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
func (b *Backend) ExposeMethods(i interface{},pattern string, name ...string) (ret Bindings) {
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
func (b *Backend) ExposeFunction(f interface{}, name ...string) Bindings {
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

	//TODO: make clean and move to ExportFunction method of BindingContainer
	pm:= b.newFunctionBinding(f,iname,fname)
	b.revision++
	return pm.S()
}

///ExposeYourself exposes some administrative and discovery methods of the gotojs backend functionality.
func (b *Backend) ExposeYourself(args ...string) (ret Bindings) {
	in := DefaultInternalInterfaceName
	if len(args) > 0 {
		in = args[0]
	}
	ret = make(Bindings,2)
	ret[0] = b.ExposeFunction(func (b *Backend) map[string]string {
		bs := b.Bindings()
		ret := make(map[string]string)
		for _,b := range bs {
			ret[b.Name()] = b.ValidationString()
		}
		return ret
	},in,"Bindings").AddInjection(b)[0]

	ret[1] = b.ExposeFunction(func (b *Backend) []string{
		return b.InterfaceNames()
	},in,"Interfaces").AddInjection(b)[0]
	return
}

// If sets a filter for the given binding. See type Filter for more information.
func (b *Binding) If(f Filter) *Binding {
	b.filters = append(b.filters,f)
	return b
}

// ClearFilter removes all filters for the given binding.
func (b* Binding) ClearFilter() *Binding {
	b.filters = make([]Filter,0)
	return b
}

// Remove removes the binding from the binding container.
func (b *Binding) Remove() {
	b.backend.RemoveBinding(b.interfaceName,b.elemName)
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
func (b *Binding) Name() string {
	return b.interfaceName + "." + b.elemName
}

// Binding searches a concrete binding by the given interface and method name.
func (b BindingContainer) Binding(i string, mn string) (r *Binding) {
	if _,found := b[i];!found {
		return
	}

	r,_ = b[i][mn]
	return
}

// Binding is a convenience method to retrieve the method of a interface. It panics if the method does not exist.
func (i Interface) Binding(n string) (r *Binding) {
	r, found := i[n]
	if !found{
		panic(fmt.Errorf("Binding \"%s\" not found for interface.",n))
	}
	return
}

// Remove an entire interface from the binding container identified by the interface name.
func (b BindingContainer) RemoveInterface(i string) {
	delete(b,i)
}

// RemoveBinding removes a single method from the binding container identified by the interface and method name.
func (b BindingContainer) RemoveBinding(i,m string) {
	delete(b[i],m)
}

// InterfaceNames retrieves all bound interface names.
func (b BindingContainer) InterfaceNames() (keys []string) {
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
func (b BindingContainer) Interfaces() (ret Interfaces) {
	ret = make(Interfaces,len(b))
	i:=0
	for _,v := range b {
		ret[i] = v
		i++
	}
	return
}

//Interface is a convenience method to retrieve an interface. It panics if the interface does not exist.
func (b BindingContainer) Interface(name string) (ret Interface) {
	ret,found := b[name]
	if !found {
		panic(fmt.Errorf("Interface \"%s\" does not exist.",name))
	}
	return
}

// BindingNames retreives all bound methods or functions names of the given interface.
func (b BindingContainer) BindingNames(i string) (methods []string) {
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
func (b BindingContainer) Bindings() (ret Bindings) {
	is := b.Interfaces()
	for _,v := range is {
		bdns := v.Bindings()
		ret = append(ret,bdns...)
	}
	return
}

// Invoke a bound method or function of the given interface and method name.
func (b BindingContainer) Invoke(i,m string, args ...interface{}) interface{} {
	return b.InvokeI(i,m,nil,args...)
}

// InvokeI is a convenience method for invoking methods/function without prior discovery.
func (b BindingContainer) InvokeI(i,m string,inj Injections, args ...interface{}) interface{} {
	if r := b.Binding(i,m);r != nil {
		return r.InvokeI(inj,args...)
	} else {
		panic(fmt.Errorf("Binding \"%s.%s\" not found.",i,m))
	}
	return nil
}

// Invoke a bound method or function with the given parameters.
func (r *Binding) Invoke(args ...interface{}) (ret interface{}) {
	return r.InvokeI(nil,args...)
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

func (r* Binding) callValuesI(inj Injections, args ...interface{}) (ret []reflect.Value) {
	targetArgCount := r.argCount()
	//log.Printf("%s target: %d, source: %d, (%d injections) ",r.interfaceName + "." + r.elemName,targetArgCount,len(args),len(inj))
	ret = make([]reflect.Value,targetArgCount)
	ic := 0 // count of found injections
	iai := 0
	for ai := 0; ai < targetArgCount; ai++ {
		at := r.argType(ai)
		//log.Printf("ArgType of %d: %s, Injection type of %d: %s",ai,at,ai,r.injections[ai])
		var av reflect.Value

		// Check if this parameter needs to be injected
		if _,ok:= r.injections[ai]; ok {
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

		// Assign final value to final call vector.
		if (at.Kind() != av.Kind()) {
			ret[ai] = av.Convert(at) // Try to convert.
		} else {
			ret[ai] = av
		}
	}

	if targetArgCount != (iai+ic) {
		panic(fmt.Errorf("Argument count does not match for method \"%s\". %d/%d. (%d injections applied)",r.elemName,targetArgCount,(iai+ic),ic))
	}

	return
}

//invokeRemoteBindingI is an internally used method to invoke a proxy type binding
// using the given unjections opjects and binding parameters
func (r* Binding) invokeMethodBindingI(inj Injections, args []interface{}) []reflect.Value {
	val := reflect.ValueOf(r.i)

	//Sanity check whether binding object is not of kind function.
	if val.Kind() == reflect.Func {
		panic(fmt.Errorf("MethodBinding for \"%s.%s\" does not bind an object.",r.interfaceName,r.elemName))
	}

	av := r.callValuesI(inj,args...)

	// Prepend the receiver
	cav := make([]reflect.Value,len(av)+1)
	cav[0] = val
	for i,v := range av {
		cav[i+1] = v
	}

	meth := reflect.TypeOf(r.i).Method(r.elemNum).Func
	return meth.Call(cav) // Call with receiver and consider injected objects.
}

//invokeFunctionBindingI is an internally used method to invoke a function type binding
// using the given unjections opjects and binding parameters
func (r* Binding) invokeFunctionBindingI(inj Injections, args []interface{}) []reflect.Value {
	val := reflect.ValueOf(r.i)

	// Sanity check whether binding object is actually of kind function
	if val.Kind() != reflect.Func {
		panic(fmt.Errorf("FunctionBinding for \"%s.%s\"  does not bind a function.",r.interfaceName,r.elemName))
	}
	meth := reflect.ValueOf(r.i)

	av := r.callValuesI(inj,args...)
	return meth.Call(av) // Call with receiver and consider injected objects.
}

//invokeRemoteBindingI is an internally used method to invoke a proxy type binding
// using the given unjections opjects and binding parameters
func (r* Binding) invokeRemoteBindingI(inj Injections, args []interface{}) []reflect.Value {
	// Sanity check whether binding object is actually of kind function
	if _,ok := r.i.(RemoteBinder); ok {
		panic(fmt.Errorf("FunctionBinding for \"%s.%s\"  does not bind a function.",r.interfaceName,r.elemName))
	}
	meth := reflect.ValueOf(r.i)

	av := r.callValuesI(inj,args) //Imporatent: use args as array parameter (NOT exploded !)
	return meth.Call(av)
}

// Invoke a bound method or function. Given injection objects are injected on demand.
// TODO: split in a seperate function for each binding type !
func (r *Binding) InvokeI(ri Injections,args ...interface{}) (ret interface{}) {
	//Merge Injections. Runtime objects overwrite singletons.
	inj := MergeInjections(r.singletons,ri)


	// Involve filters first, because Injection objects may be passed by filters
	for _,f := range r.filters {
		if !f(r,inj) {
			return nil
		}
	}

	var iret []reflect.Value
	switch r.t {
		case MethodBinding:
			iret = r.invokeMethodBindingI(inj,args)
		case FunctionBinding:
			iret = r.invokeFunctionBindingI(inj,args)
		case RemoteBinding:
			iret = r.invokeRemoteBindingI(inj,args)
		case AttributeBinding:
			return reflect.ValueOf(r.i).Elem().Field(r.elemNum).Interface()
		default:
			panic(fmt.Errorf("Invalid attribute type '%d'",r.t))
	}

	/* Check if return argument exists or not. If not nil is returned as interface{} */
	switch len(iret) {
		case 0:
			ret = nil
		default:
			log.Printf("Too many return arguments %d/%d. Ignoring.",len(iret),1)
			fallthrough
		case 1:
			ret =  iret[0].Interface() // Convert return argument to interface{}
	}
	return
}
