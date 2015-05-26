# gotojs
Package gotojs offers a library for **exposing go-interfaces** as **HTTP based RPC interface** and **JavaScript proxy objects**.
As a first step gotojs makes go-interfaces,function, attributes and handler implementations accessible as an HTTP based RPC-like API. In addition to that a JS engine is assembled which creates proxy objects as JS code and forwards its calls via JSON encoded HTTP Ajax requests. This allows web developers to easily write HTML5 based application using jQuery,YUI and other simalar frameworks without explictily dealing with ajax calls and RESTful server APIs but using a transparent RPC service.

## Usage
### Build an API

Define the Service as a ``struct`` and add the methods.
```go
import . "github.com/sebkl/gotojs"

type MyService struct {
}

func (s MyService) Echo(in string) string {
 return in
}
```

Initialize the gotojs engine and the service itself:
```go
co := NewContainer()
se := MyService{}
```
*The [container](http://godoc.org/github.com/sebkl/gotojs#BindingContainer) may be initialized with further parameters that affect its way working.*

Expose the service methods under a given name (context: `myservice`)
```go
co.ExposeInterface(se,"myservice")
```
*A wide range of further [exposure methods](http://godoc.org/github.com/sebkl/gotojs#BindingContainer) are supported such as exposing entire interfaces, sets of interface methods, single functions or just properties.*

Launch the server at web context `"/"` and listen on port `8080`:
```go
co.Start(":8080","/myapp")
```
*The [container](http://godoc.org/github.com/sebkl/gotojs#BindingContainer) itself implements the http handler interface `ServeHTTP` and thus can be easily integrated in existing environments*

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

### Further features
Expose static documents such as html and css files:
```go
fe.EnableFileServer("local/path/to/htdocs","/files")
```
*In this way it is supposed to make the Document root available which contains the main web-application files. *

Error handling:
```go
fe.ExposeFunction(func(hc *HTTPContext) { hc.Errorf(404,"To be Implemented") },"Service","TestError");
```

*More to be listed here*
* *More complex data structures and converters*
* *Filtering*
* *Injections*
* *Binary content*
* *Remote bindings*
* *Data streams*
* *Error handling*

### GO vs JS signatures
The bwlo map shows how the go-interfaces are being called using the JS proxy object.

| GO | JS | Description |
|-|-|-|
| ```func Foo(a,b int) (c int)``` | ```var c = GOTOJS.Service.Foo(a,b);``` | Synchronous call **(deprecated)**  |
| ```func Foo(a,b int) (c int)``` | ```GOTOJS.Service.Foo(a,b,function(c) { ... });``` |  Asynchronous call: The callback is always the last argument of the method call.|
| ```func Foo(postBody *BinaryContent) (b int)``` | ```GOTOJS.Service.Foo(postBody,mimetype,function(b) { ... });``` | Call with plain untouched post body data.|
| ```func Foo(w http.ResponseWriter, r *http.Request)``` | ```GOTOJS.Service.Foo(postBody,mimetype,function(w) { ... });``` | A handler function exposed as such, receives the transmitted data in the request object and replies via the response writer.|

## Examples

A list of more comprehensive examples can be found below:

1. Basic [function exposure](https://github.com/sebkl/gotojs/blob/master/example_test.go)
2. Basic [interface exposure](https://github.com/sebkl/gotojs/blob/master/example_interface_test.go)
3. A simple [web application](https://github.com/sebkl/gotojs/blob/master/example_fileserver_test.go)
4. Dealing with [sessions](https://github.com/sebkl/gotojs/blob/master/example_sessions_test.go)
5. Inline [exposures](https://github.com/sebkl/gotojs/blob/master/example_static_test.go)
6. [Handler](https://github.com/sebkl/gotojs/blob/master/example_binary_test.go) for binary POST requests
7. Standard [handler exposures](https://github.com/sebkl/gotojs/blob/master/example_handlerbinding_test.go)

*More to come...*

#### *Google App Engine* Example:
```go
package gaeexample

import (
	. "github.com/sebkl/gotojs"
	. "github.com/sebkl/gotojs/gae"
)

func init() {
	container := NewContainer()

	/* Define function that just returns all header of a incoming http request */
	f:= func (c *gae.Context) map[string][]string {
		/* Context is injected by the gae integration package. */
		return c.HTTPContext.Request.Header
	}

	/* Expose/bind function: */
	container.ExposeFunction(f,"EchoService","Header")

	SetupAndStart(container)
}
```
Which actually implements a simple HTTP Trace service (@ /gotojs/EchoService/Header) for demonstration purposes.

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
* node.js module request@2.55.0

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
└── jquery@1.8.3
#npm list request
└─┬ jquery@1.8.3
  └─┬ jsdom@0.2.19
    └── request@2.55.0
```

If these are not available, please try to install at least the above listed version. The nodejs stuff is necessary
to perform the go unit tests.
