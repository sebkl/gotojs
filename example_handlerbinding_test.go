package gotojs

import(
	"time"
	"net/http"
)

func ExampleContainer_handlerbinding() {
	// Initialize the container.
	container := NewContainer()

	// Declare a Hello World handler function.
	container.ExposeHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello World! This data is not transformed into a JS object."))
	},"main","hello")

	// Declare a fake handler that always returns "404 page not found".
	container.ExposeHandler(http.NotFoundHandler(),"main","notfound")

	// Start the server is separate go routine in parallel.
	go func() { container.Start(":8792","/gotojs") }()

	time.Sleep(1 * time.Second) // Wait for the other go routine having the server up and running.
	dump(http.Get("http://localhost:8792/gotojs/main/hello"))
	dump(http.Get("http://localhost:8792/gotojs/main/notfound"))

	// Output: 
	// Hello World! This data is not transformed into a JS object.
	// 404 page not found

}



