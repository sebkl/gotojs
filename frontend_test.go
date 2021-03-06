package gotojs

import (
	"bytes"
	"errors"
	"fmt"
	. "github.com/sebkl/gotojs/client"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

const (
	nodeCmd           = "node"
	nodeJQueryRequire = `
var $ = require("jquery");
`
	engineJQuery = "jquery"
	engineNodeJS = "nodejs"
)

var container *Container

func TestInitialization(t *testing.T) {
	container = NewContainer(
		Properties{
			P_FLAGS:       Flag2Param(F_VALIDATE_ARGS | F_ENABLE_ACCESSLOG),
			P_NAMESPACE:   "PROXY",
			P_EXTERNALURL: "http://localhost:8786/gotojs",
			P_BASEPATH:    "../.."})

	container.ExposeInterface(MyTestService)
	go func() {
		container.Start()
	}()
}

//Check whether node JS engine is executable.
func existsNodeJS() bool {
	cmd := exec.Command(nodeCmd, "-e \""+nodeJQueryRequire+"\"")
	err := cmd.Run()
	return err == nil
}

func dumpResponse(t *testing.T, resp *http.Response, err error) {
	if resp != nil {
		for hn, va := range resp.Header {
			for _, v := range va {
				t.Logf("%s: %s", hn, v)
			}
		}
		by, err := ioutil.ReadAll(resp.Body)
		t.Logf("err: %s, body: %s", err, string(by))
	} else {
		t.Logf("NO RESPONSE: %s", err)
	}
}

func fakeContext() (ret *HTTPContext) {
	ret = &HTTPContext{}
	req, err := http.NewRequest("GET", "http://localhost:8786/gotojs/t1/t2", nil)
	ret.Request = req
	ret.Client = http.DefaultClient
	if err != nil {
		panic(err)
	}
	return ret
}

func executeJS(t *testing.T, fronted *Container, engine string, postCmd ...string) (string, error) {
	t.Logf("Executing nodejs engine \"%s\"...", nodeCmd)
	cmd := exec.Command(nodeCmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Errorf("Creating nodejs pipe failed: %s", err.Error())
	}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err = cmd.Start()
	stdin.Write([]byte(nodeJQueryRequire)) // Load dependency to simulate domtree.
	//container.build(&HTTPContext{},"http://localhost:8786/gotojs","jquery",stdin)
	req, _ := http.NewRequest("GET", "http://localhost:8786/gotojs/engine."+engine, nil)
	t.Logf(req.URL.String())
	t.Logf(container.extUrl.String())
	container.build(&HTTPContext{Request: req}, stdin)
	for _, s := range postCmd {
		n, err := stdin.Write([]byte(s))
		if err != nil || n != len(s) {
			t.Errorf("Could not send javascript statements to pipe: %s", err.Error())
		}
	}
	stdin.Close()
	err = cmd.Wait()
	resp := buf.String()
	if err != nil {
		err = errors.New(fmt.Sprintf("%s :\n%s", err.Error(), resp))
	}
	return resp, err
}

func TestParseJS(t *testing.T) {
	if !existsNodeJS() {
		//TODO: Change this to skip
		t.Logf("Node.js not available. Skipping this test ...", nodeCmd)
		return
	}
	if _, err := executeJS(t, container, engineNodeJS); err != nil {
		t.Errorf("Executing nodejs parser failed: %s", err.Error())
	}
	t.Logf("Successfully parsed generated JS code.")
}

func TestValidationString(t *testing.T) {
	vs := container.bindingContainer["TestService"]["SetAndGetParam"].ValidationString()
	t.Logf("Validation string for \"%s.%s\" is : %s.", "TestService", "SetAndGetParam", vs)
	if vs != "i" {
		t.Errorf("Incorrect validation string: %s", vs)
	}
	container.ExposeFunction(func(i int, o struct{}, f float32, s string, sa []string) int {
		t.Logf("%s,%s,%s,%s,%s", i, o, f, s, sa)
		return 0
	}, "X", "test")
	vs = container.bindingContainer["X"]["test"].ValidationString()
	t.Logf("Validation string for \"%s.%s\" is : %s.", "X", "test", vs)
	if vs != "iofsa" {
		t.Errorf("Incorrect validation string: %s", vs)
	}
}

func TestParameterTypeCount(t *testing.T) {
	container.ExposeFunction(func(bc *BinaryContent) {}, "a", "b")
	b, _ := container.Binding("a", "b")
	if count := countParameterType(b, &BinaryContent{}); count != 1 {
		t.Errorf("Incorrect ParameterTypeCount: %d/%d", count, 1)
	}
}

func TestValidationStringWithInjection(t *testing.T) {
	container.ExposeFunction(func(s *Session, c *HTTPContext) int { return 0 }, "X", "test")
	vs := container.bindingContainer["X"]["test"].ValidationString()
	if len(vs) != 0 {
		t.Errorf("Incorrect validation string: \"%s\"/\"%s\"", vs, "")
	}
}

type TestService3 struct{}

func (ts *TestService3) Test(a, b, c string, session *Session) string {
	return a + b + c
}

func TestValidationStringWithInjectionAndInterfaceExposure(t *testing.T) {
	container.ExposeInterface(&TestService3{})
	defer container.RemoveInterface("TestService3")
	vs := container.bindingContainer["TestService3"]["Test"].ValidationString()
	if len(vs) != 3 {
		t.Errorf("Incorrect validation string: \"%s\"/\"%s\"", vs, "sss")
	}
}

func TestRemoveInterface(t *testing.T) {
	pi := container.Interface("X")
	if pi == nil {
		t.Errorf("Interface \"%s\" did not exist.", "X")
	}
	container.RemoveInterface("X")
	ia := container.InterfaceNames()
	if ContainsS(ia, "X") {
		t.Errorf("Interface \"%s\" still exists after removal.", "X")
	}
}

func TestCallParameter(t *testing.T) {
	res, err := http.Get("http://localhost:8786/gotojs/TestService/SetAndGetParam/101")
	if res.StatusCode != http.StatusOK {
		dumpResponse(t, res, err)
		t.Errorf("Parameter as path failed.")
	}

	res, err = http.Get("http://localhost:8786/gotojs/TestService/SetAndGetParam?p=101")
	if res.StatusCode != http.StatusOK {
		dumpResponse(t, res, err)
		t.Errorf("Parameter as query string failed.")
	}

	res, err = http.Get("http://localhost:8786/gotojs/TestService/SetAndGetParam")
	if res.StatusCode == http.StatusOK {
		dumpResponse(t, res, err)
		t.Errorf("Negative Test call parameter failed.")
	}
}

func TestWrongCall(t *testing.T) {
	res, err := http.Get("http://localhost:8786/gotojs/TestService/SetAndGetParamxyz")
	if res.StatusCode != http.StatusNotFound {
		dumpResponse(t, res, err)
		t.Errorf("Expected 404 status for unknown binding %d/%d.", res.StatusCode, http.StatusNotFound)
	}

	eh := res.Header.Get(DefaultHeaderError)
	if len(eh) <= 0 {
		t.Errorf("Expected error header for unknwon binding: '%s'", eh)
	}
}

func TestSimpleJSCall(t *testing.T) {
	if !existsNodeJS() {
		t.Logf("Node.js not available. Skipping this test ...", nodeCmd)
		return
	}
	out, err := executeJS(t, container, engineNodeJS, "PROXY.TestService.SetAndGetParam(73,function(x) { if (x != 73) { throw 'failed was: ' + x; }});")
	if err != nil {
		t.Errorf("Executing nodejs parser failed (jqquery engine): %s", err.Error())
	}
	out, err = executeJS(t, container, engineNodeJS, "PROXY.TestService.SetAndGetParam(73,function(x) { if (x != 73) { throw 'failed was: ' + x; }});")
	if err != nil {
		t.Errorf("Executing nodejs parser failed (nodejs engine): %s", err.Error())
	}
	t.Logf(out)
}

func TestSimpleCallWithMultipleArgs(t *testing.T) {
	container.ExposeFunction(func(a, b int) int {
		return a + b
	}, "Math", "Add")

	out, err := executeJS(t, container, engineNodeJS, "PROXY.Math.Add(17,4, function(r) { if (r != 21) { throw 'Unexpected return value.';}});")
	if err != nil {
		t.Errorf("Executing nodejs parser failed or error occured: %s", err.Error())
	}
	t.Logf(out)
	container.RemoveInterface("Math")
}

func TestArgumentValidation(t *testing.T) {
	if !existsNodeJS() {
		t.Logf("Node.js not available. Skipping this test ...", nodeCmd)
		return
	}
	out, err := executeJS(t, container, engineNodeJS, "PROXY.TestService.SetAndGetParam(72,5,function(){});")
	if err == nil {
		t.Errorf("Executing nodejs parser succeeded. An argument assert error was expected.")
	}
	t.Logf(out)

	out, err = executeJS(t, container, engineNodeJS, "PROXY.TestService.SetAndGetParam(74,function(){});")
	if err != nil {
		t.Errorf("A callback handler as last parameter must be accepted.")
	}
	t.Logf(out)

	out, err = executeJS(t, container, engineNodeJS, "PROXY.TestService.SetAndGetParam('INVALID',function(){})")
	if err == nil {
		t.Errorf("Executing nodejs parser succeeded. An argument assert error was expected.")
	}
	t.Logf(out)
}

func TestDynamicHTTPContextInjection(t *testing.T) {
	if !existsNodeJS() {
		t.Logf("Node.js not available. Skipping this test ...", nodeCmd)
		return
	}

	container.ExposeFunction(func(a int, b int, c *HTTPContext) int {
		if c != nil && c.Response != nil && c.Request != nil {
			return a + b
		} else {
			return -1
		}
	}, "X", "add")

	out, err := executeJS(t, container, engineNodeJS, "PROXY.X.add(1,2,function(val){ if (val != 3) { throw 'Injection failed: ' + val; }});")
	if err != nil {
		t.Logf(out)
		t.Errorf("HTTP Context Injection failed.")
	}
}

func TestError(t *testing.T) {
	buf := bytes.NewBufferString("{'abc':'def'}")
	resp, _ := http.Post("http://localhost:8786/gotojs/TestServer/SetAndGetParam", "test/plain", buf)
	if errh := resp.Header.Get(DefaultHeaderError); len(errh) == 0 {
		t.Errorf("No Error header found in response.")
	} else {
		t.Logf("Message received: %s", errh)
	}
}

func TestAutoInjectionFilter(t *testing.T) {
	fakeHeader := "text/fake"
	// Make sure to clear all filters after the test is completed.
	defer container.Bindings().ClearFilter()

	container.Bindings().If(AutoInjectF(func(inj Injections, c *HTTPContext, b Binding) bool {
		log.Println(len(inj), b.Name())
		return len(inj) > 0 && b.Name() == "TestService.SetParam"
	})).If(AutoInjectF(func(c *HTTPContext) bool {
		log.Println(c.Request.Method, c.Request.Header.Get("Content-Type"))
		return c.Request.Method == "POST" && c.Request.Header.Get("Content-Type") == fakeHeader
	}))

	// Do a quick call to SetParam
	_, _ = http.Post("http://localhost:8786/gotojs/TestService/SetParam/1000", fakeHeader, bytes.NewBufferString(""))

	container.Bindings().ClearFilter()
	res := container.Invoke("TestService", "GetParam")
	if res != 1000 {
		t.Errorf("Filter has forbidden access. %d/%d", res, 1000)
	}
}

func TestSession(t *testing.T) {
	key := GenerateKey(16)
	s := NewSession()
	s.Set("testkey", "testval")
	c := s.Cookie("gotojs", "/", key)
	t.Logf("Cookie: %s=%s", c.Name, c.Value)
	ns := SessionFromCookie(c, key)
	for k, v := range ns.Properties {
		t.Logf("%s: %s", k, v)
	}
	if ns.Get("testkey") != "testval" {
		t.Errorf("Session could not be restored.")
	}
}

func TestInvalidSession(t *testing.T) {
	key := GenerateKey(16)
	c := &HTTPContext{}
	buf := new(bytes.Buffer)
	c.Request, _ = http.NewRequest("POST", "http://localhost:666/Ignoreme", buf)
	c.Request.Header["Cookie"] = []string{DefaultCookieName + "=asdnjasndhabsdahsbdasdhasd; path=/"}
	s := c.Session(key)
	if s == nil {
		t.Errorf("Session call should always return a valid session.")
	}
}

func TestGETCall(t *testing.T) {
	r, e := http.Get("http://localhost:8786/gotojs/TestService/GetParam")
	if e != nil {
		panic(e)
	}
	if r.StatusCode != 200 {
		defer r.Body.Close()
		body, _ := ioutil.ReadAll(r.Body)
		t.Errorf("GET invocation failed with status code: %d/%d %s", r.StatusCode, 200, body)
	}

}

func TestWiredGETCall(t *testing.T) {
	r, e := http.Get("http://localhost:8786/gotojs//../gotojs//TestService/../TestService///GetParam")
	if e != nil {
		panic(e)
	}
	if r.StatusCode != 200 {

		defer r.Body.Close()
		body, _ := ioutil.ReadAll(r.Body)
		t.Errorf("GET invocation failed with status code: %d/%d %s", r.StatusCode, 200, body)
	}
}

type ImageBinary struct {
	buf      *bytes.Buffer
	mimetype string
}

func (i ImageBinary) Close() error               { return nil }
func (i ImageBinary) Read(p []byte) (int, error) { return i.buf.Read(p) }
func (i ImageBinary) MimeType() string           { return i.mimetype }

func TestWiredBinaryCall(t *testing.T) {
	mt := "image/png"
	container.ExposeFunction(func(c int) (ret Binary) {
		b := make([]byte, c)
		for i, _ := range b {
			b[i] = '_'
		}
		ret = ImageBinary{buf: bytes.NewBuffer(b), mimetype: mt}
		return
	}, "X", "GenImage")

	r, e := http.Get("http://localhost:8786/gotojs/X/GenImage/5")
	if e != nil {
		panic(e)
	}

	defer r.Body.Close()
	body, _ := ioutil.ReadAll(r.Body)

	if r.StatusCode != 200 {
		t.Errorf("Binary GET failed with status code: %d/%d %s", r.StatusCode, 200, body)
	}

	if len(body) != 5 {
		t.Errorf("Binary GET failed. Incorrect body size : %d/%d", len(body), 5)
	}
	rmt := r.Header.Get("Content-Type")
	if rmt != mt { //TODO: change contains to equals
		t.Errorf("Binary GET failed. Incorrect mime type: %s,%s", rmt, mt)
	}
}

func BenchmarkSessions(b *testing.B) {
	key := GenerateKey(16)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := NewSession()
		s.Set("param1", "value1")
		s.Set("param2", "value2")
		c := s.Cookie("gotojs", "/", key)
		_ = SessionFromCookie(c, key)
	}
}

