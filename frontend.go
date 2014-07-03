// Package gotojs offers a library for exposing go-interfaces as Javascript proxy objects.
// Therefore package gotojs assembles a JS engine which creates proxy objects as JS code 
// and forwards the calls to them via JSON encoded HTTP Ajax requests.
// This allows web developers to easily write HTML5 based application using jQuery,YUI and other 
// simalar frameworks without explictily dealing with ajax calls and RESTful server APIs but
// using a transparent RPC service.
//
// This service includes the follwing features:
//
// - Injection of Objects (like a session or http context)      
//
// - Automatic include of external and internal libaries while the engine is loaded.    
//
// - Routing to internal fileserver that serves static content like images and html files.   
package gotojs

import (
	"errors"
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
	"encoding/base64"
	"strconv"
	"compress/flate"
)

// Configuration flags.
const (
	F_CLEAR = 0
	F_LOAD_LIBRARIES = 1<<0
	F_LOAD_TEMPLATES = 1<<1
	F_VALIDATE_ARGS = 1<<2
	F_ENABLE_ACCESSLOG = 1<<3
	F_DEFAULT =	F_LOAD_LIBRARIES |
			F_LOAD_TEMPLATES |
			F_VALIDATE_ARGS |
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
	P_APPLICATIONKEY = 7
	P_FLAGS = 8
)

// Internally used constants and default values
const (
	RelativePath = "templates/js"
	RelativeLibPath = "templates/js/libs"
	Template= "binding.js"
	InterfaceTemplate= "interface.js"
	MethodTemplate= "method.js"
	DefaultNamespace = "GOTOJS"
	DefaultContext = "/gotojs"
	//DefaultEnginePath = "_engine.js"
	DefaultListenAddress = "localhost:8080"
	DefaultFileServerDir = "public"
	DefaultFileServerContext = "public"
	DefaultExternalBaseURL = "http://" + DefaultListenAddress
	DefaultBasePath = "."
	DefaultCookieName = "gotojs"
	DefaultCookiePath = "/gotojs"

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
	reflect.Complex64: '_',
	reflect.Complex128: '_',
	reflect.Array: 'a',
	reflect.Chan: '_',
	reflect.Func: '_',
	reflect.Interface: 'o',
	reflect.Map: 'm',
	reflect.Ptr: 'i',
	reflect.Slice: 'a',
	reflect.String: 's',
	reflect.Struct: 'o',
	reflect.UnsafePointer: 'i' }

type call struct {
	Interface,Method,CRID string
	Data []interface{}
}

type outgoingMessage struct {
	CRID string
	Data interface{}
}

type errorMessage struct {
	Error string
}

type cache struct {
	engine,libraries string
	revision uint64
}

// The main frontend object to the "gotojs" bindings. It can be treated as a 
// HTTP facade of "gotojs".
type Frontend struct {
	Backend //inherit from BindingContainer and extend
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
	key []byte
}

//Cookie encoder. Standard encoder uses "=" symbol which is not allowed for cookies.
var encoding = base64.NewEncoding("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_")

//Parameter type allows to define mutliple configuration parameters.
type Parameters map[int]string

//Properties are generic string string maps used for a user session.
type Properties map[string]string

// Session represents a users session on server side. Basically
// it consists of a set of properties.
type Session struct {
	Properties
	dirty bool
}

//Flag2Param converts initialization flags to a string parameter.
func Flag2Param(flag int) string{
	return fmt.Sprintf("%d",flag)
}

// NewSession creates an empty fresh session object.
func NewSession() *Session {
	return &Session{
		Properties: make(Properties),
		dirty: false}
}

// SessionFromCookie reads a session object from the cookie.
func SessionFromCookie(cookie *http.Cookie,key []byte) *Session{
	// Base64 decode
	raw,err := encoding.DecodeString(cookie.Value)
	if err != nil {
		panic(errors.New(fmt.Sprintf("Could not decode (base64) session: %s",err.Error())))
	}

	// Decrypt
	ibuf := bytes.NewBuffer(Decrypt(raw,key))

	// Enflate
	fbuf := new(bytes.Buffer)
	r := flate.NewReader(ibuf)
	fbuf.ReadFrom(r)

	// JSON Decoder
	dec := json.NewDecoder(fbuf)
	s := NewSession()
	s.dirty = false
	if err = dec.Decode(&s.Properties);err != nil {
		panic(errors.New(fmt.Sprintf("Could not decode (json) session: %s/%s",fbuf.String(),err.Error())))
	}
	return s
}

