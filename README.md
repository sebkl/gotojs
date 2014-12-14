# gotojs
Package gotojs offers a library for **exposing go-interfaces as JavaScript proxy objects**.
Therefore package gotojs assembles a JS engine which creates proxy objects as JS code and forwards the calls to them via JSON encoded HTTP Ajax requests. This allows web developers to easily write HTML5 based application using jQuery,YUI and other simalar frameworks without explictily dealing with ajax calls and RESTful server APIs but using a transparent RPC service.

## Usage
### Build an API

Define the Service as a ``struct`` and add the methods.
```go
type MyService struct {
}

func (s MyService) Echo(in string) string {
 return in
}
```

Initialize the gotojs engine and the service itself:
```go
fe := NewFrontend()
se := MyService{}
```
*The [frontend](http://godoc.org/github.com/sebkl/gotojs#Frontend) may be initialized with further parameters that affect its way working.*

Expose the service methods under a given name (context: `myservice`)
```go
fe.ExposeInterface(se,"myservice")
```
*A wide range of further [exposure methods](http://godoc.org/github.com/sebkl/gotojs#Frontend) are supported such as exposing entire interfaces, sets of interface methods, single functions or just properties.*

Launch the server at web context `"/"` and listen on port `8080`:
```go
fe.Start(":8080","/myapp")
```
*The [frontend](http://godoc.org/github.com/sebkl/gotojs#Frontend) itself implements the http handler interface `ServeHTTP` and thus can be easily integrated in existing environments*

### Client side
For accessing the JavasScript bindings, the engine has to be loaded first:
```html
<script src="/myapp/gotojs"></script>
```
Once loaded each exposed function can be directly invoked in the JavaScript code, whereby the last argument is always the asynchronous data handler callback:
```javascript
GOTOJS.myservice.Echo("Hello World!",function(d) { console.log(d); })
```

Each function or method is actually exposed as `/context/interface/method`:
```
#> curl "http://localhost:8080/myapp/myservice/Echo?p=Hello"
"Hello"

#> curl "http://localhost:8080/myapp/myservice/Echo/World"
"World"
```

A golang based client implementation:
```go
client := NewClient("http://localhost:8080/myapp")
ret,err := client.Invoke("myservice","Echo","Hello World!")
```

### Further Features
Expose static documents such as html and css files:
```go
fe.EnableFileServer("local/path/to/htdocs","/files")
```
*This can be used to expose also the web application frontend.*


*More to be listed here: *
* *More complex data structures and converters*
* *Filtering*
* *Injections*
* *Binary content*
* *Remote bindings*
* *Data streams*
* *Error handling*


## Examples

A list of more comprehensive examples can be found below:

1. Basic [function exposure](https://github.com/sebkl/gotojs/blob/master/example_test.go)
2. Basic [interface exposure](https://github.com/sebkl/gotojs/blob/master/example_interface_test.go)
3. A simple [web application](https://github.com/sebkl/gotojs/blob/master/example_fileserver_test.go)
4. Dealing with [sessions](https://github.com/sebkl/gotojs/blob/master/example_sessions_test.go)
4. Inline [exposures](https://github.com/sebkl/gotojs/blob/master/example_static_test.go)

*More to come...*

#### *Google App Engine* Example:
```go
package frontend

import (
	. "github.com/sebkl/gotojs"
	"net/http"
	"appengine"
	"appengine/urlfetch"
)

//Define the serve "Trace" with one method: "Echo"
type Trace struct {
}
func (t *Trace) Echo(ac *GAEContext,c *HTTPContext) (ret map[string]interface{}) {
	ret = make(map[string]interface{})
	ret["Header"] = c.Request.Header
	ret["Source"] = c.Request.RemoteAddr
	ret["GAEAppId"] = appengine.AppID(*ac)
	ac.Debugf("Echo called.")
	return
}

// Google App Engine context wrapper.
type GAEContext struct {appengine.Context}

// Make sure the context is always injected.
func GAEContextInjector(b *Binding,hc *HTTPContext, injs Injections) bool {
	if hc != nil {
		c := appengine.NewContext(hc.Request)
		injs.Add(&GAEContext{c})
	}
	return true
}

func init() {
	frontend := NewFrontend()
	frontend.HTTPContextConstructor = func(req *http.Request, res http.ResponseWriter) *HTTPContext {
		ret := NewHTTPContext(req,res)
		c := appengine.NewContext(req)
		ret.Client = urlfetch.Client(c) /* Allows us to specify our own http.Client impl. */
		return ret
	}

	frontend.ExposeInterface(&Trace{}).
		AddInjection(&GAEContext{}).
		If(AutoInjectF(GAEContextInjector))

	handler := frontend.Setup() // Returns just the http handler without starting a standalone server.
	http.Handle("/", handler)
}
```
Which actually implements a simple HTTP Trace service for demonstration purposes.

#### Generate an application base:
For a quick example application including predefined templates and javascript libraries a builtin tool may be used:
```
go get github.com/sebkl/gotojs/util
${GOPATH}/bin/util example ${GOPATH}/www
```

## Requirements
Gotojs requires
* go version >= 1.3
since some reflection features are used from package `reflect` that have been added in 1.3
Please keep in mind, that the example application `${GOPATH}/www/app.go` is intended to show some basic features.

## Documentation
The go documentation of the gotojs package is complete and includes some additional examples for various use cases.
It can be found [at godoc.org](http://godoc.org/github.com/sebkl/gotojs).

## Development
For development purposes more dependencies are required due to nodejs unit testing:
* node.js version v0.10.20
* npm version 1.3.11
* node.js module jquery@1.8.3

If not happend so far create your go environment and set the GOPATH environment variable accordingly:
```
#mkdir ~/go
#export GOPATH=~/go

```

Check your go, nodejs and npm versions:
```
#go version
go version go1.3 linux/amd64
#node --version
v0.10.20
#npm --version
1.3.11
#npm list jquery@1.8.3
/home/sk
└── jquery@1.8.3
```

If these are not available, please try to install at least the above listed version. The nodejs stuff is necessary
to perform the go unit tests.
