# gotojs
Package gotojs offers a library for **exposing go-interfaces as Javascript proxy objects**.   
Therefore package gotojs assembles a JS engine which creates proxy objects as JS code and forwards the calls to them via JSON encoded HTTP Ajax requests. This allows web developers to easily write HTML5 based application using jQuery,YUI and other simalar frameworks without explictily dealing with ajax calls and RESTful server APIs but using a transparent RPC service.

## Requirements
* go version 1.1.1
* node.js version v0.10.20
* npm version 1.3.11
* node.js module jquery@1.8.3

## Getting started
If not happend so far create your go environment and set the GOPATH environment variable accordingly:
```
#mkdir ~/go
#export GOPATH=~/go

```

Check your go, nodejs and npm versions:
```
#go version
go version go1.1.1 linux/amd64
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
   
Next, fetch the gotojs package:

```
go get github.com/sebkl/gotojs
go get github.com/sebkl/gotojs/util
```

and create a sample application tree:
```
${GOPATH}/bin/util example ${GOPATH}/www
```

Please keep in mind, that the example application `${GOPATH}/www/app.go` is intended to show some features. After playing around with it you should create your own !

### Documentation
The go documentation of the gotojs package is complete and also some additional examples, so its worth having a look.
```
cd ${GOPATH}
godoc -http=:8080
```
And browse to [http://localhost:8080/pkg/github.com/sebkl/gotojs/](http://localhost:8080/pkg/github.com/sebkl/gotojs/ "http://localhost:8080/pkg/github.com/sebkl/gotojs/")




