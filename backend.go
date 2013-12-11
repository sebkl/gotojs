package gotojs

import (
	"reflect"
	"log"
	"strings"
	"regexp"
)

const (
	DefaultInterfaceName string = "main"
	DefaultFunctionName string = "f"
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
type Binding struct {
	methodName string
	interfaceName string
	methodNum int
	injections map[int]InjectionType
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

// InjectionType is an injection reference that is used to maintain the target type.
type InjectionType reflect.Type

// Injections is a container of injection objects sorted by their type.
type Injections map[InjectionType]interface{}

// Backend consits of the binding container as well as some administrative attributes like revision and injection references.
type Backend struct {
	BindingContainer
	globalInjections Injections
	revision uint64
}

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

	t := reflect.TypeOf(m.i)
	v := reflect.ValueOf(m.i)

	var ac int
	var meth reflect.Value
	off := 0
	if m.methodNum >= 0 {
		ac = v.Method(m.methodNum).Type().NumIn()
		meth = t.Method(m.methodNum).Func
		off = 1
	} else {
		ac = t.NumIn()
		meth = v
	}


	for i:=off;i<(ac+off);i++ {
		at := meth.Type().In(i)
		if at == it {
			m.injections[i] = at
		}
	}
	return m
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

// NewBinding creates a new binding object that is associated with the given backend.
// All existing global Injections will be added to this binding.
func (b* Backend) NewBinding(i interface{},x int, in,mn string) (*Binding) {
	p := Binding{
		i: i,
		methodNum: x,
		methodName: mn,
		interfaceName: in,
		backend: b,
		singletons: make(Injections),
		injections: make(map[int]InjectionType)}
	p.addGlobalInjections()
	return &p
}

// Expose an entire interface. All methods of the given interface will be exposed. THe name of the
// exposed interface is either taken from type name or could be specified as additional name parameter.
func (b *Backend) ExposeInterface(i interface{},name ...string) (ret Bindings) {
	// Try to get object/receiver name from interface.
	k:=reflect.ValueOf(i).Type().Kind();

	// If interface type is not a pointer, take it.
	if k != reflect.Ptr{
		ptr := reflect.New(reflect.TypeOf(i))
		temp := ptr.Elem()
		temp.Set(reflect.ValueOf(i))
		i = ptr.Interface()
	}

	// Find name either by args or by interface type. 
	if len(name) > 0 {
		ret =  b.exposeInterfaceTo(i,name[0])
	} else {
		// TODO: Fix this crap.
		name := reflect.ValueOf(i).Type().String()
		elems := strings.Split(name,".")
		iname :=elems[len(elems)-1]
		if len(iname) <= 0 {
			iname = DefaultFunctionName
		}
		ret = b.exposeInterfaceTo(i,iname)
	}
	b.revision++
	return
}

// ExposeFunction exposes a single function. No receiver is required for this binding.
func (b *Backend) ExposeFunction(f interface{}, name ...string) (ret Bindings) {
	v:= reflect.ValueOf(f)
	if v.Kind() != reflect.Func {
		log.Fatalf("Parameter is not a function. %s/%s",v.Kind().String(),reflect.Func.String())
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
	_, found := b.Binding(iname,fname)
	if found {
		log.Printf("Function \"%s\" already exposed for interface \"%s\". Overwriting.",fname,iname)
	}

	pm:= b.NewBinding(f,-1,iname,fname)
	b.BindingContainer[iname][fname] = pm
	b.revision++
	ret = make(Bindings,1)
	ret[0] = pm
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
	b.backend.RemoveBinding(b.interfaceName,b.methodName)
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
	return b.interfaceName + "." + b.methodName
}

// Binding searches a concrete binding by the given interface and method name.
func (b BindingContainer) Binding(i string, mn string) (r *Binding, found bool) {
	im, found := b[i];
	if !found {
		im = make(map[string]*Binding)
		b[i] = im
	}

	r,found = b[i][mn]
	if !found {
		r = &Binding{}
		b[i][mn] = r
	}
	return
}

// Binding is a convenience method to retrieve the method of a interface. It panics if the method does not exist.
func (i Interface) Binding(n string) (r *Binding) {
	r, found := i[n]
	if !found{
		log.Fatalf("Binding \"%s\" not found for interface.",n)
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
		log.Fatalf("Interface \"%s\" does not exist.",name)
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
	r,found := b.Binding(i,m)
	if found {
		return r.InvokeI(inj,args...)
	} else {
		log.Fatalf("Binding \"%s.%s\" not found.",i,m)
	}
	return nil
}

// Invoke a bound method or function with the given parameters.
func (r *Binding) Invoke(args ...interface{}) (ret interface{}) {
	return r.InvokeI(nil,args...)
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

// Invoke a bound method or function. Given injection objects are injected on demand.
func (r *Binding) InvokeI(ri Injections,args ...interface{}) (ret interface{}) {
	//Merge Injections. Runtime objects overwrite singletons.
	inj := MergeInjections(r.singletons,ri)

	// Involve filters first, because Injection objects may be passed by filters
	for _,f := range r.filters {
		if !f(r,inj) {
			return nil
		}
	}

	t := reflect.TypeOf(r.i)
	v := reflect.ValueOf(r.i)
	l := len(args)

	// Convert interface slice to reflect.Value slice, which is required by reflection 
	// method call whereby first element may be a receiver and some parameters could
	// be injected.
	var va []reflect.Value
	off := 0 // Offset due to receiver
	var meth reflect.Value
	var ac int // Count of parameters expected by the real method/function invocation
	if (r.methodNum >= 0) {
		// Binding is a interface with reference to the method
		off=1 // offset because of receiver 
		//TODO: fix this reflection fuckup.
		meth = t.Method(r.methodNum).Func

		//ac = v.Binding(r.methodNum).Type().NumIn() // Argument count without receiver ?!?
		ac = meth.Type().NumIn() // Argument count including recceiver ?!?
		va = make([]reflect.Value,ac)
		va[0] = v // Set receiver as first param of real call
	} else {
		// Binding is a function
		ac = t.NumIn()
		va = make([]reflect.Value,ac)
		if v.Kind() != reflect.Func {
			log.Fatalf("Binding for \"%s.%s\" is not a function.",r.interfaceName,r.methodName)
		}
		meth = v
	}

	mt := meth.Type()  // type object of the method/function
	ic := 0 // Count of injections needed

	for fx := off; fx < ac; fx++ {
		at:= mt.In(fx) // Type object of the current parameter
		var iav reflect.Value  // final value to be assigned to the call
		// Check if this parameter needs to be injected
		if _,ok:= r.injections[fx]; ok {
			if in,ok := inj[at]; ok { // a object of type at is provided by InvokeI call
				iav = reflect.ValueOf(in).Convert(at)
				va[fx] = reflect.ValueOf(in).Convert(at)
			} else {
				log.Fatalf("Injection for type \"%s\" not found.",at)
			}

			ic++ // skip one input param
			at = mt.In(fx)

		} else {
			ia := args[fx-(ic+off)] //rearrange by receiver and injections which are not part of incoming parameters
			iav = reflect.ValueOf(ia) // Value object of the current parameter
		}

		if (at.Kind() != iav.Kind()) {
			va[fx] = iav.Convert(at)
		} else {
			va[fx] = iav
		}
	}

	if ac != (l+ic+off) {
		log.Fatalf("Argument count does not match for method \"%s\". %d/%d.",r.methodName,ac,(l+ic+off))
		return
	}


	iret := meth.Call(va) // Call with/without receiver but consider injected objects.

	/* Check if return argument exists or not. If not nil is returned as interface{} */
	if len(iret) > 0 {
		ret = iret[0].Interface() // Convert return argument to interface{}
	} else {
		return nil
	}
	return
}

// Internal function that oursources the actual exposing code from the ExposeInterface method.
func (b *Backend) exposeInterfaceTo(i interface{}, n string) (ret Bindings) {
	t := reflect.TypeOf(i)
	ow := 0 // amount of overwritten methods.
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
		_, found := b.Binding(n,mn)
		if found {
			ow++
			log.Printf("Binding \"%s\" already exposed for interface \"%s\". Overwriting.",mn,n)
		}
		pm := b.NewBinding(i,x,n,mn)
		b.BindingContainer[n][mn] = pm

		//Compile return slice
		ret[c] = pm
		c++;
	}
	log.Printf("Added %d methods to interface \"%s\". %d of %d have been overwritten.",c,n,ow,c)
	return ret[:c]
}
