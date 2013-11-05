package gotojs


// Conatins all loaded templates and references to external JS libraries.
type Templates struct {
	Binding,Interface,Method string
	Libraries []string
}


// DefaultTemplates returns the collection of internal Javascript templates for the generation of the JS engine.
func DefaultTemplates() *Templates {
	return &defaultTemplates
}

var defaultTemplates = Templates {
	Binding: `
/* #### JS/BINDING #### */
var {{.NS}} = {{.NS}} || {
	'TYPES': {
		'INTERFACES': {/* will be filled by interface templates */},
		'Proxy': function () {
			/* Attributes */
			this.callCounter = 0;
		}
	},
	'CONST': {
		'ALPHA':"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	},
	'HELPER': {
		createCHASH: function makeid() {
			var ret = "";	
			for( var i=0; i < 10; i++ )
				ret += {{.NS}}.CONST.ALPHA.charAt(Math.floor(Math.random() * {{.NS}}.CONST.ALPHA.length));
			return ret.toUpperCase();
		}
	}
}

{{.NS}}.CONST.CHASH = {{.NS}}.HELPER.createCHASH()
	
{{.NS}}.TYPES.Proxy.prototype = {
	/*Methods */
	constructor: {{.NS}}.Proxy,
	call: function(i,m,args) {
{{if .BU}}
		var url ="{{.BU}}/"+i+"/"+m;

{{else}}
		var url ="{{.BC}}/"+i+"/"+m;
{{end}}
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
					if (!(typeof o === 'number' && o % 1 == 0)) {

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
	Libraries: []string{"http://ajax.googleapis.com/ajax/libs/jquery/1.10.2/jquery.min.js"}}
