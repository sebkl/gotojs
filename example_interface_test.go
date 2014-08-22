package gotojs

import (
	"net/http"
	"io/ioutil"
	"fmt"
	"log"
	"time"
)

// Declare Service to be exposed.
type Service struct {
	name string
}

// Methods of Service that will be exposed.
func (s *Service) Hello(name string) string {
	return fmt.Sprintf("Hello %s, how are you ? Regards, %s.", name,s.name)
}

func ExampleFrontend_interface() {
	// Initialize the frontend.
	frontend := NewFrontend()

	service := Service{name: "TestService"}

	// Expose the funcation and name it.
	frontend.ExposeInterface(service)

	// Start the server is seperate go routine in parallel.
	go func() {log.Fatal(frontend.Start("localhost:8790"))}()

	time.Sleep(1 * time.Second) // Wait for the other go routine having the server up and running.

	resp,_ := http.Get("http://localhost:8790/gotojs/Service/Hello/TestEngine")
	b,_ := ioutil.ReadAll(resp.Body)
	fmt.Println( string(b) )

	// Output: 
	// "Hello TestEngine, how are you ? Regards, TestService."
}
