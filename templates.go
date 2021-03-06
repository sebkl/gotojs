package gotojs

// Contains all loaded templates and references to external JS libraries.
type Template struct {
	HTTP, Binding, Interface, Method string
	Libraries                        []string
}

//Templates per engine (web, nodejs) etc
type Templates map[string]*Template

// DefaultTemplates returns the collection of internal Javascript templates for the generation of the JS engine.
func DefaultTemplates() (ret Templates) {
	ret = make(Templates)
	ret["web"] = &defaultTemplate
	ret["nodejs"] = &defaultNodeJSTemplate
	return
}

var Platforms = []string{"web", "nodejs"}

var defaultTemplate = Template{
	HTTP: `
/* ### JS/HTTP jquery #### */
var {{.NS}} = {{.NS}} || {
	'HTTP': {
		'MaxConcurrentCalls': 20,
		'Backlog': [],
		'Status': {
			'open': { },
			'size': function() {
				var ret = 0;
				for (var k in this.open) { ret++; }
				return ret;
			},
			'oncompleted': undefined,
			'oninprogress': undefined,
			'onchange': undefined,
			'onprogress': undefined
		},
		'Queue': function(r,crid) {
			var http = this;
			var status = this.Status;
			var size = status.size()
			if (size >= this.MaxConcurrentCalls) {
				this.Backlog.push(r);
				return;
			} 

			/* If no xhr defined setup up the progress hooks */
			if (r.xhr === undefined && status.onprogress) {
				r.xhr = function () {
					var xhr = new window.XMLHttpRequest();
					xhr.upload.addEventListener("progress", function (e) {
						if (e.lengthComputable) { status.onprogress(e.loaded/e.total); }
					}, false);
					xhr.addEventListener("progress", function (e) {
						if (e.lengthComputable) { status.onprogress(e.loaded/e.total); }
					}, false);
					return xhr;
				};
			}

			r = $.ajax(r);
			r.CRID = crid;

			if (status.onchange) {
				status.onchange();
			}
			status.open[crid] = r;
			if (status.size() == 1 && status.oninprogress) {
				status.oninprogress();
			}
			r.complete(function() { 
				delete status.open[crid]
				if (status.size() == 0 && status.oncompleted) {
					status.oncompleted();
				}

				if (status.change) {
					status.onchange();
				}

				if (http.Backlog.length > 0) {
					http.Queue(http.Backlog.shift(),crid);
				}
			});
		},
		'Call': function(crid,url,i,m,data,imt,callback,method) {
			var ret;
			var tobj = { crid: crid, data: data, interface: i, method: m};
			this.Queue({
				type: method || 'POST',
				url: url,
				headers: {
					"{{.IH}}": crid,
					"Content-Type": imt
				},
				data: data,
				cache: false,
				contentType: false,
				processData: (imt == "{{.CT}}"),

				success: function(d,textStatus,request) {
					var mt = request.getResponseHeader('Content-Type');
					ret = d; /* assume that jquery parses json. */
					if (callback) {
						callback.bind(tobj)(ret);
					} 
				},
				async: callback !== undefined,
				error: function(o,estring,e) {
					throw /*console.log*/("FAIL : ["+crid+"]["+url+"]["+imt+"][" + o.status + "]["+o.getResponseHeader('x-gotojs-error')+"]["+data+"]\n" + estring + "," + e);
				}
			},crid);

			if (callback) {
				return tobj;
			} else {
				return ret;
			}
		},
		CRIDHeaderName: "{{.IH}}",
		GOTOJSContentType: "{{.CT}}"
	}
}
`,
	Binding: `
/* #### JS/BINDING #### */
var {{.NS}} = {{.NS}} || {};
{{.NS}}.TYPES={
		'INTERFACES': {/* will be filled by interface templates */},
		'Proxy': function () {
			/* Attributes */
			this.callCounter = 0;
		}
	};


{{.NS}}.CONST={
		'ALPHA':"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	};

{{.NS}}.HELPER={
		createCHASH: function makeid() {
			var ret = "";	
			for( var i=0; i < {{.CL}}; i++ )
				ret += {{.NS}}.CONST.ALPHA.charAt(Math.floor(Math.random() * {{.NS}}.CONST.ALPHA.length));
			return ret.toUpperCase();
		},
		base64Encode: function(a) {
			return btoa(JSON.stringify(a));
		},
		escapeSelector: function(id) {
			return id.replace( /(:|\.|\[|\]|,|=)/g, "\\$1" );
		},
		queryParameter: function(name) {
			name = name.replace(/[\[]/, "\\[").replace(/[\]]/, "\\]");
			var regex = new RegExp("[\\?&]" + name + "=([^&#]*)"), results = regex.exec(location.search);
			return results === null ? "" : decodeURIComponent(results[1].replace(/\+/g, " "));
		}
	};

{{.NS}}.ENCODERS={
		'o': {{.NS}}.HELPER.base64Encode
};

{{.NS}}.CONST.CHASH = {{.NS}}.HELPER.createCHASH()
	
{{.NS}}.TYPES.Proxy.prototype = {
	/*Methods */
	constructor: {{.NS}}.TYPES.Proxy,
	generateCRID: function() {
		return {{.NS}}.CONST.CHASH + "_" + (this.callCounter++);
	},
	Post: function(url,data,callback) {
		return {{.NS}}.HTTP.Call(this.generateCRID(),url,undefined,undefined,data,undefined,callback,"POST");
	},
	Call: function(i,m,args,bin,mt) {
		var url ="{{.BC}}/"+i+"/"+m;
		var callback = undefined;
		var method = "POST"

		if (this.hasCallback(args)) {
			callback = args.pop();
		}

		var crid = this.generateCRID();
		var data = ""
		if (bin !== undefined) {
			for (var i in args) { // Encode parameters
				args[i] = encodeURIComponent(args[i]);
			}

			if (args.length > 0) {
				url += "?p=" + args.join("&p=");
			}

			data = bin;
			mt = mt || "application/octet-stream";
		} else {
			data = JSON.stringify(args);
			mt = "{{.CT}}";
		}

		return {{.NS}}.HTTP.Call(crid,url,i,m,data,mt,callback,method);
	},
	buildGetUrl: function (i,m,args) {
		var ret = "{{.BC}}/"+i+"/"+m;
		var par = ""
		if(args.length > 0) {
			if (par.length <= 0) {
				par+="?"
			} else {
				par+="&"
			}
			for (var i in args) {
				//TODO: Do some encoding here.
				par+='=' + args[i]
			}
			ret+=par
		}
		return ret
	},
	hasCallback: function(args) {
		return (typeof args[args.length-1] == 'function')
	},
	argsToArray: function(ao) {
		var ret = [];
		for (var i in ao) {
			ret.push(ao[i]);
		}
		return ret;
	}{{if .MA}},
	assertArgs: function(i,m,args,as) {
		var al = args.length;
		var sl = as.length;
		/* Argument count either matchs or last argument is a callback function. */
		if (!((al == sl) ||  ((al-1 == sl) && this.hasCallback(args)))) {
			throw "Invalid argument count (" + al + "/" + al + ") for method \""+i+"." + m + "("+as+")";
		}

		for (var idx in as) {
			var o = args[idx];
			var mes = "Argument #" + (idx+1) + " of method \""+i+"." + m + "("+as+")\" is expected to be ";
			if (o === undefined) {
				throw mes + " not equal UNDEFINED.";
			}

			switch (as[idx]) {
				case 'a':
					if (!o instanceof Array) {
						throw mes+ "an Array.";
					}
					break;
				case 'o':
				case 'm':
					if (!o instanceof Object) {
						throw mes + "an object/struct/map.";

					}
					break;
				case 's':
					if (!o instanceof String) {
						throw mes + "a string.";
					}
					break;
				case 'i':
					if (!(typeof o === 'number' && Math.floor(o) == o)) {

						throw mes + "an integer.";
					}
					break;
				case 'f':
					if (!(typeof o === 'number' && !( o & 1 == 0))) {
						throw mes + "a float.";
					}
					break;

				case 'b':
					if (!(typeof o === 'boolean')) {
						throw mes + "a boolean.";
					}
					break;
				default: 
					throw "Invalid argument definition string: " + as;
			}
		}
	}{{end}}
};

/* General Proxy to expose an interface to perform HTTP AJAX calls. */
{{.NS}}.HELPER.Proxy = new {{.NS}}.TYPES.Proxy();

`,
	Interface: `
/* #### JS/INTERFACE #### */
{{.NS}}.TYPES.INTERFACES = {
	{{.IN}}: function() {
		/* Attributes */
		this.name = "{{.IN}}";
		this.proxy = {{.NS}}.HELPER.Proxy;
	}
}
{{.NS}}.TYPES.INTERFACES.{{.IN}}.prototype = {
	/* Methods */
	Call: function(m,args,bin,mt) {
		return this.proxy.Call(this.name,m,args,bin,mt);
	}
};
{{.NS}}.{{.IN}} = new {{.NS}}.TYPES.INTERFACES.{{.IN}}()
`,
	Method: `
/* #### JS/METHOD #### */

{{.NS}}.{{.IN}}.{{.MN}} = function() {
	var args = this.proxy.argsToArray(arguments);
	var bin, mt;
{{if .BIN}}
		bin = args.shift();
		mt = args.shift();
{{end}}

{{if .MA}}
		this.proxy.assertArgs("{{.IN}}","{{.MN}}",args,"{{.AS}}");
{{end}}
		return this.Call("{{.MN}}",args,bin,mt);
};

{{.NS}}.{{.IN}}.{{.MN}}.getValidationString = function() {
	return "{{.AS}}";
};

{{.NS}}.{{.IN}}.{{.MN}}.Url = function() {
	var args = {{.NS}}.{{.IN}}.proxy.argsToArray(arguments);
{{if .MA}}
	{{.NS}}.{{.IN}}.proxy.assertArgs("{{.IN}}","{{.MN}}",args,"{{.AS}}");
{{end}}
	return {{.NS}}.{{.IN}}.proxy.buildGetUrl("{{.IN}}","{{.MN}}",args);
};
`,
	//Libraries: []string{"http://ajax.googleapis.com/ajax/libs/jquery/1.10.2/jquery.min.js"} }
	Libraries: []string{"http://ajax.googleapis.com/ajax/libs/jquery/2.1.1/jquery.min.js"}}

