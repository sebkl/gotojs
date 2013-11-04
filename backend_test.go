package gotojs_test
import (
	"testing"
	. "gotojs"
	//"log"
)

type TestService struct {
	param1 int
}

type TestService2 struct {
	param2 int
}

var MyTestService = TestService{param1: 0}
var MyTestService2 = TestService2{param2: 0}
var backend = NewBackend()

func (t *TestService) SetAndGetParam(p int)  int	{ /*log.Printf("Invoked on %p",t); */ t.param1 = p; return t.param1}
func (t *TestService) GetParam() int			{ return t.param1 }
func (t *TestService) SetParam(p int)			{ t.param1 = p}
func (t TestService)  SetAndGetParam2(p int)  int	{ t.param1 = p; return t.param1}
func (t *TestService) InvalidMethod1(p int)  (int,int)	{ return p,0}
func (t *TestService) InvalidMethod2()  (int,int)	{ return 0,0}

func (t *TestService2) YetAnotherMethod(p int) int	{t.param2 =p; return t.param2 }

const (
	validMethodCount = 4
	validMethodCount2 = 1
)

func TestBasic(t *testing.T) {
	if backend.ExposeInterface(MyTestService) == 0 {
		t.Errorf("Type could not be recognized by Exposer.")
	} else {
		t.Logf("Interface successfully exposed.");
	}
	t.Log(backend)

	MyTestService.SetParam(42)
}

func TestInterfaces(t *testing.T) {
	i := backend.Interfaces()
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
	i := backend.Methods(backend.Interfaces()[0])

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

func TestExposeNamedInterface(t *testing.T) {
	a := backend.ExposeInterface(MyTestService2)
	b := backend.ExposeInterface(MyTestService2,"TestService")
	if !((a == b) && (a == 1)) {
		t.Errorf("Additional interface exposing failed. %d,%d /%d",a,b,1)
	}

	if len(backend.Interfaces()) != 2 {
		t.Errorf("Additional interface exposing failed. New Interface is mussing.")
	}

	if len(backend.Methods("TestService")) != (validMethodCount2 + validMethodCount) {
		t.Errorf("Additional interface exposing failed. New Methods are missing.")
	}
}

func TestExposeFunction(t *testing.T) {
	f:=func(i int) int {
		return i
	}
	backend.ExposeFunction(f,"TestService","f")
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
	backend.SetupInjection(&tc)
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
	if ContainsS(backend.Interfaces(),"IService") {
		t.Errorf("Interface \"%s\" still exists after removal.","IService")
	}
}

