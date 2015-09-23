package client

import (
	"testing"
)

func TestGenerateCRID(t *testing.T) {
	id := generateCRID()
	t.Logf("CRID: %s", id)
	if len(id) != CRIDLength {
		t.Errorf("Generated id has not the correct length: %d/%d", len(id), CRIDLength)
	}

	id2 := generateCRID()
	t.Logf("CRID: %s", id2)
	if id == id2 {
		t.Errorf("2nd generated id is euqal the first one: %s/%s", id, id2)
	}
}
