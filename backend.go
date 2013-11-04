package gotojs

import (
	"reflect"
	"log"
	"strings"
	"errors"
	"fmt"
)

const (
	DefaultInterfaceName string = "main"
	DefaultFunctionName string = "f"
)

// Method is a concrete method binding. It maps a interface and method name to a go object's method.
type Method struct {
	methodName string
	interfaceName string
	methodNum int
	injections map[int]Injection
	i interface{}
}

// Interface represents an interface binding which consists of a set of methods or functions.
type Interface map[string]Method

// Binding represents a binding container which constists of a set of bound interfaces.
type Binding map[string]Interface

// Injection is an injection reference that is used to maintain the target type.
type Injection struct{
	Type reflect.Type
}

// Injections is a container of injection objects.
type Injections []interface{}

// Backend consits of the binding container as well as some administrative attributes like revision and injection references.
type Backend struct {
	Binding
	injections map[reflect.Type]Injection
	revision uint64
}

//NewBackend is a constructor for the Binding data structure.
func NewBackend() (Backend) {
	return Backend{ Binding: make(Binding), injections: make(map[reflect.Type]Injection) }
}


// SetupInjection declares a type that needs to be injected. This needs to be done before an interface or single function
// taking this type as a parameter is exposed.
func (b *Backend) SetupInjection(i interface{}) {
	t:= reflect.TypeOf(i)
	b.injections[t] = Injection{Type: t}
	//log.Printf("New injection for type name: \"%s\".",t.Name())
	//TODO: Update existing bindings !
}


