package gotojs_test

import (
	"net/http"
	"time"
	"fmt"
	"log"
	"bytes"
	. "gotojs"
)

func ExampleFrontend_static() {
	// Initialize the frontend.
	frontend := NewFrontend(F_DEFAULT ,map[int]string{P_EXTERNALURL: "http://localhost:8788/gotojs"})

	// Define the content.
	index:=`
<html>
 <head>
  <script src="gotojs/engine.js"></script>
 </head>
 <body><h1>Hello World !</h1></body>
</html>`

	// Assign the content to a path.
	frontend.HandleStatic("/",index,"text/html")

	// Start the server.
	go func() {log.Fatal(frontend.Start())}()

	time.Sleep(1 * time.Second) // Wait for the other go routine having the server up and running.

	// Read the response and print it to the console.
	resp, _ := http.Get("http://localhost:8788/")
	buf:= new(bytes.Buffer)
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
