package gotojs

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

func ExampleContainer_fileserver() {
	// Initialize the container.
	container := NewContainer()

	// Define the index.html and write it to the public dir:
	index := `
<html>
 <head>
  <script src="gotojs/engine.js"></script>
 </head>
 <body><h1>Hello World !</h1></body>
</html>`

	// Create a temporary file for testing purposes within the public fileserver directory.
	b := bytes.NewBufferString(index)
	err := ioutil.WriteFile("/tmp/__gotojs_index.html", b.Bytes(), 0644)
	defer func() {
		// Clean up the temporary index.html
		os.Remove("/tmp/__gotojs_index.html")
	}()
	if err != nil {
		panic(err)
	}

	//Enable the fileserver wiht docroot at "/tmp" under path "p"
	container.EnableFileServer("/tmp", "p")

	//Create a redirect from homepage to the temporary index.html
	container.Redirect("/", "/p/__gotojs_index.html")

	// Start the server.
	go func() { log.Fatal(container.Start("localhost:8789")) }()

	time.Sleep(1 * time.Second) // Wait for the other go routine having the server up and running.

	// Read the response and print it to the console.
	resp, _ := http.Get("http://localhost:8789/")
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	fmt.Println(buf.String())

	// Output:
	// <html>
	//  <head>
	//   <script src="gotojs/engine.js"></script>
	//  </head>
	//  <body><h1>Hello World !</h1></body>
	// </html>
}
