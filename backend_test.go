package gotojs_test
import (
	"testing"
	. "gotojs"
	"log"
	//"fmt"
)

type TestService struct {
	param1 int
}

type TestService2 struct {
	param2 int
}

type TestAttributeService struct {
	Param int
}

var MyTestService = TestService{param1: 0}
var MyTestService2 = TestService2{param2: 0}
var backend = NewBackend()

func (t *TestService) SetAndGetParam(p int)  int	{ /*log.Printf("Invoked on %p",t); */ t.param1 = p; return t.param1}
func (t *TestService) GetParam() int			{ /* log.Printf("TestService.GetParam() @ %p",t);*/ return t.param1 }
func (t *TestService) SetParam(p int)			{ log.Printf("TestService.SetParam(%d) @ %p\n",p,t); t.param1 = p}
func (t TestService)  SetAndGetParam2(p int)  int	{ t.param1 = p; return t.param1}
func (t *TestService) InvalidMethod1(p int)  (int,int)	{ return p,0}
func (t *TestService) InvalidMethod2()  (int,int)	{ return 0,0}

func (t *TestService2) YetAnotherMethod(p int) int	{t.param2 =p; return t.param2 }

const (
	validMethodCount = 4
	validMethodCount2 = 1
)

func TestBasic(t *testing.T) {
	if len(backend.ExposeInterface(MyTestService)) == 0 {
		t.Errorf("Type could not be recognized by Exposer.")
	} else {
		t.Logf("Interface successfully exposed.");
	}
	t.Log(backend)

	MyTestService.SetParam(42)
}

func TestInterfaceNames(t *testing.T) {
	i := backend.InterfaceNames()
	for _,v:= range i {
		t.Logf("Found interface \"%s\".",v)
	}

	l:=len(i)

	if l!=1 {
		t.Errorf("Incorrect number of interfaces exposed: %d / %d.",l,1)
	}

	if i[0] != "TestService" {
		t.Errorf("Interface name is successfully propagated ot bindings table: \"%s\" / \"%s\".",i[0],"TestService")
	}
}

func TestMethods(t *testing.T) {
	i := backend.BindingNames(backend.InterfaceNames()[0])

	for _,v:= range i {
		t.Logf("Found method \"%s\".",v)
	}

	l:=len(i)

	if l!=validMethodCount {
		t.Errorf("Incorrect number of methods for interface: %d / %d.",l,validMethodCount)
	}
}

func TestOverwrite(t *testing.T) {
	ol:=len(backend.Interfaces())
	backend.ExposeInterface(&MyTestService)
	nl:=len(backend.Interfaces())

	if ol != nl {
		t.Errorf("Reprocessing the same interface should overwrite existing bindings. %d / %d.",nl,ol)
	}
}

func TestInvoke(t *testing.T) {
	ret := backend.Invoke("TestService","GetParam")
	val,ok := ret.(int)
	if !ok {
		t.Errorf("Unexpected method return type.")
	}

	if val !=42 {
		t.Errorf("Method invocation returned unexpected (initial) value: %d / %d",val,42)
	}

	ret = backend.Invoke("TestService","SetAndGetParam",2)
	val,ok = ret.(int)
	if val !=2 {
		t.Errorf("Method invokation returned unexpected value: %d / %d",val,2)
	}

	MyTestService.SetParam(3)
	ret = backend.Invoke("TestService","GetParam")
	val,ok = ret.(int)
	if val !=3 {
		t.Errorf("Method invokation returned unexpected value: %d / %d",val,3)
	}
}

func TestBasicSetter(t *testing.T) {
	backend.Invoke("TestService","SetParam",109)
	ret := backend.Invoke("TestService","GetParam")

	if ret != 109 {
		t.Errorf("Simple setter failed. %d/%d",ret,109)
	}

	MyTestService.SetParam(108)
	ret = backend.Invoke("TestService","GetParam")
	if ret != 108 {
		t.Errorf("Simple setter failed: %d/%d",ret,108)
	}

}

func TestExposeNamedInterface(t *testing.T) {
	a := len(backend.ExposeInterface(MyTestService2))
	b := len(backend.ExposeInterface(MyTestService2,"TestService"))
	if !((a == b) && (a == 1)) {
		t.Errorf("Additional interface exposing failed. %d,%d /%d",a,b,1)
	}

	if len(backend.Interfaces()) != 2 {
		t.Errorf("Additional interface exposing failed. New Interface is mussing.")
	}

	if len(backend.BindingNames("TestService")) != (validMethodCount2 + validMethodCount) {
		t.Errorf("Additional interface exposing failed. New Methods are missing.")
	}
}