// Set sets a property value with the given key.
func (s *Session) Set(key,val string) {
	s.dirty = true
	s.Properties[key] = val
}

// Get returns the named property value if existing. If not nil is
// returned.
func (s *Session) Get(key string) string{
	return s.Properties[key]
}

// Flush updates the cookie on client side if it was changed.
// In order to do so it sets the "Set-Cookie" header on the http
// response
func (s *Session) Flush(w http.ResponseWriter,key []byte) {
	if s.dirty {
		http.SetCookie(w,s.Cookie(DefaultCookieName,DefaultCookiePath,key))
	}
}

// Cookie generates a cookie object with the given name and path.
// the cookie value is taken from the session properties, json encoded, defalted, encrypted with the given key and finally base64 encoded.
func (s *Session) Cookie(name,path string, key []byte) *http.Cookie {
	c := new(http.Cookie);

	//JSON Encoding:
	b ,err := json.Marshal(s.Properties)
	if err != nil {
		panic(fmt.Errorf("Cannot compile cookie: %s",err.Error()))
	}

	//Deflate:
	fbuf := new(bytes.Buffer)
	fw,err := flate.NewWriter(fbuf,flate.BestCompression)
	if err != nil {
		panic(fmt.Errorf("Could not initialized compressor."))
	}

	if _,err := fw.Write(b);err!=nil {
		panic(fmt.Errorf("Could not defalte content."))
	}
	fw.Flush()

	// Encrypt and base64 encoding:
	c.Name = name
	c.Value = encoding.EncodeToString(Encrypt(fbuf.Bytes(),key))
	c.Path = path
	return c
}

// HTTPContext is a context object that will be injected by the frontend whenever an exposed method or function parameter
// is of type *HTTPContext. It contains references to all relevant http related objects like request and 
// response object.
type HTTPContext struct{
	Request *http.Request
	Response http.ResponseWriter
}

// Session tries to extract a session from the HTTPContext.
// If it cannot be extracted, a new session is created.
func (c *HTTPContext) Session(key []byte) (s *Session){
	defer func() {
		if r := recover(); r != nil {
		    // If something happens ... return fresh session
		    log.Printf("%s. Creating fresh session.",r)
		    s = NewSession()
		}
	}()
	cookie,err := c.Request.Cookie(DefaultCookieName)
	if err != nil {
		s = NewSession()
		//panic("No Cookie")
	} else {
		s = SessionFromCookie(cookie,key)
	}
	return s
}