func BenchmarkContainer(b *testing.B) {
	container.ExposeFunction(fibonacci, "MATH", "FIBO")
	defer container.RemoveInterface("MATH")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fakeHeader := "text/fake"
		buf := bytes.NewBufferString("{\"Interface\":\"MATH\",\"Method\":\"FIBO\",\"Data\": [100000000]}")
		_, _ = http.Post("http://localhost:8786/gotojs/MATH/FIBO", fakeHeader, buf)
	}
}

func TestExposeProxyBase(t *testing.T) {
	hc := fakeContext()
	s := NewSession()
	inj := NewI(hc, s)

	b := container.ExposeRemoteBinding("http://localhost:8786/gotojs", "TestService", "GetParam", "", "Proxy", "GetParam")
	b.AddInjection(hc)
	b.AddInjection(s)
	ret := b.InvokeI(NewI())
	if rv, ok := ret.(float64); !ok || rv != 1000 {
		t.Errorf("Simple remote get call failed: %d", ret)
	}

	b = container.ExposeRemoteBinding("http://localhost:8786/gotojs", "TestService", "SetAndGetParam", "i", "Proxy", "SetAndGetParam")
	b.AddInjection(hc)
	b.AddInjection(s)
	ret = b.InvokeI(inj, 1001)
	if rv, ok := ret.(float64); !ok || rv != 1001 {
		t.Errorf("Simple remote get call failed: %d", ret)
	}

	resp, _ := http.Get("http://localhost:8786/gotojs/Proxy/SetAndGetParam/1000")
	by, _ := ioutil.ReadAll(resp.Body)
	if i, err := strconv.Atoi(string(by)); err != nil || i != 1000 {
		t.Errorf("Simple remote get call failed: %s,%d", err, i)
	}

	resp, _ = http.Get("http://localhost:8786/gotojs/Proxy/GetParam")
	by, _ = ioutil.ReadAll(resp.Body)
	if i, err := strconv.Atoi(string(by)); err != nil || i != 1000 {
		t.Errorf("Simple remote get call failed: %s,%d", err, i)
	}
}