func TestExposeFunction(t *testing.T) {
	f:=func(i int) int {
		return i
	}
	backend.ExposeFunction(f,"TestService","f")
}

func TestInterfaceAndBindingCount(t* testing.T) {
	is := backend.Interfaces();
	if len(is) != 2 {
		t.Errorf("Invalid count of Interfaces: %d/%d",len(is),2)
	}

	bs := backend.Bindings();
	if len(bs) != 7 {
		t.Errorf("Invalid count of total bindings: %d/%d",len(bs),7)
	}
}

func TestInvokeFunction(t *testing.T) {
	ret := backend.Invoke("TestService","f",17)
	val := ret.(int)
	if val != 17 {
		t.Errorf("Function invokation returned unexpected value: %d / %d",val,17)
	}
}

type TestContext struct {
	val string
}

func TestDynamicInjection(t *testing.T) {

	f := func(a int,b int, tc *TestContext) (string){
		return tc.val;
	}

	tc:= TestContext{val: "INITIAL"}
	backend.SetupGlobalInjection(&tc)
	backend.ExposeFunction(f,"IService","testMe")

	tc.val = "ASSERTME"
	res := backend.InvokeI("IService","testMe",NewI(&tc),5,6)
	if res != "ASSERTME" {
		t.Errorf("TestContext was not successfully injected %s",res)
	}

}

func TestDynamicInjectionFirstParam(t *testing.T) {
	ff := func(tc *TestContext,a int ,b int) (string){
		return tc.val;
	}

	backend.ExposeFunction(ff,"IService","testMe2")

	tc:= TestContext{val: "ASSERTME2"}
	res := backend.InvokeI("IService","testMe2",NewI(&tc),5,6)
	if res != "ASSERTME2" {
		t.Errorf("TestContext was not successfully injected %s",res)
	}
}

func TestDynamicInjectionSimilarParameter(t *testing.T) {
	f := func(a interface{},tc *TestContext,b interface{},c interface{}) (string){
		ia := a.(int)
		ib := b.(int)
		ic := c.(int)
		if (ia+ib) == ic {
			return tc.val
		}
		return ""
	}

	backend.ExposeFunction(f,"IService","testMe3")

	tc:= TestContext{val: "ASSERTME3"}
	res := backend.InvokeI("IService","testMe3",NewI(&tc),1,5,6)
	if res != "ASSERTME3" {
		t.Errorf("TestContext was not successfully injected %s",res)
	}
}

func TestInterfaceRemoval(t *testing.T) {
	backend.RemoveInterface("IService")
	if ContainsS(backend.InterfaceNames(),"IService") {
		t.Errorf("Interface \"%s\" still exists after removal.","IService")
	}
}

func TestSimpleFilterChain(t *testing.T) {
	works := false

	// Make sure to clear all filters after the test is completed.
	defer backend.Bindings().ClearFilter()

	_ = backend.Bindings().If(func(b *Binding,inj Injections) bool {
		return b.Name() == "TestService.GetParam"
	}).If(func(b* Binding, inj Injections) bool {
		works = true
		return works
	})

	_ = backend.Invoke("TestService","GetParam")

	if !works {
		t.Errorf("Filter was not invovled.")
	}
}

func TestSingletonInjection(t * testing.T) {
	type TestType struct {
		val int
	}

	mytt := TestType{42}

	backend.ExposeFunction(func(tt *TestType) int {
		return tt.val;
	},"TT","Get")


	backend.ExposeFunction(func(tt *TestType,v int) {
		tt.val = v
	},"TT","Set")

	defer backend.RemoveInterface("TT")

	backend.Interface("TT").Bindings().AddInjection(&mytt)

	ret := backend.Invoke("TT","Get")
	if ret != 42 {
		t.Errorf("Initial value incorrect: %d/%d",ret,42)
	}

	mytt.val = 43
	ret = backend.Invoke("TT","Get")
	if ret != 43 {
		t.Errorf("Value incorrect: %d/%d",ret,43)
	}

	backend.Invoke("TT","Set",44)
	ret = mytt.val
	if ret != 44 {
		t.Errorf("Value incorrect: %d/%d",ret,44)
	}

	ret = backend.InvokeI("TT","Get",NewI(&TestType{45}))
	if ret != 45 {
		t.Errorf("Value incorrect: %d/%d",ret,45)
	}

	mytt.val = 46
	backend.InvokeI("TT","Set",NewI(&TestType{45}),47)
	ret = mytt.val
	if ret != 46 {
		t.Errorf("Value incorrect: %d/%d",ret,46)
	}
}