var defaultNodeJSTemplate = Template{
	HTTP: `
var {{.NS}} = {{.NS}} || {
	'HTTP': {
		Request: require('request'),
		URL: "{{.BC}}",
		Jar: null,
		Call: function(crid,url,i,m,data,imt,callback,method) {
			var ret = { state: "loading", crid: crid, data: data, interface: i, method: m, result: null}
			if (callback === undefined) {
				callback = function(d) { console.log(d); ret.result = d;ret.state="finished";}
			}
			this.Request({
				uri: this.URL + "/" + i + "/" + m,
				method: method || "POST",
				jar: this.Jar,
				headers: {'content-type' : imt, '{{.IH}}': crid},
				timeout: 10000,
				body: data,
				followRedirect: true
			}, function(error, response, d) {
				if (error) {
					throw ("FAIL1["+crid+"] '" + i + "." + m + "(" + data +")' @ " + url + ":\n" + data + "\n=>" + error);
				}

				try {
					var mt = response.headers["content-type"] ;
					if (mt != "{{.CT}}") {
						ret.result = eval('(' + d + ')');
					} else {
						ret.result = d;
					}
				} catch (e) {
					throw ("FAIL2["+crid+"] '" + i + "." + m + "(" + data +")' @ " + url + ":\n" + data + "\n=>" + e +"\n" + d);
				}
				callback.bind(ret)(ret.result);
			});

			return ret;
		},
		CRIDHeaderName: "{{.IH}}",
		GOTOJSContentType: "{{.CT}}"
	}
};
{{.NS}}.HTTP.Jar = {{.NS}}.HTTP.Request.jar();
GLOBAL.{{.NS}} = {{.NS}};
`,
	Binding:   defaultTemplate.Binding,
	Interface: defaultTemplate.Interface,
	Method:    defaultTemplate.Method,
	Libraries: []string{}}
