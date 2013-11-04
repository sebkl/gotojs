// Package gotojs offers a library for exposing go-interfaces as Javascript proxy objects.
// Therefore package gotojs assembles a JS engine which creates proxy objects as JS code 
// and forwards the calls to them via JSON encoded HTTP Ajax requests.
// This allows web developers to easily write HTML5 based application using jQuery,YUI and other 
// simalar frameworks without explictily dealing with ajax calls and RESTful server APIs but
// using a transparent RPC service.
//
// This service includes the follwing features:
//	- Injection of Objects (like a http context)
//	- Automatic include of external and internal libaries while the engine is loaded.
//	- Routing to internal fileserver that serves static content like images and html files.
package gotojs

import (
	"reflect"
	"text/template"
	"log"
	"path"
	"bytes"
	"io"
	"strings"
	"os"
	"net/http"
	"net/url"
	"time"
	"fmt"
	"encoding/json"
)

// Configuration flags.
const (
	F_CLEAR = 0
	F_LOAD_LIBRARIES = 1<<0
	F_LOAD_TEMPLATES = 1<<1
	F_VALIDATE_ARGS = 1<<2
	F_ENABLE_FILESERVER = 1<<3
	F_ENABLE_ACCESSLOG = 1<<4
	F_DEFAULT =	F_LOAD_LIBRARIES |
			F_LOAD_TEMPLATES |
			F_VALIDATE_ARGS |
			F_ENABLE_FILESERVER |
			F_ENABLE_ACCESSLOG
)

// Identifier of initialization parameter
const (
	P_BASEPATH = 0
	P_EXTERNALURL = 1
	P_NAMESPACE = 2
	P_PUBLICDIR = 3
	P_CONTEXT = 4
	P_LISTENADDR = 5
	P_PUBLICCONTEXT = 6
)


// Internally used constants and default values
const (
	RelativePath = "templates/js"
	RelativeLibPath = "templates/js/libs"
	Template= "binding.js"
	InterfaceTemplate= "interface.js"
	MethodTemplate= "method.js"
	DefaultNamespace = "PROXY"
	DefaultContext = "/jsproxy"
	//DefaultEnginePath = "_engine.js"
	DefaultListenAddress = "localhost:8080"
	DefaultFileServerDir = "public"
	DefaultFileServerContext = "public"
	DegaultExternalBaseURL = "http://" + DefaultListenAddress
	DefaultBasePath = "."

	tokenNamespace = "NS"
	tokenInterfaceName = "IN"
	tokenMethodName = "MN"
	tokenBaseContext = "BC"
	tokenArgumentsString = "AS"
	tokenValidateArguments = "MA"
	tokenBaseURL = "BU"
)

// Mapping of kind to char for method/function signature validation string.
var kindMapping = map[reflect.Kind]byte{
	reflect.Bool: 'i',
	reflect.Int: 'i',
	reflect.Int8: 'i',
	reflect.Int16: 'i',
	reflect.Int32: 'i',
	reflect.Int64: 'i',
	reflect.Uint: 'i',
	reflect.Uint8: 'i',
	reflect.Uint16: 'i',
	reflect.Uint32: 'i',
	reflect.Uint64: 'i',
	reflect.Uintptr: 'i',
	reflect.Float32: 'f',
	reflect.Float64: 'f',
	reflect.Complex64: 'f',
	reflect.Complex128: 'f',
	reflect.Array: 'a',
	reflect.Chan: '_',
	reflect.Func: '_',
	reflect.Interface: '_',
	reflect.Map: 'm',
	reflect.Ptr: 'i',
	reflect.Slice: 'a',
	reflect.String: 's',
	reflect.Struct: 'o',
	reflect.UnsafePointer: 'i' }

type incomingMessage struct {
	Interface,Method,CRID string
	Data []interface{}
}

type outgoingMessage struct {
	CRID string
	Data interface{}
}

type cache struct {
	engine,libraries string
	revision uint64
}

// The main frontend object to the "gotojs" bindings. It can be treated as a 
// HTTP facade of "gotojs".
type Frontend struct {
	Backend //inherit from Binding and extend
	template *template.Template
	namespace string
	context string
	extUrl *url.URL
	templateBasePath string
	flags int
	mux *http.ServeMux
	httpd *http.Server
	addr string
	cache cache
	publicDir string
	publicContext string
	fileServer http.Handler
}

// HTTPContext is a context object that will be injected by the frontend whenever an exposed method or function parameter
// is of type *HTTPContext. It contains references to all relevant http related objects like request and 
// response object.
type HTTPContext struct{
	Request *http.Request
	Response *http.ResponseWriter
}