func TestProxySession(t *testing.T) {
	ts := "TESTSTRING"
	container.ExposeFunction(func(s *Session, val string) { log.Printf("### set %s", val); s.Set("test", val) }, "SessionTest", "Set")
	container.ExposeFunction(func(s *Session) string { ret := s.Get("test"); log.Printf("###get %s", ret); return ret }, "SessionTest", "Get")
	container.ExposeRemoteBinding("http://localhost:8786/gotojs", "SessionTest", "Set", "s", "Proxy", "Set")
	container.ExposeRemoteBinding("http://localhost:8786/gotojs", "SessionTest", "Get", "", "Proxy", "Get")
	resp, _ := http.Get("http://localhost:8786/gotojs/Proxy/Set/" + ts)
	c := resp.Cookies()[0]
	log.Printf("%s", c)

	req, _ := http.NewRequest("GET", "http://localhost:8786/gotojs/Proxy/Get", nil)
	req.AddCookie(c)
	resp, _ = http.DefaultClient.Do(req)
	b, _ := ioutil.ReadAll(resp.Body)
	if string(b) != "\""+ts+"\"" { // is json encoded
		t.Errorf("Simple remote get call failed: '%s'/'%s'", string(b), ts)
	}
}

func TestProxyHeader(t *testing.T) {
	container.ExposeFunction(func(s *HTTPContext, hn string) string {
		ret := s.Request.Header.Get(hn)
		log.Printf("%s: %s", hn, ret)
		return ret
	}, "Echo", "Header")
	bu := "http://localhost:8786/gotojs"
	container.ExposeRemoteBinding(bu, "Echo", "Header", "s", "Proxy", "Header")
	resp, _ := http.Get("http://localhost:8786/gotojs/Proxy/Header/" + DefaultProxyHeader)
	b, _ := ioutil.ReadAll(resp.Body)
	if string(b) != "\""+bu+"\"" { // is json encoded
		t.Errorf("Proxy header not set: '%s'/'%s'", string(b), bu)
	}
}

