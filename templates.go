package gotojs

// Conatins all loaded templates and references to external JS libraries.
type Template struct {
	HTTP,Binding,Interface,Method string
	Libraries []string
}

//Templates per engine (jquery, nodejs) etc
type Templates map[string]*Template

// DefaultTemplates returns the collection of internal Javascript templates for the generation of the JS engine.
func DefaultTemplates() (ret Templates) {
	ret = make(Templates)
	ret["jquery"] = &defaultTemplate
	ret["nodejs"] = &defaultNodeJSTemplate
	return
}

var Platforms = []string{"jquery","nodejs"}

var defaultTemplate = Template {
	HTTP:`
/* ### JS/HTTP jquery #### */
var {{.NS}} = {{.NS}} || {
	'HTTP': {
		Post: function(crid,url,i,m,args,data,callback) {
			var ret;
			$.ajax( {
				type: 'POST',
				url: url,
				data: data,
				success: function(d) {
					if (typeof(d) =='string') {
						ret = eval('(' + d + ')');
					} else {
						ret = d;
					}
					if (ret['Data'] && ret['CRID'] == crid) {
						ret = ret['Data']
						if (callback) {
							callback(ret);
						} 
					} else {
						throw ("IFAIL["+crid+"] \"" + i + "." + m + "(" + args.join(",")+")\" @ " + url + ":\n" + data + "\n=>" + ret);
					}
				},
				async: callback !== undefined,
				error: function(o,estring,e) {
					throw /*console.log*/("FAIL : \"" + i + "." + m + "(" + args.join(",")+")\" @ " + url + ":\n" + data + "\n=>" + e);
				}
			});
			return ret;
		}
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
			for( var i=0; i < 10; i++ )
				ret += {{.NS}}.CONST.ALPHA.charAt(Math.floor(Math.random() * {{.NS}}.CONST.ALPHA.length));
			return ret.toUpperCase();
		}
	};

{{.NS}}.CONST.CHASH = {{.NS}}.HELPER.createCHASH()
	
{{.NS}}.TYPES.Proxy.prototype = {
	/*Methods */
	constructor: {{.NS}}.TYPES.Proxy,
	call: function(i,m,args) {
		var url ="{{.BC}}/"+i+"/"+m;
		var callback = undefined;
		if (this.hasCallback(args)) {
			callback = args.pop();
		}

		var crid = {{.NS}}.CONST.CHASH + "." + (this.callCounter++);
		var data = JSON.stringify({
			'Interface': i,
		    	'Method': m,
		    	'CRID': crid,
		    	'Data': args
		});

		return {{.NS}}.HTTP.Post(crid,url,i,m,args,data,callback);
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
			throw "Invalid argument count (" + al + ") for method \""+i+"." + m + "("+as+")";
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
						throw mes + "an float.";
					}
					break;
				default: 
					throw "Invalid argument definition string: " + as;
			}
		}
	}{{end}}
};
`,
	Interface: `
/* #### JS/INTERFACE #### */
{{.NS}}.TYPES.INTERFACES = {
	{{.IN}}: function() {
		/* Attributes */
		this.name = "{{.IN}}";
		this.proxy = new {{.NS}}.TYPES.Proxy();
	}
}
{{.NS}}.TYPES.INTERFACES.{{.IN}}.prototype = {
	/* Methods */
	call: function(m,args) {
		return this.proxy.call(this.name,m,args);
	}
};
{{.NS}}.{{.IN}} = new {{.NS}}.TYPES.INTERFACES.{{.IN}}()
`,
	Method: `
/* #### JS/METHOD #### */
{{.NS}}.{{.IN}}.{{.MN}} = function() {
	var args = this.proxy.argsToArray(arguments);
{{if .MA}}
	this.proxy.assertArgs("{{.IN}}","{{.MN}}",args,"{{.AS}}");
{{end}}
	return this.call("{{.MN}}",args);
};
`,
	Libraries: []string{"http://ajax.googleapis.com/ajax/libs/jquery/1.10.2/jquery.min.js"} }

var defaultNodeJSTemplate = Template{
	HTTP: `
var {{.NS}} = {{.NS}} || {
	'HTTP': {
		Request: require('request'),
		URL: "{{.BC}}",
		Jar: null,
		Post: function(crid,url,i,m,args,data,callback) {
			var ret = { state: "loading",data: null };
			if (callback === undefined) {
				callback = function(d) { console.log(d); ret.data = d;ret.state="finished";}
			}
			this.Request({
				uri: this.URL + "/" + i + "/" + m,
				method: "POST",
				jar: this.Jar,
				headers: {'content-type' : 'application/json'},
				timeout: 10000,
				body: data,
				followRedirect: true
			}, function(error, response, d) {
				if (error) {
					throw ("FAIL1["+crid+"] '" + i + "." + m + "(" + args.join(",")+")' @ " + url + ":\n" + data + "\n=>" + error);
				}

				if (typeof(d) =='string') {
					try {
						ret.data = eval('(' + d + ')');
					} catch (e) {
						throw ("FAIL2["+crid+"] '" + i + "." + m + "(" + args.join(",")+")' @ " + url + ":\n" + data + "\n=>" + e +"\n" + d);
					}
				} else {
					ret.data = d;
				}

				if (ret.data['Data'] && ret.data['CRID'] == crid) {
					callback(ret.data['Data']);
				} else {
					throw ("FAIL3["+crid+"] '" + i + "." + m + "(" + args.join(",")+")' @ " + url + ":\n" + data + "\n=>" + ret);
				}
			});
			return ret;
		}
	}
};
{{.NS}}.HTTP.Jar = {{.NS}}.HTTP.Request.jar();
GLOBAL.{{.NS}} = {{.NS}};
`,
	Binding:  defaultTemplate.Binding,
	Interface: defaultTemplate.Interface,
	Method: defaultTemplate.Method,
	Libraries: []string{} }