// NewFrontend creates a new proxy frontend object. Required parameter are the configuration flags. Optional
// parameters are:
//	1) Namespace to be used
//	2) External URL, the system is reachable via
//	3) Base path, where to look for template and library subdirectories
//func NewFrontend(flags int,args ...string) (*Frontend){
func NewFrontend(flags int ,args map[int]string) (*Frontend){
	f := Frontend{
		Backend: NewBackend(),
		flags: flags,
		extUrl: nil,
		addr: DefaultListenAddress,
		templateBasePath: DefaultBasePath,
		namespace: DefaultNamespace,
		context: DefaultContext,
		publicDir: DefaultFileServerDir,
		publicContext: DefaultFileServerContext}

	for k,v:= range args {
		switch k {
			case P_EXTERNALURL:
				url,err := url.Parse(v)
				if err != nil {
					log.Fatalf("Could not parse external url: \"%s\".",args[1])
				}
				f.extUrl = url
				f.context = string(url.Path)
				f.addr = string(url.Host)
			case P_LISTENADDR:
				f.addr = v
			case P_BASEPATH:
				f.templateBasePath = v
			case P_PUBLICDIR:
				f.publicDir = v
			case P_CONTEXT:
				f.context = v
			case P_NAMESPACE:
				f.namespace = v
			case P_PUBLICCONTEXT:
				f.publicContext = v

		}
	}

	f.SetupInjection(&HTTPContext{})
	f.mux = http.NewServeMux();
	return &f
}


// Preload JS libraries if existing.
// TODO: An order needs to specified somehow.
// TODO: Simplify this crap
func (b *Frontend) loadLibraries() int{
	libbuf := new(bytes.Buffer)
	log.Printf("Loading default libraries ...")
	for _,u:= range defaultTemplates.Libraries {
		loadExternalLibrary(u,libbuf)
	}

	log.Printf("Searching for include JS libraries ...")
	fd,err := os.Open(b.templateBasePath + "/" + RelativeLibPath)
	if err == nil {
		fia,err := fd.Readdir(-1);
		if err == nil {
			for  _,fi := range fia {
				if fi.IsDir() {
					continue
				}
				elems := strings.Split(fi.Name(),".")
				suffix := elems[len(elems)-1]


				switch suffix {
					case "js":
						fd,err := os.Open(b.templateBasePath+"/"+RelativeLibPath+"/"+fi.Name());
						if err != nil {
							log.Printf("Could not open library file %s: %s",fi.Name(),err.Error());
							break
						}
						log.Printf("Reading JS library: %s",fi.Name());
						libbuf.ReadFrom(fd);
					case "url":
						fd,err := os.Open(b.templateBasePath+"/"+RelativeLibPath+"/"+fi.Name());
						if err != nil {
							log.Printf("Could not open library file %s: %s",fi.Name(),err.Error());
							break
						}
						log.Printf("Reading external JS library: %s",fi.Name());
						murl:= new(bytes.Buffer)
						murl.ReadFrom(fd);
						url,e := url.Parse(strings.TrimSpace(murl.String()))
						if e != nil {
							log.Printf("Could not parse url \"%s\".",url.String())
						} else {
							loadExternalLibrary(url.String(),libbuf)
						}
					default:
						log.Printf("Ignoring file: %s",fi.Name())
				}
			}

		} else {
			log.Println("Faild to retrieve directory info of library directory. Ignoring. %s",err.Error())
		}
	} else {
		log.Println("Failed to read libraries directory. Ignoring. %s",err.Error())
	}

	if bl:=libbuf.Len(); bl > 0 {
		b.cache.libraries = libbuf.String()
		return bl
	}
	return 0
}

