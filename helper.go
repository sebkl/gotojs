package gotojs


//The functions provided in helper.go are supposed to be extracted to a dedicated 
//package or repository.

import (
	"log"
	"strings"
	"bytes"
	"math/rand"
	"time"
	"crypto/aes"
	"crypto/cipher"
	"io"
	"runtime"
	"strconv"
	"fmt"
)

// Check if a slice contains a certain string.
func ContainsS(a []string, s string) bool {
	for _,v:= range a {
		if (v == s) {
			return true
		}
	}
	return false
}

// MapAppend string parameter to a string parameter map.
func MapAppend(to map[string]string,from map[string]string) map[string]string {
	for k,v:=range from {
		to[k] = v
	}
	return to
}

// Log is helper function that logs in various paremeter
// separated by a pipe in a standardized way.
func Log(t string,args ...string) {
	log.Printf("[%s]%d|%s",t,runtime.NumGoroutine(),strings.Join(args,"|"))
}

// generateKey generates a random application key.
func GenerateKey(size int) (ba []byte) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ba = make([]byte,size)

	for i:=0;i<size;i++ {
		ba[i] = byte(r.Intn(256))
	}
	return
}

// encrypt encrypts an input string using the given key.
func Encrypt(in,key []byte) []byte {
	ibuf := bytes.NewBuffer(in)
	obuf := new(bytes.Buffer)

	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	// If the key is unique for each ciphertext, then it's ok to use a zero
	// IV.
	var iv [aes.BlockSize]byte
	stream := cipher.NewOFB(block, iv[:])

	writer := &cipher.StreamWriter{S: stream, W: obuf}

	if _, err := io.Copy(writer, ibuf); err != nil {
		panic(err)
	}

	return obuf.Bytes()
}

// decrypt decrypts an encrypted input string.
func Decrypt(in,  key []byte) []byte {
	ibuf := bytes.NewBuffer(in)
	obuf := new(bytes.Buffer)

	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	// If the key is unique for each ciphertext, then it's ok to use a zero
	// IV.
	var iv [aes.BlockSize]byte
	stream := cipher.NewOFB(block, iv[:])

	reader := &cipher.StreamReader{S: stream, R: ibuf}

	if _, err := io.Copy(obuf, reader); err != nil {
		panic(err)
	}

	return obuf.Bytes()
}

//sToIArray is an string var args to interface{} array converter.
func SAToIA(args ...string) (ret []interface{}) {
	ret = make([]interface{},len(args))
	for i,v := range args {
		ret[i] = v
	}
	return
}

// ConvertTime tries to convert an incoming interface to a 
// local/native time object. Basically an order of formats will be tried here.
// Generally plain numbers are interpreted as unix timestamp in ms.
func ConvertTime(o interface{}) (ret time.Time,err error) {
	if iv,ok := o.(int64); ok {
		//Assume unix timestamp (ms)
		return time.Unix(int64(iv/1000),0),nil
	} else if fv, ok := o.(float64); ok {
		return time.Unix(int64(fv/1000),0),nil
	} else if sv, ok := o.(string); ok {
		//Integer as string
		if ms,err := strconv.ParseInt(sv,10,63); err == nil {
			return time.Unix(int64(ms/1000),0),nil
		}

		layouts := []string { time.RFC3339, time.RFC3339Nano, time.ANSIC, time.UnixDate, time.RubyDate, time.RFC822, time.RFC822Z, time.RFC850, time.RFC1123, time.RFC1123Z, time.Kitchen, time.Stamp, time.StampMilli, time.StampMicro, time.StampNano, "2006-01-02", "2006.01.02", "2006/02/01" }

		for _,lay := range layouts {
			ret, err = time.Parse(lay,sv)
			if err == nil {
				return
			}
		}
		err = fmt.Errorf("No suitable time format identified: %s",sv)

	}
	err = fmt.Errorf("Cannot convert time object: %s",o)
	return
}

type ReaderArray []io.Reader

//NewReaderArray creates a new ReaderArray. It is an equivalent to
//default array constructor ReaderArray{...}. But this may change in the future
// with further functionality.
func NewReaderArray(r ...io.Reader) ReaderArray { return r }

//Add adds a Reader to the ReaderArray. It is an equivalent to
// readerArray = append(readerArray, rd)
func (r *ReaderArray) Add(rd io.Reader) { *r = append(*r,rd) }

//Read reads successively from the ReaderArray.
func (r ReaderArray) Read(p []byte) (n int, err error) {
	l := len(p)
	i := 0


	//TODO: simplify
	for ;n < l && i < len(r);i++ {
		err = nil
		rb := -1

		for rb != 0 && err != io.EOF {
			rb,err = r[i].Read(p[n:])
			n += rb
		}

		if err != nil && err != io.EOF {
			return
		}
	}
	r = r[i:]
	return
}

//Close closes all Reader of the ReaderArray.
func (r ReaderArray) Close() (err error) {
	for _,i := range r {
		if rc, ok := i.(io.Closer); ok {
			err = rc.Close()
			if err != nil {
				return
			}
		}
	}
	return
}


