package main

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func TestComment(t *testing.T) {
	want := true
	got := isComment("# comment number one")
	if want != got {
		t.Fatalf("want %v but got %v\n", want, got)
	}
}

func TestSection(t *testing.T) {
	want := "sctn1"
	got := section("[Sctn1]")
	if want != got {
		t.Fatalf("want %v but got %v\n", want, got)
	}
}

func TestProductVersion(t *testing.T) {
	want := "1.2.3"
	f, err := ioutil.ReadFile("testdata/simple/OPSI/control")
	if err != nil {
		t.Fatal(err)
	}
	r := bytes.NewReader(f)
	m, err := parse(r)
	if err != nil {
		t.Fatal(err)
	}
	got := m["product_version"]
	if want != got {
		t.Fatalf("want %q but got %q\n", want, got)
	}
}