// Load the contents of the given "url" and write it to the "out" writer.
func loadExternalLibrary(url string, out io.Writer) {
	log.Printf("Loading external JS library: %s",url)
	resp,e := http.Get(url)
	if e != nil {
		log.Printf("Could not load library %s: %s",url,e.Error())
		return
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	buf.WriteTo(out)
}

// Load the required templates ("binding.js", "interface.js" and "method.js") from the template directory.
// The template directory itself can be specified  by the NewFrontend constructor function. This
// only succeeds if all tempaltes can be loaded successfully. Otherwise the internal templates will
// be used.
func (b *Frontend) loadTemplatesFromDir() {
	ntemplate,e := template.ParseFiles(
		path.Join(b.templateBasePath,RelativePath,Template),
		path.Join(b.templateBasePath,RelativePath,InterfaceTemplate),
		path.Join(b.templateBasePath,RelativePath,MethodTemplate))
	if e!=nil {
		log.Printf("Could not load template \"%s\". Using default templates.",e.Error())
		b.loadDefaultTemplates()
	} else {
		for _,t:= range b.template.Templates() {
			log.Printf("Template found: %s.",t.Name())
		}
		b.template = ntemplate
	}
}



// Load internal default templates for "binding.js", "interface.js" and "method.js".
func (b *Frontend) loadDefaultTemplates() {
	t := template.New(Template)
	_,e1 := t.Parse(defaultTemplates.Binding)

	t = t.New(InterfaceTemplate)
	_,e2 := t.Parse(defaultTemplates.Interface)

	t = t.New(MethodTemplate)
	_,e3 := t.Parse(defaultTemplates.Method)

	if e1 != nil || e2 != nil || e3 != nil {
		log.Fatalf("Could not load internal templates: %s %s %s",e1.Error(),e2.Error(),e3.Error())
	}
	b.template = t
}

// ClearCache clears the internally used cache. This also includes the engine code which needs 
// to be reassembled afterwards. This happens on the next call that requests the engine.
func (b Frontend) ClearCache() {
	log.Printf("Clearing Cache at revision %d",b.cache.revision)
	b.cache = cache{}
}

// Flags gets and sets configuration flags. If method marameter are omitted, flags are just read from
// frontend object.
func (b *Frontend) Flags(flags ...int) int{
	if len(flags) > 0 {
		b.ClearCache()
		n := int(F_CLEAR)
		for _,f := range flags {
			n|=f
		}
		b.flags = n
	}
	return b.flags
}

// Build compiles the javascript based proxy-object (JS engine) including external libraries.
// This consists of 4 areas:
//	1:	Libraries (jQuery etc) which are read from fs during initialization.
//	2:	 Engine (binding engine) whose tempalte is read from fs during initialization.
//	3:	Interface objects whose template is read from ds during initialization.
//	4:	Methods per interface whose template is read from fs during initialization.
func (b *Frontend) Build(out io.Writer) {
	if  len(b.cache.engine) <= 0 || b.cache.revision < b.revision {
		buf := new(bytes.Buffer)
		b.build(buf)
		b.cache.engine = buf.String()
		b.cache.revision = b.revision
	}
	//io.WriteString(out,b.cache.engine)
	out.Write([]byte(b.cache.engine))
}

// Internally used function the actually generates the code for the JS engine. This includes
// also external and internal libariries if the corresponding configuration flag F_INCLUDE_LIBRARIES is set
// TODO: Split this in individual methods if feasible.
func (b *Frontend) build(out io.Writer) {
	log.Printf("Generating proxy object at revision %d for context: %s.",b.revision,b.context)
	// (1) Libraries
	if (b.flags & F_LOAD_LIBRARIES) > 0 && (len(b.cache.libraries) > 0 || b.loadLibraries() > 0) {
		io.WriteString(out,b.cache.libraries)
	}

	// (2)  Engine (binding engine)
	//TODO: Find a way to minimize templates while they are loaded.
	if (b.flags & F_LOAD_TEMPLATES) > 0 {
		b.loadTemplatesFromDir()
	} else {
		b.loadDefaultTemplates()
	}

	vav := ""
	if (b.flags & F_VALIDATE_ARGS) > 0 {
		vav="true"
	}
	proxyParams := map[string]string{
		tokenNamespace: b.namespace,
		tokenBaseContext: b.context,
		tokenValidateArguments: vav}

	if b.extUrl != nil {
		proxyParams[tokenBaseContext] = b.extUrl.String()
	}

	b.template.Lookup(Template).Execute(out,proxyParams)

	// (3) Interface objects
	interfaces:=b.Interfaces()
	for _,in := range interfaces{
		interfaceParams := Append(map[string]string{
			tokenInterfaceName: in }, proxyParams)

		b.template.Lookup(InterfaceTemplate).Execute(out,interfaceParams)

		// (4) Method objects
		methods := b.Binding.Methods(in)
		for _,m := range methods {
			methodParams := Append(map[string]string{
				tokenMethodName: m,
				tokenArgumentsString: b.ValidationString(in,m)},interfaceParams)
			b.template.Lookup(MethodTemplate).Execute(out,methodParams)

		}
	}
	return
}

// ValidationString generate a string that represents the signature of a method or function. It
// is used to perform a runtime validation when calling a JS proxy method.
func (b Binding) ValidationString(i,m string) (ret string){
	r:= b[i][m]
	t:=reflect.TypeOf(r.i)
	var methodType reflect.Type
	first := 0;
	if (r.methodNum >= 0) {
		methodType = t.Method(r.methodNum).Type
		first =1
	} else {
		methodType = t
	}
	argCount := methodType.NumIn();
	for n:=first;n < argCount;n++ {
		// If a injection is found for this parameter it will
		// be ignored in the validation string.
		if _,found := r.injections[n]; found {
			continue
		}

		at:= methodType.In(n)
		ret += string(kindMapping[at.Kind()])
	}
	return
}

//Context gets or sets the jsproxy path context. This path element defines
//how the engine code where the engine js code is served.
func (f *Frontend) Context(args ...string) string {
	al:=len(args)
	if (al > 0) {
		f.context = args[0]
		if !strings.HasPrefix(f.context,"/") {
			f.context = "/" + f.context
		}
	}
	return f.context
}

// Log incoming http requests.
func (f *Frontend) accessLog(w http.ResponseWriter,r *http.Request) {
	if (f.flags & F_ENABLE_ACCESSLOG) > 0 {
		log.Printf("[%s] %s ",r.Method,r.URL.Path)
	}
}

// Start starts the http frontend. This method only returns in case a panic or unexpected
// error occurs.
func (f *Frontend) Start(args ...string) error {
	al:=len(args)
	addr:=f.addr

	if (al > 0) {
		addr = args[0]
	}

	if (al > 1) {
		f.context = args[1]
	}

	// Setup file server handler.
	if f.flags & F_ENABLE_FILESERVER > 0 {
		if _,err := os.Stat(f.publicDir); err == nil {
			f.fileServer = http.StripPrefix("/"+f.publicContext+"/",http.FileServer(http.Dir(f.publicDir)))
			f.mux.HandleFunc("/" + f.publicContext + "/",func(w http.ResponseWriter,r *http.Request) {
				f.accessLog(w,r)
				f.fileServer.ServeHTTP(w,r)
			})
		} else {
			log.Printf("FileServer is enabled, but root dir \"%s\" does not exist or is not accessible.",f.publicDir)
		}
	}

	// Setup jsproxy engine handler.
	f.mux.Handle(f.context + "/",f)
	f.httpd = &http.Server{
		Addr:		addr,
		Handler:	f.mux,
		ReadTimeout:	1 * time.Second,
		WriteTimeout:	2 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Printf("Starting server at \"%s\".",addr)
	return f.httpd.ListenAndServe()
}

//Mux returns the internally user request multiplexer. It allows to assign additional http handlers
func (f* Frontend) Mux() *http.ServeMux {
	return f.mux
}

// Redriect is a convenience method which configures a redirect handler from the patter to adestination url.
func (f* Frontend) Redirect(pattern,url string) {
	f.mux.Handle(pattern,http.RedirectHandler(url,http.StatusFound))
}

//HandleStatic is a convenience method which defines a static content handle that is assigned to the given path pattern.
// It allows to declare statically served content like HTML or JS snippets.
// The Content-Type can be optionally specified.
func (f* Frontend) HandleStatic(pattern, content string, mime ...string) {
	f.mux.HandleFunc(pattern, func(w http.ResponseWriter,r *http.Request) {
		if len(mime) > 0 {
			w.Header().Set("Content-Type", mime[0])
		}
		w.Write([]byte(content))
	})
}

// ServeHTTP processes http request. Depending on the mehtod either a call is expected (POST) or 
// the JS engine is returned (GET)
func (f *Frontend) ServeHTTP(w http.ResponseWriter,r *http.Request) {
	defer func() {
		if re:=recover();re!=nil {
			http.Error(w,"",http.StatusInternalServerError)
			log.Printf("[%s][%s] %s FAILED: %s",r.Method,r.URL.Path,re)
		} else {
			f.accessLog(w,r)
		}

	}()

	switch r.Method {
		case "POST":
			w.Header().Set("Content-Type", "application/json")
			e := f.processCall(r.Body,w,&HTTPContext{Request: r,Response: &w})
			if e != nil{
				panic(e.Error())
			}
		case "GET":
			w.Header().Set("Content-Type", "application/javascript")
			f.Build(w)
	}
}

// Internally used method to process a call. Input parameters, interface name and method name a read from a JSON encoded
// input stream. The result is encoded to a JSON output stream.
func (f *Frontend) processCall(in io.Reader, out io.Writer,context *HTTPContext) (e error) {
	var b []byte
	var m incomingMessage
	defer func() {
		log.Printf("[CALL] %s.%s(%s) => %s",m.Interface,m.Method,"...",string(b))
	}()
	dec:=json.NewDecoder(in)
	if err := dec.Decode(&m); err != nil{
		return err
	}
	//ret := f.Invoke(m.Interface,m.Method,m.Data...)
	ret := f.InvokeI(m.Interface,m.Method,NewI(context),m.Data...)
	o := outgoingMessage{
		CRID: m.CRID,
		Data: ret}

	b, err := json.Marshal(o)
	if err != nil {
		fmt.Println("error:", err)
		return e
	}
	out.Write(b)
	return nil
}
