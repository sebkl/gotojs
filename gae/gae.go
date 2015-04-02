// Package gae offers a google appengine integration for the gotojs package.
package gae

import (
	. "github.com/sebkl/gotojs"
	"log"
	"appengine"
	"appengine/urlfetch"
	"net/http"
	"time"
)

// Google App Engine context wrapper.
type Context struct {
	appengine.Context
	Client *http.Client
	HTTPContext *HTTPContext
}

// ContextInjector is a gotojs injector method that takes care to add the GAE context
// to then injection vector and allows bindings to take it as injection argument.
func ContextInjector(b Binding,hc *HTTPContext, injs Injections) bool {
        if hc != nil {
                injs.Add(NewContext(hc))
        } else {
		log.Printf("No HTTPContext injected.")
	}
        return true
}

//NewContext creates a new appengine context wrapper by the given http call attributes.
func NewContext(hc *HTTPContext) (*Context){
	c := appengine.NewContext(hc.Request)
	client := urlfetch.Client(c)
	hc.Client = client
	if trans, ok := client.Transport.(*urlfetch.Transport); ok {
		trans.Deadline = 60*time.Second
		client.Transport = trans
	}
	return &Context{Context: c,Client: client, HTTPContext: hc}
}

// Writer to log on appengine info level.
func (g Context) Write(p []byte) (n int, err error) {
	g.Infof("%s",string(p))
	return len(p), nil
}

// ContextConstructor creates a gotos HTTPContext
func ContextConstructor(req *http.Request, res http.ResponseWriter) *HTTPContext {
	c := NewContext(NewHTTPContext(req,res))
	return c.HTTPContext
}

type ModuleController interface {
        Start(*Context)
        Stop(*Context)
}

type BaseModuleController struct {
	frontend *Frontend
	Next ModuleController
}

// Start is the menthod of the module controller that will be called when a
// appengine backend module is started (manual/basic_scaling)
func (con *BaseModuleController) Start(c *Context) {
	log.SetOutput(*c)
	http.DefaultClient = c.Client //TODO: This is ugly
	if con.Next != nil {
		con.Start(c)
	}
}

// Stop is the menthod of the module controller that will be called when a
// appengine backend module is stoped or aborted (manual/basic_scaling)
func (con *BaseModuleController) Stop(c *Context) {
	if con.Next != nil {
		con.Next.Stop(c)
	}
}

// ServeHTTP dispatches incoming requests to module controller methods (start/stop) and
// the gotojs handler.
func (con *BaseModuleController) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	hc := &HTTPContext{Request: req, Response: res}
	switch req.URL.Path {
		case "/_ah/start":
			con.Start(NewContext(hc))
		case "/_ah/stop":
			con.Stop(NewContext(hc))
		default:
			con.frontend.ServeHTTP(res,req)
	}
}

// NewBaseModuleController creates a generic module control that supoprts start/stop methods and integrates
// the gae specific context injections. The constructor should be called after the service bindings have been defined.
// A generic controller will be created and if given by parameter a subsequent one is chained in.
func NewBaseModuleController(f *Frontend, cons ...ModuleController) *BaseModuleController {
	ret := &BaseModuleController{frontend: f}
	if len(cons) > 0 {
		ret.Next = cons[0]
	}

	f.HTTPContextConstructor = ContextConstructor
	f.Bindings().
		AddInjection(&Context{}).
		If(AutoInjectF(ContextInjector))

	return ret
}


func SetupAndStart(f *Frontend,cons ...ModuleController) {
	mc :=NewBaseModuleController(f,cons...)
	f.Setup()
	http.Handle("/",mc)
}
