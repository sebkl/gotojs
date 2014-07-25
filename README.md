# gotojs
Package gotojs offers a library for **exposing go-interfaces as Javascript proxy objects**.   
Therefore package gotojs assembles a JS engine which creates proxy objects as JS code and forwards the calls to them via JSON encoded HTTP Ajax requests. This allows web developers to easily write HTML5 based application using jQuery,YUI and other simalar frameworks without explictily dealing with ajax calls and RESTful server APIs but using a transparent RPC service.

## Requirements
* go version 1.3

## Getting started:
```
go get github.com/sebkl/gotojs
```

Once installed a simple service "hello.go" can exposed as follows:
```
package main

import(
        . "github.com/sebkl/gotojs"
)

func main() {
        fe := NewFrontend()
        fe.ExposeFunction(func (name string) string { return "Hello " + name + "!" },"Sample","Hello")
        fe.Start(":8080")
}
```
Run the service:
```
go run hello.go
```
And query it as follows:
```
curl "http://localhost:8080/gotojs/Sample/Hello?name=Dude"
{"CRID":"","Data":"Hello Dude!"}
```

#### Google App Engine Example:
```
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

#### Generate application base:
For a quick example application follow these steps
```
go get github.com/sebkl/gotojs/util
${GOPATH}/bin/util example ${GOPATH}/www
```

Please keep in mind, that the example application `${GOPATH}/www/app.go` is intended to show some features. After playing around with it you should create your own !


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

