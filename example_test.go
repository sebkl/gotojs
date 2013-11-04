package gotojs_test

import (
	"net/http"
	"fmt"
	"log"
	"bytes"
	"time"
	. "gotojs"
)

func ExampleFrontend() {
	// Initialize the frontend.
	frontend := NewFrontend(F_DEFAULT ,map[int]string{P_EXTERNALURL: "http://localhost:8787/jsproxy"})

	// Declare function which needs to be exposed.
	f:= func ( context *HTTPContext,name string) string {
			// The exposed function takes the HTTPContext as param. The HTTPContext 
			// will be injected by gotojs in order to give functions access to HTTP 
			// related information.
			return fmt.Sprintf("Hello %s, how are you ? (@%s)", name,context.Request.URL.String());
	}

	// Expose the funcation and name it.
	frontend.ExposeFunction(f,"Example","Hello")

	// Start the server is seperate go routine in parallel.
	go func() {log.Fatal(frontend.Start())}()

	time.Sleep(1 * time.Second) // Wait for the other go routine having the server up and running.
	fmt.Println( Post("http://localhost:8787/jsproxy/","Example","Hello","TestEngine") )

	// Output: 
	// {"CRID":"TEST","Data":"Hello TestEngine, how are you ? (@/jsproxy/)"}
}

// Post performs a call to the gotojs proxy backend without the JS engine.
// It show how the JS engine internally converts method invocations into HTTP
// POST requests.
func Post(url,in,mn,name string) string{
	ibuf:= bytes.NewBufferString("{ \"CRID\":\"TEST\",\"Interface\": \"" + in + "\",\"Method\": \"" + mn + "\", \"Data\": [\"" + name + "\"] }")
	obuf:= new(bytes.Buffer)
	resp, err := http.Post(url,"application/json",ibuf)
	if err != nil {
		log.Fatalf("Failed to perform post call: %s",err.Error())
	}
	obuf.ReadFrom(resp.Body)
	defer resp.Body.Close()
	return obuf.String()
}
