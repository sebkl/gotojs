package gotojs

import(
	"encoding/base64"
	"testing"
)

func TestArrayContains(t *testing.T) {
	x := [...]string{"a","b","c"}

	if !ContainsS(x[:],"b") {
		t.Errorf("Positiv contains test failed.")
	}

	if ContainsS(x[:],"x") {
		t.Errorf("Negative contains test failed.")
	}
}


func TestAppend(t *testing.T) {
	m1:= map[string]string{
		"a": "a" }

	m2:= map[string]string{
		"b": "a"}

	m3:= Append(m1,m2)

	_,found1 := m3["a"]
	_,found2 := m3["b"]
	_,found3 := m1["b"]
	_,found4 := m2["a"]

	if !(found1 && found2 && found3 && !found4) {
		t.Errorf("Map append failed.")
	}

}


func TestEncryption(t * testing.T) {
	key := GenerateKey(16)
	s := base64.StdEncoding.EncodeToString(GenerateKey(2000))

	enc := Encrypt([]byte(s),key)

	if string(enc) == s {
		t.Errorf("Encryption failed: source equals encrypted")
	}

	dec := Decrypt(enc,key)

	if s != string(dec) {
		t.Errorf("Decryption failed: source does not match decrypted")
	}
}
