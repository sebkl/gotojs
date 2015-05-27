package gotojs

import(
	"time"
	"net/http"
	"bytes"
	"io"
	"io/ioutil"
)

func ExampleContainer_binarycontent() {
	// Initialize the container.
	container := NewContainer()

	// Declare a Hello World handler function. The input parameter is taken from the POST body
	// and passed as a "BinaryContent" object. The returned string will be JSON encoded.
	container.ExposeFunction(func(bc *BinaryContent) string {
		defer bc.Close()
		b,_ := ioutil.ReadAll(bc)
		return string(b)
	},"main","echo1")

	//Declare a Hello World handler function. The response is directly passed to the ResponseWriter.
	//The returning data is not anymore encoded as JSON.
	container.ExposeFunction(func(bc *BinaryContent, hc *HTTPContext) {
		defer bc.Close()
		io.Copy(hc.Response,bc)
	},"main","echo2")

	// Start the server is separate go routine in parallel.
	go func() { container.Start(":8793","/gotojs") }()

	time.Sleep(1 * time.Second) // Wait for the other go routine having the server up and running.

	buf := bytes.NewBufferString("Hello Echo Server!")
	dump(http.Post("http://localhost:8793/gotojs/main/echo1","text/plain",buf))

	buf = bytes.NewBufferString("This is not JSON!")
	dump(http.Post("http://localhost:8793/gotojs/main/echo2","text/plain",buf))

	// Output: 
	// "Hello Echo Server!"
	// This is not JSON!
}