func TestVarProxyHeader(t *testing.T) {
	hn := "x-proxy-test"
	hv := "1234"

	req, _ := http.NewRequest("GET", "http://localhost:8786/gotojs/Proxy/Header/"+hn, nil)
	req.Header.Set(hn, hv)
	resp, _ := http.DefaultClient.Do(req)
	b, _ := ioutil.ReadAll(resp.Body)
	if string(b) != "\""+hv+"\"" { // is json encoded
		t.Errorf("Proxy header not set: '%s'/'%s'", string(b), hv)
	}
}

func TestClient(t *testing.T) {
	c := NewClient("http://localhost:8786/gotojs")
	p, err := c.Invoke("TestService", "GetParam")

	if err != nil {
		t.Errorf("Client call failed: %s", err)
	}

	if pi, ok := p.(float64); !(ok || pi != 1000) {
		t.Errorf("Client call failed: %d/%d", int(pi), 1000)
	}
}

func TestParallelClients(t *testing.T) {
	ret := 0
	container.ExposeFunction(func() {
		ret++
		time.Sleep(time.Second * 5)
	}, "ServerTest", "Sleep")

	for i := 0; i < 5; i++ {
		go func() {
			c := NewClient("http://localhost:8786/gotojs")
			c.Invoke("ServerTest", "Sleep")
		}()
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(100 * time.Millisecond)

	if ret != 5 {
		t.Errorf("No parallel execution: %d/%d", ret, 5)
	}

	container.RemoveInterface("ServerTest")
}

func TestBindingUrl(t *testing.T) {
	b, _ := container.Binding("TestService", "GetParam")
	path := "/gotojs/TestService/GetParam"
	ev := "http://localhost:8786" + path
	url := b.Url()
	if url.String() != ev {
		t.Errorf("BindingUrl missmatch: '%s'/'%s'", url, ev)
	}

	if url.Path != path {
		t.Errorf("BindingUrl missmatch: '%s'/'%s'", url.Path, path)
	}
}

func TestObjectCall(t *testing.T) {
	type ts struct {
		A int
		B int
	}

	container.ExposeFunction(func(v ts) int {
		return v.A + v.B
	}, "Math", "AddS")

	container.ExposeFunction(func(v *ts) int {
		return v.A + v.B
	}, "Math", "AddP")

	out, err := executeJS(t, container, engineNodeJS, "PROXY.Math.AddS({a: 17, b: 4}, function(r) { if (r != 21) { throw 'Unexpected return value:' + r;}});")
	if err != nil {
		t.Errorf("Executing nodejs parser failed or error occured: %s", err.Error())
	}

	out, err = executeJS(t, container, engineNodeJS, "PROXY.Math.AddP({a: 16, b: 2}, function(r) { if (r != 18) { throw 'Unexpected return value:' + r;}});")
	if err != nil {
		t.Errorf("Executing nodejs parser failed or error occured: %s", err.Error())
	}

	t.Logf(out)
	container.RemoveInterface("Math")
}

type TestType struct{}

func (TestType) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b := []byte("\"OK\"")
	w.Write(b)
}

func TestExposeHandler(t *testing.T) {
	h := func(w http.ResponseWriter, r *http.Request) {
		b := []byte("\"OK\"")
		w.Write(b)
	}
	container.ExposeHandlerFunc(h, "H", "HANDLERFUNC")
	container.ExposeHandler(TestType{}, "H", "HANDLER")
	defer container.RemoveInterface("H")

	out, err := executeJS(t, container, engineNodeJS, "PROXY.H.HANDLERFUNC('asdasd','application/octet-stream',function(r) { if (r != \"OK\") { throw 'Unexpected return value:' + r;}});")
	if err != nil {
		t.Errorf("Could not invoke handler function: %s", err)
	}

	out, err = executeJS(t, container, engineNodeJS, "PROXY.H.HANDLER('asdasd','application/octet-stream',function(r) { if (r != \"OK\") { throw 'Unexpected return value:' + r;}});")

	if err != nil {
		t.Errorf("Could not invoke handler object: %s", err)
	}

	t.Logf(out)
}
