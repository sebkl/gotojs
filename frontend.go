// Package gotojs offers a library for exposing go-interfaces as Javascript proxy objects.
// Therefore gotojs assembles a JS engine which creates proxy objects as JS code 
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
	"io/ioutil"
	"runtime/debug"
	"github.com/dchest/jsmin"
	. "github.com/sebkl/gotojs/client"
	compilerapi "github.com/sebkl/go-closure-compilerapi"
)

// Configuration flags.
const (
	F_CLEAR = 0
	F_LOAD_LIBRARIES =	1<<iota
	F_LOAD_TEMPLATES =	1<<iota
	F_VALIDATE_ARGS =	1<<iota
	F_ENABLE_ACCESSLOG =	1<<iota
	F_ENABLE_MINIFY =	1<<iota
	F_DEFAULT =	F_LOAD_LIBRARIES |
			F_LOAD_TEMPLATES |
			F_ENABLE_ACCESSLOG |
			F_ENABLE_MINIFY

	F_DEVELOPMENT =	F_LOAD_LIBRARIES |
			F_LOAD_TEMPLATES |
			F_VALIDATE_ARGS |
			F_ENABLE_ACCESSLOG
)

// Identifier of initialization parameter
const (
	P_BASEPATH	= "basepath"
	P_EXTERNALURL	= "eternalurl"
	P_NAMESPACE	= "namespace"
	P_PUBLICDIR	= "pubdir"
	P_CONTEXT	= "context"
	P_LISTENADDR	= "addr"
	P_PUBLICCONTEXT = "pubcontext"
	P_APPLICATIONKEY = "appkey"
	P_FLAGS		= "flags"
	P_COOKIENAME	= "cookie"
)

// Internally used constants and default values
const (
	RelativeTemplatePath = "templates"
	RelativeTemplateLibPath = "libs"
	HTTPTemplate = "http.js"
	BindingTemplate= "binding.js"
	InterfaceTemplate= "interface.js"
	MethodTemplate= "method.js"
	CTHeader = "Content-Type"
	DefaultNamespace = "GOTOJS"
	DefaultContext = "/gotojs/"
	//DefaultEnginePath = "_engine.js"
	DefaultListenAddress = "localhost:8080"
	DefaultFileServerDir = "public"
	DefaultFileServerContext = "public"
	DefaultExternalBaseURL = "http://" + DefaultListenAddress
	DefaultBasePath = "."
	DefaultCookieName = "gotojs"
	DefaultCookiePath = "/gotojs"
	DefaultPlatform = "web"
	DefaultMimeType = "application/json"
	DefaultHeaderCRID = "x-gotojs-crid"
	DefaultHeaderError = "x-gotojs-error"
	DefaultProxyHeader = "x-gotojs-proxy"
	DefaultCRID = "undefined"

	tokenNamespace = "NS"
	tokenInterfaceName = "IN"
	tokenMethodName = "MN"
	tokenBaseContext = "BC"
	tokenArgumentsString = "AS"
	tokenValidateArguments = "MA"
	tokenHttpMethod = "ME"
	tokenHasBinary = "BIN"
	tokenHeaderCRID = "IH"
	tokenContentType = "CT"
	tokenCRIDLength = "CL"
)

type cache struct {
	engine string
	libraries string
	revision uint64
}

// Return interface which allows to return binary content with a specific mime type 
// non json encoded
type Binary interface {
	io.ReadCloser
	MimeType() string
}

//BinaryContent is an internal implementation to wrap a POST call as a Binary interface
type BinaryContent struct { *http.Request }

func (b *BinaryContent) MimeType() string {
	return b.Request.Header.Get(CTHeader)
}

func (b *BinaryContent) Read(p []byte) (n int, err error) {
	return b.Request.Body.Read(p)
}

func (b *BinaryContent) Close() error {
	return b.Request.Body.Close()
}

func NewBinaryContent(req *http.Request) (ret *BinaryContent) {
	if req.Body != nil {
		ret = &BinaryContent{req}
	}
	return
}

type HTTPContextConstructor func(*http.Request,http.ResponseWriter) *HTTPContext

