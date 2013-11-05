package gotojs_test

import (
	"testing"
	"os/exec"
	"bytes"
	"fmt"
	"errors"
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
	frontend = NewFrontend(F_VALIDATE_ARGS | F_ENABLE_FILESERVER,map[int]string{
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
	vs := frontend.ValidationString("TestService","SetAndGetParam")
	t.Logf("Validation string for \"%s.%s\" is : %s.","TestService","SetAndGetParam",vs)
	if vs != "i" {
		t.Errorf("Incorrect validation string: %s",vs)
	}
	frontend.ExposeFunction( func (i int, o struct{}, f float32, s string, sa []string) int {
		t.Logf("%s,%s,%s,%s,%s",i,o,f,s,sa)
		return 0
	},"X","test")
	vs = frontend.ValidationString("X","test")
	t.Logf("Validation string for \"%s.%s\" is : %s.","X","test",vs)
	if vs != "iofsa" {
		t.Errorf("Incorrect validation string: %s",vs)
	}
}

func TestRemoveInterface(t *testing.T) {
	pi := frontend.Interface("X")
	if pi == nil {
		t.Errorf("Interface \"%s\" did not exist.","X")
	}
	frontend.RemoveInterface("X")
	ia := frontend.Interfaces()
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

