package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"
)

func TestFinalOpsiPackageFilename(t *testing.T) {
	want := "p1_1.2.3-4.opsi"
	m := map[string]string{"package_version": "4",
		"product_version": "1.2.3",
		"product_id":      "p1",
	}
	got := filename(m)
	if want != got {
		t.Fatalf("want %v but got %v\n", want, got)
	}
}

func TestSimplePackage(t *testing.T) {
	buf, err := ioutil.ReadFile("testdata/simple/OPSI/control")
	if err != nil {
		t.Fatal(err)
	}
	workbench, err := tmpDir()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		log.Printf("removing %s\n", workbench)
		if err := os.RemoveAll(workbench); err != nil {
			log.Fatal(fmt.Errorf("cannot remove %s: %s", workbench, err))
		}
	}()
	if err := mkpkg(buf,
		"testdata/simple/OPSI",
		"testdata/simple/CLIENT_DATA",
		os.TempDir(),
		workbench,
	); err != nil {
		t.Fatal(err)
	}
}

func TestParseControlfile(t *testing.T) {
	f, err := ioutil.ReadFile("testdata/simple/OPSI/control")
	if err != nil {
		t.Fatalf("cannot read control file: %s\n", err)
	}
	m, err := parse(bytes.NewReader(f))
	if err != nil {
		t.Fatal(err)
	}
	want := "1.2.3"
	got := m["product_version"]
	if want != got {
		t.Fatalf("want %q but got %q\n", want, got)
	}
}

func TestControlfileTemplate(t *testing.T) {
	const (
		key     = "product_version"
		version = "0.0.0"
	)

	// Parse template control file
	buf, err := ioutil.ReadFile("testdata/template/OPSI/control")
	if err != nil {
		t.Fatalf("cannot read control file: %s\n", err)
	}

	// Resolve
	m := make(Metadata)
	m[key] = version
	r := resolve(string(buf), m)

	// Parse resolved template file
	m2, err := parse(&r)
	if err != nil {
		t.Fatal(err)
	}
	want := version
	got := m2[key]
	if want != got {
		t.Fatalf("want %q but got %q\n", want, got)
	}
}
