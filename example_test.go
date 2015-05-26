package gotojs

import (
	"net/http"
	"fmt"
	"log"
	"time"
	"io/ioutil"
)

func ExampleFrontend() {
	// Initialize the container.
	container := NewContainer();

	// Declare function which needs to be exposed.
	f:= func ( context *HTTPContext,name string) string {
			// The exposed function takes the HTTPContext as param. The HTTPContext 
			// will be injected by gotojs in order to give functions access to HTTP 
			// related information.
			return fmt.Sprintf("Hello %s, how are you ? (@%s)", name,context.Request.URL.String());
	}

	// Expose the function and name it.
	container.ExposeFunction(f,"Example","Hello")

	// Start the server is seperate go routine in parallel.
	go func() {log.Fatal(container.Start("localhost:8787"))}()

	time.Sleep(1 * time.Second) // Wait for the other go routine having the server up and running.

	// Parameters read from the url path.
	dump(http.Get("http://localhost:8787/gotojs/Example/Hello/TestEngine"))

	// Parameter can also be read from query string.
	dump(http.Get("http://localhost:8787/gotojs/Example/Hello?p=TestEngine"))

	// Output: 
	// "Hello TestEngine, how are you ? (@/gotojs/Example/Hello/TestEngine)"
	// "Hello TestEngine, how are you ? (@/gotojs/Example/Hello?p=TestEngine)"
}

func dump(resp *http.Response,err error) {
	b,_ := ioutil.ReadAll(resp.Body)
	fmt.Println( string(b) )
}