func TestPassInjection(t *testing.T) {
	type TestType1 struct{
		val1 int
	}

	type TestType2 struct {
		val2 int
	}

	mytt1 := TestType1{1}
	mytt2 := TestType2{2}

	backend.ExposeFunction(func(tt *TestType2) int {
		return tt.val2;
	},"TT","Get")
	defer backend.RemoveInterface("TT")

	b := backend.Interface("TT").Bindings()

	b.AddInjection(&mytt2) // Delcare type and assign singleton
	b.AddInjection(&mytt1) // Declare type and assign singleton
	b.If(AutoInjectF(func(tt1 *TestType1, inj Injections) bool {
		if tt1.val1 == 1  {
			tt2 := TestType2{3}
			inj.Add(&tt2)
		}
		return true
	}))

	ret := backend.Invoke("TT","Get")
	if ret != 3 {
		t.Errorf("Injection passing failed: %d/%d",ret,3)
	}
}

func TestNegativeFilterChain(t *testing.T) {
	MyTestService.SetParam(0)
	// Now SetParam still works
	backend.Invoke("TestService","SetParam",17)

	b,_ := backend.Binding("TestService","SetParam")
	defer b.ClearFilter() // Make sure to remove filter after test.

	b.If(func(b *Binding,inj Injections) bool {
		return true // This one allows
	}).If(func(b *Binding,inj Injections) bool {
		return false // this one forbids
	})

	// This call is expected ot to be successful.
	_ = backend.Invoke("TestService","SetParam",18)

	if MyTestService.GetParam() != 17 {
		t.Errorf("Filter failed. Call was successful. %d/%d",MyTestService.GetParam(),17)
	}
}

func BenchmarkInvocation(b *testing.B) {
	/* Check the plain invocation framework here. */
	for i := 0; i < b.N; i++ {
		 _ = backend.Invoke("TestService","GetParam")
	}
}

func fibonacci(in int) (ret int) {
	cache := []int{0,1}
	ret = cache[in % 2]
	for i:=2; i <= in;i++ {
		ret := cache[0] + cache[1]
		cache[0] = cache[1]
		cache[1] = ret
	}
	return
}

func BenchmarkFibonacci(b *testing.B) {
	backend.ExposeFunction(fibonacci,"MATH","FIBO")
	defer backend.RemoveInterface("MATH")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		 _ = backend.Invoke("MATH","FIBO",100000000)
	}
}

func TestExposeMethod(b *testing.T) {
	backend.ExposeMethod(MyTestService,"GetParam","AlternateService")
	_,found := backend.Binding("AlternateService","GetParam")

	i := backend.Interface("AlternateService")
	defer backend.RemoveInterface("AlternateService")

	if len(i.Bindings()) != 1 || !found {
		b.Errorf("Regexp filter for exposing a single method failed.")
	}
}

func TestRegexpFilter(t *testing.T) {
	pattern := `\.(Get|Set)Param$`
	bs := backend.Bindings().Match(pattern)
	found := 0
	for _,b := range bs {
		if !(b.Name() == "TestService.GetParam" || b.Name() == "TestService.SetParam") {
			t.Errorf("Regexp filter failed: \"%s\" -> \"%s\"",pattern,b.Name())
		} else {
			found++
		}
	}

	if found < 2 {
		t.Errorf("Regexp filter failed. Not all bindings matching \"%s\" found. %d,%d",pattern,found,2)
	}
}

func TestExposeAttributes(t* testing.T) {
	obj := TestAttributeService{Param: 5}
	backend.ExposeAllAttributes(&obj,"AS")
	defer backend.RemoveInterface("AS")

	x := backend.Invoke("AS","Param")
	if x != 5 {
		t.Errorf("Exposing of attributes failed. %d/%d",x,5)
	}
}


func TestExposeYourself(t *testing.T) {
	backend.ExposeYourself("A")
	defer backend.RemoveInterface("AS")
	list := backend.Invoke("A","Bindings").([]string)

	if len(list) <= 0 {
		t.Errorf("Selfexposure failed.")
	}

	for _,s := range list {
		t.Logf("Method: %s",s)
	}
}

