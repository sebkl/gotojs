package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"
)

const (
	ALPHA      = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	CRIDLength = 20
)

type Client struct {
	Client      *http.Client
	baseUrl     *url.URL
	proxyHeader string
	baseCRID    string
	callCount   int
	Header      http.Header
}

//BinaryResponse represents a non json content which cannot be inspected by gotojs
// like an image or other content.
type BinaryResponse struct{ *http.Response }

func (b *BinaryResponse) MimeType() string {
	return b.Response.Header.Get("Content-Type")
}

func (b *BinaryResponse) Read(p []byte) (n int, err error) {
	return b.Response.Body.Read(p)
}

func (b *BinaryResponse) Close() error {
	return b.Response.Body.Close()
}

func (b *BinaryResponse) Catch() (ret []byte, err error) {
	ret, err = ioutil.ReadAll(b.Response.Body)
	if err != nil {
		return
	}
	err = b.Response.Body.Close()
	return
}

//NewBinaryResponse returns a new binary response object for a http response.
func NewBinaryResponse(res *http.Response) (ret *BinaryResponse) {
	if res.Body != nil {
		ret = &BinaryResponse{res}
	}
	return
}

//generateCRID generates a random corelation ID.
func generateCRID() (ret string) {
	rb := make([]byte, CRIDLength)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	alpha := []byte(ALPHA)
	for i := 0; i < CRIDLength; i++ {
		rb[i] = alpha[r.Intn(len(alpha))]
	}
	return string(rb)
}

//nextCRID returns the next valid CRID for the next call to the remote gotojs instance.
func (c *Client) nextCRID() (ret string) {
	//TODO: make thread safe !
	c.callCount++
	ret = fmt.Sprintf("%s.%d", c.baseCRID, c.callCount)
	return
}

//NewProxyClient creates a new gotojs client that can be used to
// proxy incoming requests to a remote gotojs instance from within a local
// gotojs instance.
func NewProxyClient(c *http.Client, jar http.CookieJar, bu *url.URL, ph string, crid string) (ret *Client) {
	if c == nil {
		c = &http.Client{}
	}

	ret = &Client{
		Client:      c,
		baseUrl:     bu,
		baseCRID:    crid,
		Header:      make(http.Header, 0),
		proxyHeader: ph,
	}

	ret.Client.Jar = jar
	return
}

//NewClient creates a plain gotojs client that points to a remote gotojs instance
func NewClient(bu string) (ret *Client) {
	u, err := url.Parse(bu)
	ret = &Client{
		Client:   &http.Client{},
		baseUrl:  u,
		baseCRID: generateCRID(),
		Header:   make(http.Header, 0),
	}
	ret.Client.Jar, err = cookiejar.New(nil)
	if err != nil {
		panic(err)
	}

	return
}

//CopyHeader copies incoming request header to the outgoing proxy call.
// Some headers are excluded like the cookie header.
func (c *Client) CopyHeader(req *http.Request) {
	for hn, ha := range req.Header {
		switch hn {
		case "Cookie", "Date":
			//ingore cookies
		default:
			for _, hv := range ha {
				c.Header.Set(hn, hv)
			}
		}
	}
}

//url generates the remote url based on the interface name and method name.
func (c *Client) url(in, mn string) string {
	//TODO: improve, this is a quick implementation
	return c.baseUrl.String() + "/" + in + "/" + mn
}

//Invoke a method/binding on the remote site.
func (c *Client) Invoke(in, mn string, args ...interface{}) (ret interface{}, err error) {
	by, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("Cannot encode remote request body: %s", err)
	}

	buf := bytes.NewBuffer(by)
	req, err := http.NewRequest("POST", c.url(in, mn), buf)
	if err != nil {
		return nil, fmt.Errorf("Cannot create remote request: %s", err)
	}

	//Build request Headers
	req.Header = c.Header
	req.Header.Set("Content-Type", "application/json")
	if len(c.proxyHeader) > 0 {
		req.Header.Set("x-gotojs-proxy", c.proxyHeader)
	}
	req.Header.Set("x-gotojs-crid", c.nextCRID())

	//Perform remote call
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Remote request call failed: %s", err)
	}

	mt := resp.Header.Get("Content-Type")
	eh := resp.Header.Get("x-gotojs-error")
	if len(eh) > 0 {
		err = fmt.Errorf(eh)
		return
	}

	switch mt {
	case "application/json":
		body, err := ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()
		err = json.Unmarshal(body, &ret)
		if err != nil {
			return nil, fmt.Errorf("Remote response could not be parsed: %s", err)
		}
	default:
		br := NewBinaryResponse(resp)
		by, err = br.Catch()
		ret = string(by)
	}
	return
}