// Expose an entire interface. All methods of the given interface will be exposed. THe name of the
// exposed interface is either taken from type name or could be specified as additional name parameter.
func (b *Backend) ExposeInterface(i interface{},name ...string) (ret int) {
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
func (b *Backend) ExposeFunction(f interface{}, name ...string) (ret int) {
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

	//TODO: make clean and move to ExportFunction method of Binding
	_, found := b.Method(iname,fname)
	if found {
		log.Printf("Function \"%s\" already exposed for interface \"%s\". Overwriting.",fname,iname)
	}
	pm:=Method{ i: f, methodNum: -1,methodName: fname,interfaceName: iname}
	b.UpdateInjection(&pm)
	b.Binding[iname][fname] = pm
	b.revision++
	return 1
}

// Method searches a concrete binding by the given interface and method name.
func (b Binding) Method(i string, mn string) (r Method, found bool) {
	im, found := b[i];
	if !found {
		im = make(map[string]Method)
		b[i] = im
	}

	r,found = b[i][mn]
	if !found {
		r = Method{}
		b[i][mn] = r
	}
	return
}

// Method is a convenience method to retrieve the method of a interface. It panics if the method does not exist.
func (i Interface) Method(n string) (r Method) {
	r, found := i[n]
	if !found{
		log.Fatalf("Method \"%s\" not found for interface.",n)
	}
	return
}


// Remove an entire interface from the binding container identified by the interface name.
func (b Binding) RemoveInterface(i string) {
	delete(b,i)
}

// REmoveMethod removes a single method from the binding container identified by the interface and method name.
func (b Binding) RemoveMethod(i,m string) {
	delete(b[i],m)
}


// Interfaces retrieves all bound interface names.
func (b Binding) Interfaces() (keys []string) {
	keys = make([]string,len(b));
	i:=0
	for k,_ := range ( b ) {
		keys[i] = k
		i++
	}
	return
}

//Interface is a convenience method to retrieve an interface. It panics if the interface does not exist.
func (b Binding) Interface(name string) (ret Interface) {
	ret,found := b[name]
	if !found {
		log.Fatalf("Interface \"%s\" does not exist.",name)
	}
	return
}


// Methods retreives all bound methods or functions of the given interface name.
func (b Binding) Methods(i string) (methods []string) {
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


// Invoke a bound method or function of the given interface and method name.
func (b Binding) Invoke(i,m string, args ...interface{}) interface{} {
	return b.InvokeI(i,m,nil,args...)
}


// InvokeI is a convenience method for invoking methods/function without prior discovery.
func (b Binding) InvokeI(i,m string,inj Injections, args ...interface{}) interface{} {
	r,found := b.Method(i,m)
	if found {
		return r.InvokeI(inj,args...)
	} else {
		log.Fatalf("Method \"%s.%s\" not found.",i,m)
	}
	return nil
}


// Invoke a bound method or function with the given parameters.
func (r *Method) Invoke(args ...interface{}) (ret interface{}) {
	return r.InvokeI(nil,args...)
}


// Lookup whether the injections contain an object of the given type.
// Returns the object and nil as error if found. If not found the return object is nil
// and error is set accordingly.
func (i Injections) findType(t reflect.Type) (interface{},error) {
	if i == nil {
		return nil,errors.New(fmt.Sprintf("Empty injections list."))
	}
	for _,o := range []interface{}(i) {
		if reflect.TypeOf(o) == t {
			return o,nil;
		}
	}
	return nil,errors.New(fmt.Sprintf("Injection type %s not found.",t.Name()))
}

// NewI constructs a new Injections object. Each parameter is part of the Injections object.
func NewI(args ...interface{}) (Injections) {
	return args
}


// Invoke a bound method or function. Given injection objects are injected on demand.
func (r *Method) InvokeI(inj Injections,args ...interface{}) (ret interface{}) {
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
		//TODO: fix this reflection fuckup, maybe a bug in relect pkg
		// The data type structure of go's reflection implementation is either not understood or simply weired.
		meth = t.Method(r.methodNum).Func

		//ac = v.Method(r.methodNum).Type().NumIn() // Argument count without receiver ?!?
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
		if val,ok:= r.injections[fx]; ok {
			if in,ok := inj.findType(val.Type); ok == nil {
				iav = reflect.ValueOf(in).Convert(at)
				va[fx] = reflect.ValueOf(in).Convert(at)
			} else {
				log.Fatal(ok.Error())
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

// UpdateInjection checks if this method needs an injection. If yes the binding will be updated with proper references.
func (b *Backend) UpdateInjection(m *Method) {
	m.injections = make(map[int]Injection)

	t := reflect.TypeOf(m.i)
	v := reflect.ValueOf(m.i)

	var ac int
	var meth reflect.Value
	if m.methodNum >= 0 {
		ac = v.Method(m.methodNum).Type().NumIn()
		meth = t.Method(m.methodNum).Func
	} else {
		ac = t.NumIn()
		meth = v
	}

	for i:=0;i<ac;i++ {
		at := meth.Type().In(i)
		if val,ok := b.injections[at]; ok {
			m.injections[i] = val
		}
	}
}

// Internal function that oursources the actual exposing code from the ExposeInterface method.
func (b *Backend) exposeInterfaceTo(i interface{}, n string) (c int) {
	t := reflect.TypeOf(i)
	ow := 0
	for x:=0;x < t.NumMethod();x++ {
		mt := t.Method(x)
		/* Sanity check on method signature: */
		if (mt.Type.NumOut() > 1) {
			/* Only up to one return argument is allowed. */
			log.Printf("Ignoring method \"%s\" due to invalid amount of return parameters. %d / %d",mt.Name,mt.Type.NumOut(),1);
			continue
		}

		mn := mt.Name
		_, found := b.Method(n,mn)
		if found {
			ow++
			log.Printf("Method \"%s\" already exposed for interface \"%s\". Overwriting.",mn,n)
		}
		pm := Method{ i: i, methodNum: x,methodName: mn, interfaceName: n}
		b.UpdateInjection(&pm)
		b.Binding[n][mn] = pm
		c++;
	}
	log.Printf("Added %d methods to interface \"%s\". %d of %d have been overwritten.",c,n,ow,c)
	return
}
