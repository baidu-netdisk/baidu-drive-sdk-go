package api

import "testing"

func TestPtr(t *testing.T) {
	s := Ptr("hello")
	if *s != "hello" {
		t.Errorf("Ptr(string) = %v, want hello", *s)
	}

	i := Ptr(42)
	if *i != 42 {
		t.Errorf("Ptr(int) = %v, want 42", *i)
	}

	b := Ptr(true)
	if *b != true {
		t.Errorf("Ptr(bool) = %v, want true", *b)
	}
}
