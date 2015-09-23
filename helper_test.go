package gotojs

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"testing"
)

func TestArrayContains(t *testing.T) {
	x := [...]string{"a", "b", "c"}

	if !ContainsS(x[:], "b") {
		t.Errorf("Positiv contains test failed.")
	}

	if ContainsS(x[:], "x") {
		t.Errorf("Negative contains test failed.")
	}
}

func TestAppend(t *testing.T) {
	m1 := map[string]string{
		"a": "a"}

	m2 := map[string]string{
		"b": "a"}

	m3 := MapAppend(m1, m2)

	_, found1 := m3["a"]
	_, found2 := m3["b"]
	_, found3 := m1["b"]
	_, found4 := m2["a"]

	if !(found1 && found2 && found3 && !found4) {
		t.Errorf("Map append failed.")
	}

}

func TestEncryption(t *testing.T) {
	key := GenerateKey(16)
	s := base64.StdEncoding.EncodeToString(GenerateKey(2000))

	enc := Encrypt([]byte(s), key)

	if string(enc) == s {
		t.Errorf("Encryption failed: source equals encrypted")
	}

	dec := Decrypt(enc, key)

	if s != string(dec) {
		t.Errorf("Decryption failed: source does not match decrypted")
	}
}

func TestReaderArray(t *testing.T) {
	a := bytes.NewBufferString("-A-")
	b := bytes.NewBufferString("-B-")
	c := bytes.NewBufferString("-C-")

	ra := NewReaderArray(a, b, c)

	d := bytes.NewBufferString("-D-")
	ra.Add(d)

	expbuf := "-A--B--C--D-"

	by, err := ioutil.ReadAll(ra)
	if err != nil {
		t.Errorf("ReaderArray read failed: %s", err)
	}

	if string(by) != expbuf {
		t.Errorf("Concatenation of ReaderArray failed: %s/%s", string(by), expbuf)
	}
}

func TestReaderArrayBigData(t *testing.T) {
	a := bytes.NewBufferString("")
	b := bytes.NewBufferString("")
	c := bytes.NewBufferString("")
	tlen := int64(0)
	astr := "0123456789"
	ra := NewReaderArray(a, b, c)

	for i := 0; i < 1024*1024; i++ {
		a.Write([]byte(astr))
		b.Write([]byte(astr))
		c.Write([]byte(astr))
		tlen += int64(len(ra) * len(astr))
	}

	by, err := ioutil.ReadAll(ra)

	if err != nil {
		t.Errorf("ReaderArray read failed: %s", err)
	}

	if int64(len(by)) != tlen {
		t.Errorf("ReadArray read incomplete: %d/%d", len(by), tlen)
	}
}