// NewHTTPContext creates a new HTTP context based on incoming 
func NewHTTPContext(request *http.Request, response http.ResponseWriter) *HTTPContext {
	return &HTTPContext{
		Request: request,
		Response: response,
		Client: http.DefaultClient,
		ErrorStatus: http.StatusInternalServerError,
		ReturnStatus: http.StatusOK,
	}
}

// Errorf sets the current HTTP context into an error state with the given status code
// and formated error message.
func (c *HTTPContext) Errorf(status int ,f string, args ...interface{})  {
	c.ErrorStatus = status
	hv :=fmt.Sprintf(f,args...)
	c.Response.Header().Set(DefaultHeaderError,hv)
	//log.Printf("%d: %s",status,hv)
	panic(hv)
}

// The main frontend object to the "gotojs" bindings. It can be treated as a 
// HTTP facade of "gotojs".
type Frontend struct {
	backend //embedd BindingContainer and extend
	*http.ServeMux //embedd http muxer
	templateSource Templates
	template map[string]*template.Template
	namespace string
	context string
	extUrl *url.URL
	templateBasePath string
	flags int
	httpd *http.Server
	addr string
	cache map[string]*cache
	publicDir string
	publicContext string
	fileServer http.Handler
	key []byte //key used to enctrypt the cookie.
	HTTPContextConstructor HTTPContextConstructor
}

//Cookie encoder. Standard encoder uses "=" symbol which is not allowed for cookies.
var Encoding = base64.NewEncoding("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_+")

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
	raw,err := Encoding.DecodeString(cookie.Value)
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

// Delete deletes the named property value if existing.
func (s *Session) Delete(key string) {
	delete(s.Properties,key)
	s.dirty = true
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
	c.Value = Encoding.EncodeToString(Encrypt(fbuf.Bytes(),key))
	c.Path = path
	return c
}

//SetCookies wraps the cookies into this session. This can effectively be used
// as cookie proxy.
func (s *Session) SetCookies(u *url.URL, cookies []*http.Cookie) {
	for _,c := range cookies {
		//TODO: encapsulate more information
		//log.Printf("Found Cookie: %s=%s",c.Name,c.Value)
		s.Set(c.Name,c.Value)
	}
}

//Cookies returns alls cookies that belong to this url. This can effectively be used 
// as cookie proxy.
func (s *Session) Cookies(u *url.URL) []*http.Cookie {
	ret := make([]*http.Cookie,0)
	for k,v := range s.Properties {
		//TODO: encapsulate more information like expires etc.
		//c := &http.Cookie{Name: k, Value: v, Path: url.Path, Domain: url.Host}
		c := &http.Cookie{Name: k, Value: v, Path: "/", Domain: u.Host}
		//log.Printf("Adding Cookie: %s",c)
		ret = append(ret,c)
	}
	return ret
}

