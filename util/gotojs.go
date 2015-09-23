package main

import (
	"encoding/json"
	"flag"
	"fmt"
	compilerapi "github.com/ant0ine/go-closure-compilerapi"
	"github.com/sebkl/flagconf"
	. "github.com/sebkl/gotojs"
	. "github.com/sebkl/gotojs/client"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
)

const (
	fflag = os.FileMode(0644)
	dflag = os.FileMode(0755)
)

func printUsage() {
	cmd := os.Args[0]
	fmt.Printf(`
Usage:
	%s <command> <mandatory_arguments> [optional_arguments]

The Commands are:
	example <path_to_app_root>		Create a sample app.
	create 	<path_to_app_root>		Create a sample directory structure.
	export 	<path_to_template_dir>		Exports internally used templates.
	compile <path_to_js_file> [output]	Compile javascript file.
	<interface_name>.<method_name> [args]   Invoke call of remote gotojs instance. The
						configuration is taken from the "GJHOST"
						environment variable. Default is:
						"http://localhost:8080/gotojs"

Examples:
	GJSHOST="http://somehost.com/gotojs" %s Trace.Echo
	%s --HOST="http://somehost.com/gotojs" Trace.Echo
	%s create /var/www/helloworld

`, cmd, cmd, cmd, cmd)
	flag.PrintDefaults()
}

var gotojsHost string

func init() {
	flag.StringVar(&gotojsHost, "HOST", "http://localhost:8080/gotojs", "GOTOJS endpoint to use.")
}

func check(e error) {
	if e != nil {
		printUsage()
		panic(e)
	}
}

func exportTemplates(path string) {
	fflag := os.FileMode(0644)

	temp := DefaultTemplates()

	for p, t := range temp {
		err := ioutil.WriteFile(path+"/"+p+"/"+HTTPTemplate, []byte(t.HTTP), fflag)
		check(err)
		err = ioutil.WriteFile(path+"/"+p+"/"+BindingTemplate, []byte(t.Binding), fflag)
		check(err)
		err = ioutil.WriteFile(path+"/"+p+"/"+InterfaceTemplate, []byte(t.Interface), fflag)
		check(err)
		err = ioutil.WriteFile(path+"/"+p+"/"+MethodTemplate, []byte(t.Method), fflag)
		check(err)
	}
}

func createBaseDirs(path string) {
	err := os.MkdirAll(path, dflag)
	check(err)
	err = os.MkdirAll(path+"/"+DefaultFileServerDir, dflag)
	check(err)
	err = os.MkdirAll(path+"/"+RelativeTemplatePath, dflag)
	check(err)
	for _, p := range Platforms {
		err = os.MkdirAll(path+"/"+RelativeTemplatePath+"/"+p, dflag)
		check(err)
		err = os.MkdirAll(path+"/"+RelativeTemplatePath+"/"+p+"/"+RelativeTemplateLibPath, dflag)
		check(err)
	}
}

func createSampleFiles(path string) {
	createBaseDirs(path)
	err := ioutil.WriteFile(path+"/"+DefaultFileServerDir+"/index.html", []byte(`
<!DOCTYPE HTML>
<html>
 <head>
  <title>gotojs example</title>
  <link type="text/css" href="css/main.css" rel="Stylesheet"/>
  <script src="/gotojs/"></script>
  <script src="/my.js"></script>
 </head>
 <body>
  <h1>Hello World !</h1>
 </body>
</html> `), fflag)
	check(err)
	err = os.MkdirAll(path+"/"+DefaultFileServerDir+"/css", dflag)
	check(err)
	err = os.MkdirAll(path+"/"+DefaultFileServerDir+"/js", dflag)
	check(err)
	err = ioutil.WriteFile(path+"/"+DefaultFileServerDir+"/css/main.css", []byte(`
h1{ font-family: sans-serif; color: #AAAAAA; }
`), fflag)
	check(err)

	err = ioutil.WriteFile(path+"/app.go", []byte(`
package main

import (
	"log"
	"fmt"
	"strings"
	. "github.com/sebkl/gotojs"
)

// Declare Service to be exposed.
type Service struct {}

// Methods of Service that will be exposed.
func (s *Service) UpperCase(mes string) string {
    return fmt.Sprintf("%s",strings.ToUpper(mes))
}

// Function that takes the HTTPContext as injection.
func AppendURL(context *HTTPContext, source string) string{
	return fmt.Sprintf("%s (%s)",source,context.Request.URL.String())
}

func main() {
	// Initialize the frontend.
	frontend := NewContainer()

	// Setup the service object.
	service := Service{}

        // Declare some js code that is doing the calls. Usually this is done by a flat file in the public directory,
        // but in this case we would like to show how to use HandleStatic.
        myjs := "$(document).ready(function() {"
        myjs += "       var text = $('h1').html();"
        myjs += "       text = GOTOJS.Service.UpperCase(text);" // Make the title uppcase by the server side implementation
        myjs += "       text = GOTOJS.Service.AppendURL(text);" // Append the URL by the server side  implementation
        myjs += "       $('h1').html(text);"
        myjs += "});"

	// Expose the interface and setup the request routing.
	frontend.ExposeInterface(&service,"Service")
	frontend.ExposeFunction(AppendURL,"Service","AppendURL") // Name the function and expose it to existing interface.
	frontend.EnableFileServer("public","p")
	frontend.Redirect("/","/p/")
	frontend.HandleStatic("/my.js",myjs,"application/javascript")
	log.Fatal(frontend.Start(":8080","/gotojs"))
}
`), fflag)
	check(err)
}

func main() {
	flagconf.Parse("GJS")
	al := len(os.Args)
	if al < 2 {
		printUsage()
		return
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "example":
		createSampleFiles(args[0])
		exportTemplates(args[0] + "/" + RelativeTemplatePath)
	case "create":
		createBaseDirs(args[0])
		exportTemplates(args[0] + "/" + RelativeTemplatePath)
	case "export":
		exportTemplates(args[0])
	case "compile":
		client := &compilerapi.Client{Language: "ECMASCRIPT5", CompilationLevel: "SIMPLE_OPTIMIZATIONS"}
		bs, err := ioutil.ReadFile(args[0])
		check(err)
		o := client.Compile(bs)

		if al > 3 {
			out := os.Args[3]
			ioutil.WriteFile(out, []byte(o.CompiledCode), fflag)
		} else {
			fmt.Println(o.CompiledCode)
		}

		//Log Errors and Warnings last.
		for _, v := range o.Warnings {
			fmt.Println(v.AsLogline())
		}

		for _, v := range o.Errors {
			fmt.Println(v.AsLogline())
		}
	default:
		r := regexp.MustCompile(`^(.*)\.(.*)$`)
		if r.MatchString(cmd) {
			sa := r.FindStringSubmatch(cmd)
			iname := sa[1]
			mname := sa[2]
			fmt.Printf("> %s.%s(%s) @ %s\n\n", iname, mname, strings.Join(args, ","), gotojsHost)
			c := NewClient(gotojsHost)
			ret, err := c.Invoke(iname, mname, SAToIA(args...)...)

			if err != nil {
				fmt.Printf("FAILED: %s", err)
				os.Exit(1)
			} else {
				by, _ := json.MarshalIndent(ret, "", "  ")
				fmt.Printf("%s", string(by))
			}
		} else {
			fmt.Printf("Unknown command: %s\n\n", cmd)
			printUsage()
		}
	}

}
