package gotojs_test

import (
	"testing"
	"os/exec"
	"bytes"
	"fmt"
	"errors"
	"encoding/json"
	"net/http"
	. "gotojs"
)

const (
	nodeCmd = "node"
	nodeRequire =`
var $ = require("jquery");
`

)

var frontend *Frontend

func TestInitialization(t *testing.T) {
	frontend = NewFrontend(
		Parameters{
			P_FLAGS: Flag2Param(F_VALIDATE_ARGS | F_ENABLE_ACCESSLOG),
			P_NAMESPACE: "PROXY",
			P_EXTERNALURL: "http://localhost:8786/gotojs",
			P_BASEPATH: "../.."})

	frontend.ExposeInterface(MyTestService)
	go func() {
		frontend.Start()
	}()
}

//Check whether node JS engine is executable.
func existsNodeJS() bool {
	cmd := exec.Command(nodeCmd,"-e \"" + nodeRequire + "\"");
	err := cmd.Run();
	return err == nil
}

func executeJS(t *testing.T,fronted *Frontend, postCmd ...string) (string,error){
	t.Logf("Executing nodejs engine \"%s\"...",nodeCmd)
	cmd := exec.Command(nodeCmd)
	stdin,err:= cmd.StdinPipe()
	if err != nil {
		t.Errorf("Creating nodejs pipe failed: %s",err.Error())
	}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err = cmd.Start()
	stdin.Write([]byte(nodeRequire)) // Load dependency to simulate domtree.
	frontend.Build(stdin)
	for _,s := range postCmd {
		n,err := stdin.Write([]byte(s));
		if err!= nil || n!=len(s) {
			t.Errorf("Could not send javascript statements to pipe: %s",err.Error())
		}
	}
	stdin.Close()
	err = cmd.Wait()
	resp:= buf.String()
	if err != nil {
		err = errors.New(fmt.Sprintf("%s :\n%s",err.Error(),resp))
	}
	return resp,err
}

func TestParseJS(t *testing.T) {
	if !existsNodeJS() {
		//TODO: Change this to skip
		t.Logf("Node.js not available. Skipping this test ...",nodeCmd)
		return
	}
	if _,err := executeJS(t,frontend); err != nil {
		t.Errorf("Executing nodejs parser failed: %s",err.Error())
	}
	t.Logf("Successfully parsed generated JS code.")
}

func TestValidationString(t *testing.T) {
	vs := frontend.BindingContainer["TestService"]["SetAndGetParam"].ValidationString()
	t.Logf("Validation string for \"%s.%s\" is : %s.","TestService","SetAndGetParam",vs)
	if vs != "i" {
		t.Errorf("Incorrect validation string: %s",vs)
	}
	frontend.ExposeFunction( func (i int, o struct{}, f float32, s string, sa []string) int {
		t.Logf("%s,%s,%s,%s,%s",i,o,f,s,sa)
		return 0
	},"X","test")
	vs = frontend.BindingContainer["X"]["test"].ValidationString()
	t.Logf("Validation string for \"%s.%s\" is : %s.","X","test",vs)
	if vs != "iofsa" {
		t.Errorf("Incorrect validation string: %s",vs)
	}
}

func TestValidationStringWithInjection(t *testing.T) {
	frontend.ExposeFunction( func (s *Session,c *HTTPContext) int { return 0 },"X","test")
	vs := frontend.BindingContainer["X"]["test"].ValidationString();
	if len(vs) != 0 {
		t.Errorf("Incorrect validation string: \"%s\"/\"%s\"",vs,"");
	}
}

type TestService3 struct{}
func (ts *TestService3) test(a,b,c string,session *Session) string{
	return a + b + c;
}

func TestValidationStringWithInjectionAndInterfaceExposure(t *testing.T) {
	frontend.ExposeInterface(&TestService3{})
	defer frontend.RemoveInterface("TestService3")
	vs := frontend.BindingContainer["TestService3"]["test"].ValidationString();
	if len(vs) != 3 {
		t.Errorf("Incorrect validation string: \"%s\"/\"%s\"",vs,"sss");
	}
}

func TestRemoveInterface(t *testing.T) {
	pi := frontend.Interface("X")
	if pi == nil {
		t.Errorf("Interface \"%s\" did not exist.","X")
	}
	frontend.RemoveInterface("X")
	ia := frontend.InterfaceNames()
	if ContainsS(ia,"X") {
		t.Errorf("Interface \"%s\" still exists after removal.","X")
	}
}

func TestSimpleCall(t *testing.T) {
	if !existsNodeJS() {
		t.Logf("Node.js not available. Skipping this test ...",nodeCmd)
		return
	}
	out,err := executeJS(t,frontend,"PROXY.TestService.SetAndGetParam(73,function(x) { if (x != 73) { throw 'failed'; }});");
	if err != nil {
		t.Errorf("Executing nodejs parser failed: %s",err.Error())
	}
	t.Logf(out)

}

func TestSimpleCallWithMultipleArgs(t *testing.T) {
	frontend.ExposeFunction( func (a,b int) int {
		return a+b
	},"Math","Add")

	out,err := executeJS(t,frontend,"PROXY.Math.Add(17,4, function(r) { if (r != 21) { throw 'Unexpected return value.';}});");
	if err != nil {
		t.Errorf("Executing nodejs parser failed or error occured: %s",err.Error())
	}
	t.Logf(out)
	frontend.RemoveInterface("Math")
}


