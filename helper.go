package gotojs

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

// Append string parameter to a string parameter map.
func Append(to map[string]string,from map[string]string) map[string]string {
	for k,v:=range from {
		to[k] = v
	}
	return to
}

// Log is helper function that logs in various paremeter
// seperated by a pipe in a standardized way.
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

//toArray is an var args to array converter.
func sToIArray(args ...string) (ret []interface{}) {
	ret = make([]interface{},len(args))
	for i,v := range args {
		ret[i] = v
	}
	return
}

