package gotojs_test

import(
	"fmt"
	"time"
	"net/http/cookiejar"
	"net/http"
	. "gotojs"
)

func ExampleFrontend_session() {
	// Initialize the frontend.
	frontend := NewFrontend()

	// Declare a public login function.
	login := func (session *Session, username,password string) string{
		if len(username) == len(password) { // Very stupid auth  ;)
			session.Set("username",username)
			session.Set("authorized","true")
			return "OK"
		} else {
			return "Invalid password."
		}
	}

	// Declare a private function callable.
	private := func (session *Session,i string) string {
		return fmt.Sprintf("This is private %s of user %s",i,session.Get("username"))
	}

	//Expose all functions and name them:
	frontend.ExposeFunction(login,"main","login")
	frontend.ExposeFunction(private,"main","private").If(
		AutoInjectF(func (session *Session, c *HTTPContext) (b bool) {
			if b = session.Get("authorized") == "true";!b {
				//Status code should be set to 403
				c.Response.WriteHeader(http.StatusForbidden)
			}
			return
		}))


	// Start the server is seperate go routine in parallel.
	go func() { frontend.Start(":8791","/gotojs") }()

	time.Sleep(1 * time.Second) // Wait for the other go routine having the server up and running.
	http.DefaultClient.Jar,_ = cookiejar.New(nil) // Cookie jar is needed here in order to associate session
	// First call without previous login should result in a not authorized message.
	fmt.Println( post("http://localhost:8791/gotojs/","main","private","TestData"))
	// Second call has an invalid password
	fmt.Println( post("http://localhost:8791/gotojs/","main","login","Alice","123456"))
	// Third call is a coorect login
	fmt.Println( post("http://localhost:8791/gotojs/","main","login","Alice","12345"))
	// Lat call is a successfull request for private data.
	fmt.Println( post("http://localhost:8791/gotojs/","main","private","TestData"))
	http.DefaultClient.Jar = nil // Remove the cookie jar

	// Output: 
	// {"CRID":"TEST","Data":null}
	// {"CRID":"TEST","Data":"Invalid password."}
	// {"CRID":"TEST","Data":"OK"}
	// {"CRID":"TEST","Data":"This is private TestData of user Alice"}
}