// HTTPContext is a context object that will be injected by the frontend whenever an exposed method or function parameter
// is of type *HTTPContext. It contains references to all relevant http related objects like request and 
// response object.
type HTTPContext struct{
	Client *http.Client
	Request *http.Request
	Response http.ResponseWriter
	ErrorStatus int
	ReturnStatus int
	Frontend *Frontend
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

//CRID returns the coreltation id if existing. Otherwise nil.
func (c *HTTPContext) CRID() string {
	return c.Request.Header.Get(DefaultHeaderCRID)
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
func NewFrontend(args ...Properties) (*Frontend){
	f := Frontend{
		backend: newBackend(),
		ServeMux: http.NewServeMux(),
		flags: F_DEFAULT,
		extUrl: nil,
		addr: DefaultListenAddress,
		templateSource: DefaultTemplates(),
		templateBasePath: DefaultBasePath,
		namespace: DefaultNamespace,
		context: DefaultContext,
		publicDir: DefaultFileServerDir,
		key: GenerateKey(16),
		cache: make(map[string]*cache),
		template: make(map[string]*template.Template),
		HTTPContextConstructor: NewHTTPContext,
		publicContext: DefaultFileServerContext}

	//Initialize cache
	for _,p := range Platforms {
		f.cache[p] = &cache{}
	}

	if len(args) > 0 {
		for k,v:= range args[0] {
			switch k {
				case P_EXTERNALURL:
					url,err := url.Parse(v)
					if err != nil {
						panic(fmt.Errorf("Could not parse external url: \"%s\".",args[1]))
					}
					f.extUrl = url
					f.Context(string(url.Path))
					f.addr = string(url.Host)
				case P_LISTENADDR:
					f.addr = v
				case P_BASEPATH:
					f.templateBasePath = v
				case P_PUBLICDIR:
					f.publicDir = v
				case P_CONTEXT:
					f.Context(v)
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

	// HTTPContext is always availabel, dummy will never be used
	f.SetupGlobalInjection(&HTTPContext{})

	//Session is always available if request (by method parameter), dummy, will never be used
	f.SetupGlobalInjection(&Session{})

	f.SetupGlobalInjection(f)

	// BinaryContent may be nil
	var bc *BinaryContent = nil
	f.SetupGlobalInjection(bc)

	return &f
}

//BaseUrl returns the eternal base url of the service. 
// This may be a full qualified URL or just the path component.
func (b* Frontend) BaseUrl() string {
	if (b.extUrl != nil) {
		return b.extUrl.String()
	} else {
		return b.context
	}
}

// Preload JS libraries if existing.
// TODO: An order needs to specified somehow.
// TODO: Simplify this crap
func (b *Frontend) loadLibraries(c *HTTPContext,plat string) int{
	log.Printf("Loading default libraries ...")


	libbuf := new(bytes.Buffer)
	for _,u:= range b.templateSource[plat].Libraries {
		loadExternalLibrary(c,u,libbuf)
	}

	path :=b.templateBasePath + "/" + RelativeTemplatePath + "/" + plat + "/" + RelativeTemplateLibPath
	log.Printf("Searching for include JS libraries in `%s`",path)
	fd,err := os.Open(path)
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
						fd,err := os.Open(path +"/"+fi.Name());
						if err != nil {
							log.Printf("Could not open library file %s: %s",fi.Name(),err.Error());
							break
						}
						log.Printf("Reading JS library: %s",fi.Name());
						libbuf.ReadFrom(fd);
					case "url":
						fd,err := os.Open(path+"/"+fi.Name());
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
							loadExternalLibrary(c,url.String(),libbuf)
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
		b.cache[plat].libraries = libbuf.String()
		return bl
	}
	return 0
}

// Load the contents of the given "url" and write it to the "out" writer.
func loadExternalLibrary(c *HTTPContext,url string, out io.Writer) {
	log.Printf("Loading external JS library: %s",url)
	resp,e := c.Client.Get(url)
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
func (b *Frontend) loadTemplatesFromDir(plat string) {
	ntemplate,e := template.ParseFiles(
		path.Join(b.templateBasePath,RelativeTemplatePath,plat,HTTPTemplate),
		path.Join(b.templateBasePath,RelativeTemplatePath,plat,BindingTemplate),
		path.Join(b.templateBasePath,RelativeTemplatePath,plat,InterfaceTemplate),
		path.Join(b.templateBasePath,RelativeTemplatePath,plat,MethodTemplate))
	if e!=nil {
		log.Printf("Could not load template \"%s\". Using default templates.",e.Error())
		b.loadDefaultTemplates()
	} else {

		for _,t:= range ntemplate.Templates() {
			log.Printf("Template for '%s' platform found: %s.",plat,t.Name())
		}
		b.template[plat] = ntemplate
	}
}

// Load internal default templates for "binding.js", "interface.js" and "method.js".
func (b *Frontend) loadDefaultTemplates() {
	for p,t := range b.templateSource {
		ft := template.New(HTTPTemplate)
		_,e1 := ft.Parse(t.HTTP)

		ft = ft.New(BindingTemplate)
		_,e2 := ft.Parse(t.Binding)

		ft = ft.New(InterfaceTemplate)
		_,e3 := ft.Parse(t.Interface)

		ft = ft.New(MethodTemplate)
		_,e4 := ft.Parse(t.Method)

		if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
			panic(fmt.Errorf("Could not load internal templates for platform '%s': %s %s %s %s",p,e1.Error(),e2.Error(),e3.Error(),e4.Error()))
		}
		b.template[p] = ft
	}
}

// ClearCache clears the internally used cache. This also includes the engine code which needs 
// to be reassembled afterwards. This happens on the next call that requests the engine.
func (b *Frontend) ClearCache() {
	for p,_ := range b.cache {
		log.Printf("Clearing platform cache '%s' cache at revision %d",p,b.cache[p].revision)
		b.cache[p] = &cache{}
	}
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

// engineCacheKey generates a) an appropriate cache key for the given platform which may inlcude
// the request URL of the current request context. And b) the url the given platform will use
// to access the server side. This url is used for the templating engine which is cached.
func (f *Frontend) engineCacheKey(url *url.URL,platform string) (key string, rurl string) {
	if f.extUrl != nil {
		url = f.extUrl
	}

	rurl = url.String()
	key = platform

	switch platform {
		case "nodejs":
			key = fmt.Sprintf("%s.%s%s",url.Scheme,url.Host,f.Context())
		case "web":
			if f.extUrl != nil {
				rurl = f.extUrl.String()
			} else {
				rurl = f.Context()
			}
	}
	return
}

//platform identify the requested platform of request.
func platform(r *http.Request) string {
	for _,plat := range Platforms {
		if strings.HasSuffix(r.URL.Path,plat) {
			return plat
		}
	}
	return DefaultPlatform
}

//Minify tries to cpmpile the given javascript source code using the google closure compiler.
// If the closure compiler failes it falls back to a pure go implementation.
func Minify(c *http.Client,source []byte) []byte {
	//Use Closure compiler API first:
	client := &compilerapi.Client{HTTPClient: c,Language:"ECMASCRIPT5", CompilationLevel: "SIMPLE_OPTIMIZATIONS"}
	o := client.Compile(source)
	if len(o.Errors) <= 0 && o.ServerErrors == nil && len(o.CompiledCode) > 10  {
		return []byte(o.CompiledCode)
	}

	// Log errors from Closure Compiler
	for _,e := range o.Errors {
		log.Println(e.AsLogline())
	}

	log.Println("Closure Compiler failed. Using pure go implemenation.")
	if ret, err := jsmin.Minify(source); err == nil {
		return ret
	} else {
		log.Printf("Minify failed: %s.",err)
	}

	return source
}

// build is an internally used function that compiles the javascript based 
//proxy-object (JS engine) including external libraries. This consists of 4 areas:
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
// This includes also external and internal libariries if the corresponding configuration 
// flag F_INCLUDE_LIBRARIES is set
// TODO: Split this in individual methods if feasible.
// TODO: Improve minify step.
func (b *Frontend) build(c *HTTPContext,out io.Writer) {
	p:=platform(c.Request)
	url := b.externalUrlFromRequest(c.Request)
	ckey,baseUrl := b.engineCacheKey(url,p)

	if _,exists := b.cache[ckey];!exists {
		b.cache[ckey] = &cache{}
	}

	//TODO: improve, its not nice
	//Platform nodejs can only be cached if the external URL is explicitly defined.
	if  len(b.cache[ckey].engine) <= 0 || b.cache[ckey].revision < b.revision  {
		buf := new(bytes.Buffer)

		log.Printf("Generating proxy object at revision %d for context: %s at baseUrl: %s",b.revision,b.context,baseUrl)
		// (1) Libraries
		if (b.flags & F_LOAD_LIBRARIES) > 0 && (len(b.cache[p].libraries) > 0 || b.loadLibraries(c,p) > 0) {
			io.WriteString(buf,b.cache[p].libraries)
		}

		// (2)  Engine (binding engine)
		//TODO: Find a way to minimize templates while they are loaded.
		if (b.flags & F_LOAD_TEMPLATES) > 0 {
			b.loadTemplatesFromDir(p)
		} else {
			b.loadDefaultTemplates()
		}

		vav := ""
		if (b.flags & F_VALIDATE_ARGS) > 0 {
			vav="true"
		}
		proxyParams := map[string]string{
			tokenNamespace: b.namespace,
			tokenValidateArguments: vav,
			tokenHeaderCRID: DefaultHeaderCRID,
			tokenContentType: DefaultMimeType,
			tokenCRIDLength: fmt.Sprintf("%d",CRIDLength),
			tokenBaseContext: baseUrl }

		//TODO: check which params are actually needed here.
		minbuf := new(bytes.Buffer) //Buffer for the js code that will by minified.
		b.template[p].Lookup(HTTPTemplate).Execute(minbuf,proxyParams)
		b.template[p].Lookup(BindingTemplate).Execute(minbuf,proxyParams)

		// (3) Interface objects
		interfaces:=b.InterfaceNames()
		for _,in := range interfaces{
			interfaceParams := MapAppend(map[string]string{
				tokenInterfaceName: in }, proxyParams)

			b.template[p].Lookup(InterfaceTemplate).Execute(minbuf,interfaceParams)

			// (4) Method objects
			methods := b.bindingContainer.BindingNames(in)
			for _,m := range methods {
				bi,_ := b.Binding(in,m)
				vs:= bi.ValidationString()

				rbc := ""
				if receivesBinaryContent(bi) {
					rbc = "true"
				}

				methodParams := MapAppend(map[string]string{
					tokenMethodName: m,
					tokenHttpMethod: "POST",
					tokenHasBinary: rbc,
					tokenArgumentsString: vs},interfaceParams)
				b.template[p].Lookup(MethodTemplate).Execute(minbuf,methodParams)
			}
		}

		//Minify
		if (b.flags & F_ENABLE_MINIFY) > 0 {
			buf.Write(Minify(c.Client,minbuf.Bytes()))
		} else {
			buf.Write(minbuf.Bytes())
		}

		//Set the cache 
		b.cache[ckey].engine = buf.String()
		b.cache[ckey].revision = b.revision
	}
	//io.WriteString(out,b.cache.engine)
	out.Write([]byte(b.cache[ckey].engine))
}

//Context gets or sets the gotojs path context. This path element defines
//how the engine code where the engine js code is served.
func (f *Frontend) Context(args ...string) string {
	al:=len(args)
	if (al > 0) {
		f.context = args[0]

		//Sanity checks on the context
		if len(f.context) <= 0 {
			f.context = "/"
		}
		if !strings.HasPrefix(f.context,"/") {
			f.context = "/" + f.context
		}
		if !strings.HasSuffix(f.context,"/") {
			f.context = f.context + "/"
		}
	}

	return f.context[:len(f.context)-1]
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
		log.Printf("FileServer enabled at '/%s'",f.publicContext)
		f.fileServer = http.StripPrefix("/"+f.publicContext+"/",http.FileServer(http.Dir(f.publicDir)))
		f.Handle("/" + f.publicContext + "/",f.fileServer)
	} else {
		log.Printf("FileServer is enabled, but root dir \"%s\" does not exist or is not accessible.",f.publicDir)
	}
}

// logWrapper type acts as a http handler that wrapps any other Muxer
// or Handler
type logWrapper struct{
	handler http.Handler
}

// ServeHTTP is a httpn handler method and wraps the origin one of 
// LogMuxer
func (lm *logWrapper) ServeHTTP(w http.ResponseWriter,r *http.Request)  {
	t := time.Now()
	defer Log(r.Method,strconv.FormatInt(time.Since(t).Nanoseconds() / (1000),10),r.URL.Path)
	lm.handler.ServeHTTP(w,r)
}

//NewlogWrapper creates a new LogMuxer, that wraps the given http 
// handler. See LogMuxer for more details.
func NewlogWrapper(origin http.Handler) *logWrapper {
	return &logWrapper{origin}
}

// Setup creates and returns the final http handler for the frontend.
// It is called automatically by start, but if the frontend is used as
// an handler somewhere alse this setup method should be called instead.
// TODO: check what can be moved to initialization phase.
func (f *Frontend) Setup(args ...string) (handler http.Handler){
	al:=len(args)

	if (al > 0) {
		f.addr = args[0]
	}

	if (al > 1) {
		f.Context(args[1])
	}

	// Setup gotojs engine handler.
	log.Printf("GotojsEngine enabled at '%s'",f.context)

	f.HandleFunc(f.context,func (res http.ResponseWriter, req *http.Request) {
		f.serveHTTP(res,req)
	})

	if f.flags & F_ENABLE_ACCESSLOG  > 0{
		handler = NewlogWrapper(f)
	} else {
		handler = f
	}

	f.httpd = &http.Server{
		Addr:		f.addr,
		Handler:	handler,
		ReadTimeout:	5 * time.Second,
		WriteTimeout:	10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	return
}

// Start starts the http frontend. This method only returns in case a panic or unexpected
// error occurs.
// 1st optional parameter is the listen address ("localhost:8080") and 2nd optional parmaeter is
// the engine context ("/gotojs")
// If these are not provided, default or initialization values are used
func (f *Frontend) Start(args ...string) error {
	_  = f.Setup(args...)
	log.Printf("Starting server at \"%s\".",f.addr)
	return f.httpd.ListenAndServe()
}

// Redirect is a convenience method which configures a redirect handler from the patter to adestination url.
func (f* Frontend) Redirect(pattern,url string) {
	f.Handle(pattern,http.RedirectHandler(url,http.StatusFound))
}

//HandleStatic is a convenience method which defines a static content handle that is assigned to the given path pattern.
// It allows to declare statically served content like HTML or JS snippets.
// The Content-Type can be optionally specified.
func (f* Frontend) HandleStatic(pattern, content string, mime ...string) {
	f.HandleFunc(pattern, func(w http.ResponseWriter,r *http.Request) {
		if len(mime) > 0 {
			w.Header().Set(CTHeader, mime[0])
		}
		w.Write([]byte(content))
	})
}

//externalUrlFromRequest builds a full qualified URL from the given request object.
func (f *Frontend) externalUrlFromRequest(r *http.Request) (ret *url.URL) {
	ret = &url.URL{}
	if r.TLS == nil {
		ret.Scheme = "http"
	} else {
		ret.Scheme = "https"
	}
	ret.Host = r.Host

	if elems := strings.SplitAfterN(r.URL.Path,f.Context(),2); len(elems) == 2 {
		ret.Path = elems[0]
	} else {
		ret.Path = r.URL.Path
	}

	ret.RawQuery = r.URL.RawQuery
	ret.Fragment = r.URL.Fragment
	return
}

//ExposeRemoteBinding ExposeRemoteBinding exposes a remote Binding by specifying the corresponding url.
//A proxy function will be installed that passes the binding request to the remote side.
func (b *Frontend) ExposeRemoteBinding(u,rin,rmn,signature,lin,lfn string) Bindings {

	url,err := url.Parse(u)
	if err != nil {
		panic(fmt.Errorf("'%s' parameter is not a valid url: %s",u,err))
	}

	proxy := func (hc *HTTPContext, ses *Session, in []interface{}) interface{} {
		cli := NewProxyClient(hc.Client,ses,url,b.externalUrlFromRequest(hc.Request).String(),hc.CRID())
		//take incoming header.
		cli.CopyHeader(hc.Request)

		ret,err := cli.Invoke(rin,rmn,in...)
		if err != nil {
			panic(err)
		}
		return ret
	}

	//TODO: make clean and move to ExportFunction method of BindingContainer
	pm:=b.newRemoteBinding(proxy,signature,lin,lfn)
	b.revision++
	return pm.S()
}

// serveHTTP processes http request. The behaviour depends on the path and method of the call ass follows:
//	"POST": regular binding call. Interface and method name as well as parameter 
//		are expected in the body of the POST call as a JSON object.
//	"GET":  If the call points to a binding ("/<interface>/<method>"), the binding will be
//		invoked using the url-parameter in the given order (parameter names are ignored):
//		i.e "/gotojs/Test/Hello?p=My&x=Name&z=is&p=Earl" would invoke the signature
//		func (string,string,string,String).
//		If the call does not point to a binding like ("/gotojs") the engine code is returned.
func(f *Frontend) serveHTTP(w http.ResponseWriter,r *http.Request) {
	Log("REQUEST","-",r.URL.Path)
	mt := DefaultMimeType
	obuf := new(bytes.Buffer)
	crid := DefaultCRID
	var httpContext *HTTPContext
	defer func() {
		w.Header().Set(CTHeader,mt)
		w.Header().Set(DefaultHeaderCRID,crid)
		w.Header().Set("Access-Control-Allow-Origin","*")
		if re:=recover();re!=nil {
			//TODO: Create a HTTPErrorf() (besiedes httpcontest.Errorf() )
			// here that also contains the desired status code.
			mes := fmt.Sprintf("/*\n\n%s\n\n*/",re)
			w.Header().Set(DefaultHeaderError,mes) //TODO: maybe some encoding here.
			if httpContext != nil {
				http.Error(w,mes,httpContext.ErrorStatus)
			} else {
				//Happens only if Context Constructor fails.
				http.Error(w,mes,http.StatusInternalServerError)
			}
			debug.PrintStack()
		} else {
			w.WriteHeader(httpContext.ReturnStatus)
		}
		obuf.WriteTo(w)
		r.Body.Close()
	}()

	httpContext = f.HTTPContextConstructor(r,w)

	if httpContext.Frontend == nil {
		httpContext.Frontend = f
	}

	if crid = httpContext.Request.Header.Get(DefaultHeaderCRID); len(crid) == 0 {
		crid = DefaultCRID
	}

	session := httpContext.Session(f.key)

	defer session.Flush(w,f.key) //Update session on client side if necessary.

	path := r.URL.Path
	if strings.HasPrefix(path,f.context) {
		sub:= strings.SplitAfterN(path,f.context,2)
		elems := strings.Split(sub[1],"/")
		if len(elems) >= 2 {
			//Check if binding exists
			if b,found := f.Binding(elems[0],elems[1]); found {
				//Take paremeters from path
				args := SAToIA(elems[2:]...)

				//Check if the query string contains parameters
				if vals,err := url.ParseQuery(r.URL.RawQuery); err == nil {
					for _,v := range vals {
						args = append(args,SAToIA(v...)...)
					}
				}

				//Parameter in json body are only accepted for POST calls and ContentType "application/json"
				if ct := httpContext.Request.Header.Get(CTHeader); strings.HasPrefix(ct,DefaultMimeType) && r.Method == "POST" {
					dec:=json.NewDecoder(r.Body)
					var i []interface{}
					if e:= dec.Decode(&i); e != nil {
						panic(e)
					}

					args = append(args,i...)
				}

				vs := b.ValidationString()
				if len(vs) != len(args) {
					httpContext.Errorf(http.StatusBadRequest,"Invalid parameter count: %d/%d (%s)%s",len(args),len(vs),vs,args)
					return
				}

				switch r.Method {
					case "GET":
						mt = b.processCall(obuf,NewI(httpContext,session),args...)
					default:
						mt = b.processCall(obuf,NewI(httpContext,session,NewBinaryContent(r)),args...)
				}
			} else {
				httpContext.Errorf(http.StatusNotFound,"Binding %s.%s not found.",elems[0],elems[1])
			}
		} else {
			log.Printf("Sending Engine.")
			mt = "application/javascript"
			f.build(httpContext,obuf)
		}
	} else {
		//Not Gotojs context
		httpContext.Errorf(http.StatusNotFound,"Not within gotojs context.")
		return;
	}
}

// Internally used method to process a call. Input parameters, interface name and method name are read from a JSON encoded
// input stream. The result is encoded to a JSON output stream.
func (f Binding) processCall(out io.Writer,injs Injections,args ...interface{}) (mime string) {
	var b []byte
	var err error
	defer func() {Log("CALL","-",f.Name(),strconv.Itoa(len(b))) }()
	ret := f.InvokeI(injs,args...)

	if bin,ok := ret.(Binary); ok {
		defer bin.Close()
		b, err = ioutil.ReadAll(bin)
		mime = bin.MimeType()
	} else {
		b, err = json.Marshal(ret)
		mime = DefaultMimeType
	}

	if err != nil {
		panic(err)
	}

	out.Write(b)
	return
}