// NewFrontend creates a new proxy frontend object. Required parameter are the configuration flags. Optional
// parameters are:
//
// 1) Namespace to be used
//
// 2) External URL the system is accessed
//
// 3) The base path where to look for template and library subdirectories
//func NewFrontend(flags int,args ...string) (*Frontend){
func NewFrontend(args ...Parameters) (*Frontend){
	f := Frontend{
		Backend: NewBackend(),
		flags: F_DEFAULT,
		extUrl: nil,
		addr: DefaultListenAddress,
		templateBasePath: DefaultBasePath,
		namespace: DefaultNamespace,
		context: DefaultContext,
		publicDir: DefaultFileServerDir,
		key: GenerateKey(16),
		publicContext: DefaultFileServerContext}

	if len(args) > 0 {
		for k,v:= range args[0] {
			switch k {
				case P_EXTERNALURL:
					url,err := url.Parse(v)
					if err != nil {
						panic(fmt.Errorf("Could not parse external url: \"%s\".",args[1]))
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
				case P_APPLICATIONKEY:
					f.key = []byte(v)
				case P_FLAGS:
					if iv,err := strconv.Atoi(v); err != nil {
						panic(fmt.Errorf("Could not parse initialization flags: %s",err.Error()))
					} else {
						f.flags = iv
					}
			}
		}
	}

	f.SetupGlobalInjection(&HTTPContext{})
	f.SetupGlobalInjection(&Session{})
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
			log.Printf("Faild to retrieve directory info of library directory. Ignoring. %s",err.Error())
		}
	} else {
		log.Printf("Failed to read libraries directory. Ignoring. %s",err.Error())
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

		for _,t:= range ntemplate.Templates() {
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
		panic(fmt.Errorf("Could not load internal templates: %s %s %s",e1.Error(),e2.Error(),e3.Error()))
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
//
// 1:	Libraries (jQuery etc) which are read from either the file system or the interned during initialization.
//
// 2:	Engine (binding engine).
//
// 3:	Interface objects.
//
// 4:	Methods per interface.
//
// Templates for 2),3),4) are either taken from application memory or read from the filesystem.
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
	interfaces:=b.InterfaceNames()
	for _,in := range interfaces{
		interfaceParams := Append(map[string]string{
			tokenInterfaceName: in }, proxyParams)

		b.template.Lookup(InterfaceTemplate).Execute(out,interfaceParams)

		// (4) Method objects
		methods := b.BindingContainer.BindingNames(in)
		for _,m := range methods {
			bi,_ := b.Binding(in,m)
			vs:= bi.ValidationString()
			methodParams := Append(map[string]string{
				tokenMethodName: m,
				tokenArgumentsString: vs},interfaceParams)
			b.template.Lookup(MethodTemplate).Execute(out,methodParams)

		}
	}
	return
}

// ValidationString generate a string that represents the signature of a method or function. It
// is used to perform a runtime validation when calling a JS proxy method.
func (r *Binding) ValidationString() (ret string){
	t:=reflect.TypeOf(r.i)
	var methodType reflect.Type
	first := 0;
	if (r.elemNum >= 0) {
		methodType = t.Method(r.elemNum).Type
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

//Context gets or sets the gotojs path context. This path element defines
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

// EnableFileServer configures the file server and assigns the rooutes to the Multiplexer.
func (f *Frontend) EnableFileServer(args ...string) {
	al:=len(args)
	if al > 0 {
		f.publicDir = args[0]
	}

	if al > 1 {
		f.publicContext = args[1]
	}

	if _,err := os.Stat(f.publicDir); err == nil {
		f.fileServer = http.StripPrefix("/"+f.publicContext+"/",http.FileServer(http.Dir(f.publicDir)))
		f.mux.Handle("/" + f.publicContext + "/",f.fileServer)
	} else {
		log.Printf("FileServer is enabled, but root dir \"%s\" does not exist or is not accessible.",f.publicDir)
	}
}

// LogWraper type acts as a http handler that wrapps any other Muxer
// or Handler
type LogWraper struct{
	handler http.Handler
}

// ServeHTTP is a httpn handler method and wraps the origin one of 
// LogMuxer
func (lm *LogWraper) ServeHTTP(w http.ResponseWriter,r *http.Request)  {
	t := time.Now()
	defer Log(r.Method,strconv.FormatInt(time.Since(t).Nanoseconds() / (1000),10),r.URL.Path)
	lm.handler.ServeHTTP(w,r)
}

//NewLogWraper creates a new LogMuxer, that wraps the given http 
// handler. See LogMuxer for more details.
func NewLogWraper(origin http.Handler) *LogWraper {
	return &LogWraper{origin}
}

// Start starts the http frontend. This method only returns in case a panic or unexpected
// error occurs.
// 1st optional parameter is the listen address ("localhost:8080") and 2nd optional parmaeter is
// the engine context ("/gotojs")
// If these are not provided, default or initialization values are used
func (f *Frontend) Start(args ...string) error {
	al:=len(args)

	if (al > 0) {
		f.addr = args[0]
	}

	if (al > 1) {
		f.context = args[1]
	}

	// Setup gotojs engine handler.
	f.mux.Handle(f.context + "/",f)

	//final http handler
	var handler http.Handler
	if f.flags & F_ENABLE_ACCESSLOG  > 0{
		handler = NewLogWraper(f.mux)
	} else {
		handler = f.mux
	}

	f.httpd = &http.Server{
		Addr:		f.addr,
		Handler:	handler,
		ReadTimeout:	5 * time.Second,
		WriteTimeout:	10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Printf("Starting server at \"%s\".",f.addr)
	return f.httpd.ListenAndServe()
}


//Mux returns the internally user request multiplexer. It allows to assign additional http handlers
func (f* Frontend) Mux() *http.ServeMux {
	return f.mux
}

// Redirect is a convenience method which configures a redirect handler from the patter to adestination url.
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

// ErrorToJSON translate a go error into a JSON object.
func ErrorToJSONString(e error) string {
	mes := "unknown"
	if e != nil {
		mes = e.Error()
	}
	b,_:= json.Marshal(errorMessage{Error: mes })
	buf := new(bytes.Buffer)
	buf.Write(b)
	return buf.String()
}

// ServeHTTP processes http request. Depending on the mehtod either a call is expected (POST) or 
// the JS engine is returned (GET)
func(f *Frontend) ServeHTTP(w http.ResponseWriter,r *http.Request) {
	defer func() {
		if re:=recover();re!=nil {
			e,_ :=re.(error)
			http.Error(w,ErrorToJSONString(e),http.StatusInternalServerError)
		}
		r.Body.Close()
	}()

	httpContext := &HTTPContext{Request: r,Response: w}
	session := httpContext.Session(f.key)
	obuf := new(bytes.Buffer)

	defer func() {
		session.Flush(w,f.key) //Update session on client side if necessary.
		obuf.WriteTo(w)
	}()

	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
		case "POST":
			w.Header().Set("Content-Type", "application/json")
			dec:=json.NewDecoder(r.Body)
			var m call
			if e := dec.Decode(&m); e != nil{
				panic(e)
			}

			if b,found := f.Binding(m.Interface,m.Method); found {
				if len(b.ValidationString()) != len(m.Data) {
					http.Error(w,ErrorToJSONString(fmt.Errorf("Invalid parameter count: %d/%d",len(m.Data),len(b.ValidationString()))),http.StatusBadRequest)
				} else {
					b.processCall(obuf,NewI(httpContext,session),m.CRID,m.Data...)
				}
			} else {
				http.Error(w,ErrorToJSONString(errors.New(fmt.Sprint("Binding %s.%s not found.",m.Interface,m.Method))),http.StatusNotFound)
				return
			}
		case "GET":
			path := r.URL.Path
			if strings.Contains(path,f.context) {
				sub:= strings.SplitAfterN(path,f.context,2)
				elems := strings.Split(sub[1],"/")

				if len(elems) > 0 {
					elems = elems[1:]
				}

				if len(elems) >= 2 {
					if b,f := f.Binding(elems[0],elems[1]); f {
						if len(b.ValidationString()) != 0 {
							http.Error(w,ErrorToJSONString(fmt.Errorf("Invalid parameter count: %d/%d",0,len(b.ValidationString()))),http.StatusBadRequest)
						} else {
							b.processCall(obuf,NewI(httpContext,session),"")
						}
					} else {
						http.Error(w,ErrorToJSONString(errors.New(fmt.Sprintf("Binding %s.%s not found.",elems[0],elems[1]))),http.StatusNotFound)
					}
					return
				}
			}
			w.Header().Set("Content-Type", "application/javascript")
			f.Build(obuf)
		default:
			http.Error(w,"Unsupported Method",http.StatusMethodNotAllowed)
	}
}

// Internally used method to process a call. Input parameters, interface name and method name a read from a JSON encoded
// input stream. The result is encoded to a JSON output stream.
func (f *Binding) processCall(out io.Writer,injs Injections,id string,args ...interface{}) {
	var b []byte
	defer func() {Log("CALL","-",f.interfaceName + "." + f.elemName,strconv.Itoa(len(b))) }()
	ret := f.InvokeI(injs,args...)
	o := outgoingMessage{
		CRID: id,
		Data: ret}

	b, err := json.Marshal(o)
	if err != nil {
		panic(err)
	}
	out.Write(b)
	return
}
