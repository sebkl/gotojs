package gotojs_test

import (
	"fmt"
	"log"
	"time"
	. "gotojs"
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
	frontend := NewFrontend(F_DEFAULT ,map[int]string{P_EXTERNALURL: "http://localhost:8790/gotojs"})

	service := Service{name: "TestService"}

	// Expose the funcation and name it.
	frontend.ExposeInterface(service)

	// Start the server is seperate go routine in parallel.
	go func() {log.Fatal(frontend.Start())}()

	time.Sleep(1 * time.Second) // Wait for the other go routine having the server up and running.
	fmt.Println( Post("http://localhost:8790/gotojs/","Service","Hello","TestEngine") )

	// Output: 
	// {"CRID":"TEST","Data":"Hello TestEngine, how are you ? Regards, TestService."}
}
