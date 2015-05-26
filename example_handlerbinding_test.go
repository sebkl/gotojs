package gotojs

import(
	"time"
	"net/http"
)

func ExampleBindingContainer_handlerbinding() {
	// Initialize the container.
	container := NewContainer()

	// Declare a Hello World handler function.
	container.ExposeHandler(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello World! This data is not transformed into a JS object."))
	},"main","hello")

	// Start the server is seperate go routine in parallel.
	go func() { container.Start(":8792","/gotojs") }()

	time.Sleep(1 * time.Second) // Wait for the other go routine having the server up and running.
	dump(http.Get("http://localhost:8792/gotojs/main/hello"))

	// Output: 
	// Hello World! This data is not transformed into a JS object.
}