func TestArgumentValidation(t *testing.T) {
	if !existsNodeJS() {
		t.Logf("Node.js not available. Skipping this test ...",nodeCmd)
		return
	}
	out,err := executeJS(t,frontend,"PROXY.TestService.SetAndGetParam(72,5,function(){});");
	if err == nil {
		t.Errorf("Executing nodejs parser succeeded. An argument assert error was expected.")
	}
	t.Logf(out)

	out,err = executeJS(t,frontend,"PROXY.TestService.SetAndGetParam(74,function(){});");
	if err != nil {
		t.Errorf("A callback handler as last parameter must be accepted.")
	}
	t.Logf(out)

	out,err = executeJS(t,frontend,"PROXY.TestService.SetAndGetParam('INVALID',function(){})");
	if err == nil {
		t.Errorf("Executing nodejs parser succeeded. An argument assert error was expected.")
	}
	t.Logf(out)
}

func TestDynamicHTTPContextInjection(t *testing.T) {
	if !existsNodeJS() {
		t.Logf("Node.js not available. Skipping this test ...",nodeCmd)
		return
	}

	frontend.ExposeFunction(func (a int,b int,c *HTTPContext) int {
		if c != nil && c.Response != nil && c.Request != nil {
			return a+b
		} else {
			return -1
		}
	},"X","add")

	out,err := executeJS(t,frontend,"PROXY.X.add(1,2,function(val){ if (val != 3) { throw 'Injection failed: ' + val; }});");
	if err !=nil {
		t.Logf(out)
		t.Errorf("HTTP Context Injection failed.")
	}
}

func TestJSONError(t *testing.T) {
	buf := bytes.NewBufferString("{'abc':'def'}")
	resp,_ := http.Post("http://localhost:8786/gotojs/TestServer/SetAndGetParam", "test/plain", buf)
	o := struct {Error string}{}
	dec:=json.NewDecoder(resp.Body)
	if err := dec.Decode(&o); err != nil{
		t.Errorf("Could not decode error message: %s",err.Error())
	}
	t.Logf("Message received: %s",o.Error)

}

func TestAutoInjectionFilter(t *testing.T) {
	fakeHeader := "text/fake"
	// Make sure to clear all filters after the test is completed.
	defer frontend.Bindings().ClearFilter()

	frontend.Bindings().If(AutoInjectF(func(inj Injections,c *HTTPContext, b *Binding) bool {
		return len(inj) > 0 && b.Name() == "TestService.SetParam"
	})).If(AutoInjectF(func(c *HTTPContext) bool {
		return c.Request.Method == "POST" && c.Request.Header.Get("Content-Type") == fakeHeader
	}))

	// Do a quick call to SetParam
	buf := bytes.NewBufferString("{\"Interface\":\"TestService\",\"Method\":\"SetParam\",\"Data\": [1000]}")
	_,_ = http.Post("http://localhost:8786/gotojs/TestServer/SetParam", fakeHeader, buf)

	frontend.Bindings().ClearFilter()
	res := frontend.Invoke("TestService","GetParam")
	if res != 1000 {
		t.Errorf("Filter has forbidden access. %d/%d",res,1000)
	}
}

func TestSession (t *testing.T) {
	key := GenerateKey(16)
	s := NewSession()
	s.Set("testkey","testval")
	c := s.Cookie("gotojs","/",key)
	t.Logf("Cookie: %s=%s",c.Name,c.Value)
	ns := SessionFromCookie(c,key)
	for k,v:= range ns.Properties {
		t.Logf("%s: %s",k,v)
	}
	if ns.Get("testkey") != "testval" {
		t.Errorf("Session could not be restored.")
	}
}

func TestInvalidSession (t *testing.T) {
	key := GenerateKey(16)
	c := &HTTPContext{}
	buf := new(bytes.Buffer)
	c.Request,_ = http.NewRequest("POST","http://localhost:666/Ignoreme",buf)
	c.Request.Header["Cookie"] = []string{DefaultCookieName + "=asdnjasndhabsdahsbdasdhasd; path=/"}
	s := c.Session(key)
	if s == nil {
		t.Errorf("Session call should always return a valid session.")
	}
}

func BenchmarkSessions (b *testing.B) {
	key := GenerateKey(16)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s:= NewSession()
		s.Set("param1","value1")
		s.Set("param2","value2")
		c:= s.Cookie("gotojs","/",key)
		_ = SessionFromCookie(c,key)
	}
}

func BenchmarkFrontend (b *testing.B) {
	frontend.ExposeFunction(fibonacci,"MATH","FIBO")
	defer frontend.RemoveInterface("MATH")
	b.ResetTimer()
	for i:=0; i<b.N; i++ {
		fakeHeader := "text/fake"
		buf := bytes.NewBufferString("{\"Interface\":\"MATH\",\"Method\":\"FIBO\",\"Data\": [100000000]}")
		_,_ = http.Post("http://localhost:8786/gotojs/MATH/FIBO", fakeHeader, buf)
	}
}


