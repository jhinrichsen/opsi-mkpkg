// Create an OPSI package without any external dependencies
// Exit codes:
// 1: error (unspecific)
// 2: bad usage
// 3: unusable "from" input directory

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Metadata holds a key value map
type Metadata map[string]string

func main() {
	flag.Usage = usage
	opsidir := flag.String("opsidir", "./OPSI", "OPSI directory used as input")
	datadir := flag.String("datadir", "./CLIENT_DATA", "data directory used as input")
	control := flag.String("control", "./OPSI/control", "OPSI control file")
	into := flag.String("into", ".", "OPSI package destination directory")
	keep := flag.Bool("keep", false, "keep OPSI interim workbench for debugging purposes")
	flag.Parse()

	cmdMeta, err := parseArgs()
	if err != nil {
		log.Fatalf("cannot parse commandline: %s\n", err)
	}

	// Check input directories
	if !exists(*opsidir) {
		fmt.Fprintf(os.Stderr, "missing expected directory %s\n", *opsidir)
		os.Exit(3)
	}
	if !exists(*datadir) {
		fmt.Fprintf(os.Stderr, "missing expected directory %s\n", *datadir)
		os.Exit(3)
	}
	if !exists(*control) {
		fmt.Fprintf(os.Stderr, "missing controlfile %s\n", *control)
		os.Exit(3)
	}
	controlfile, err := ioutil.ReadFile(*control)
	if err != nil {
		log.Fatalf("error reading %s: %s\n", *control, err)
	}
	log.Printf("using control file %s, into %s\n", *control, *into)

	// Resolve controlfile template parameters
	rs := resolve(string(controlfile), cmdMeta)

	workbench, err := tmpDir()
	if err != nil {
		log.Fatalf("cannot create workbench: %s\n", err)
	}
	defer func(path string, remove bool) {
		if remove {
			log.Printf("removing workbench %s\n", path)
			removeDir(path)
		} else {
			log.Printf("keeping workbench %s\n", path)
		}
	}(workbench, !*keep)

	if err := mkpkg(rs.Bytes(), *opsidir, *datadir, *into, workbench); err != nil {
		log.Fatal(err)
	}
}

func compress(intoFilename string, fromFilename string) (os.FileInfo, error) {
	log.Printf("compressing %s from %s\n", intoFilename, fromFilename)
	from, err := os.Open(fromFilename)
	if err != nil {
		return nil, fmt.Errorf("error opening %s: %s", fromFilename, err)
	}
	defer from.Close()

	into, err := os.Create(intoFilename)
	if err != nil {
		return nil, fmt.Errorf("error creating %s: %s", intoFilename, err)
	}
	defer into.Close()
	w := gzip.NewWriter(into)
	n, err := io.Copy(w, from)
	if err != nil {
		return nil, fmt.Errorf("error writing to %s: %s", intoFilename, err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("error closing %s: %s", intoFilename, err)
	}

	fi, err := os.Stat(intoFilename)
	if err != nil {
		return nil, fmt.Errorf("cannot stat() %s: %s", intoFilename, err)
	}

	var factor float64
	if fi.Size() == 0 {
		factor = 1.0
	} else {
		factor = float64(n) / float64(fi.Size())
	}
	log.Printf("compress factor %.1f\n", factor)
	return fi, nil
}

// exists returns whether the given file or directory exists or not
func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	panic(err)
}

// return filename for final OPSI package, which is derived from package's metadata
func filename(m Metadata) string {
	return fmt.Sprintf("%s_%s-%s.opsi",
		get(m, "product_id"),
		get(m, "product_version"),
		get(m, "package_version"))
}

func get(m Metadata, key string) string {
	value := m[key]
	if len(value) == 0 {
		log.Fatalf("missing required value for key %q in metadata", key)
	}
	return value
}

// no care has been taken to minimize interim file creation. Cowardly refusing
// to stack tar writer into gzip writer because documentation says:
// "Writes may be buffered and not flushed until Close."
// This is definitely not something we want, so just to be on the safe side,
// each and every tar and gzip step starts from and ends in the file system.
// TODO research io.Pipe() approach
func mkpkg(controlfile []byte, opsidir, datadir, into, tmpdir string) error {
	// create OPSI.tar
	opsiTarFilename := filepath.Join(tmpdir, "OPSI.tar")
	log.Printf("creating %s\n", opsiTarFilename)
	opsiTarFile, err := os.Create(opsiTarFilename)
	if err != nil {
		return fmt.Errorf("cannot create %s: %s", opsiTarFilename, err)
	}
	tw := tar.NewWriter(opsiTarFile)
	if err := writeControlfile(tw, controlfile); err != nil {
		return fmt.Errorf("error writing control file to %s: %s", opsiTarFilename, err)
	}
	if err := write(tw, opsidir, true); err != nil {
		return fmt.Errorf("error writing control file to %s: %s", opsiTarFilename, err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("error closing %s: %s", opsiTarFilename, err)
	}

	// create OPSI.tar.gz
	opsiTarGzFilename := filepath.Join(tmpdir, "OPSI.tar.gz")
	_, err = compress(opsiTarGzFilename, opsiTarFilename)
	if err != nil {
		return fmt.Errorf("cannot compress %s into %s: %s", opsiTarFilename, opsiTarGzFilename, err)
	}

	// create CLIENT_DATA.tar
	clientTarFilename := filepath.Join(tmpdir, "CLIENT_DATA.tar")
	log.Printf("creating %s\n", clientTarFilename)
	clientTarFile, err := os.Create(clientTarFilename)
	if err != nil {
		return fmt.Errorf("cannot create %s: %s", clientTarFilename, err)
	}
	ctw := tar.NewWriter(clientTarFile)
	if err := write(ctw, datadir, false); err != nil {
		return fmt.Errorf("cannot write %s: %s", clientTarFilename, err)
	}
	if err := ctw.Close(); err != nil {
		return fmt.Errorf("error closing %s: %s", clientTarFilename, err)
	}

	// create CLIENT_DATA.tar.gz
	clientTarGzFilename := filepath.Join(tmpdir, "CLIENT_DATA.tar.gz")
	log.Printf("creating %s from %s\n", clientTarGzFilename, clientTarFilename)
	_, err = compress(clientTarGzFilename, clientTarFilename)
	if err != nil {
		return fmt.Errorf("cannot compress %s into %s: %s", clientTarFilename, clientTarGzFilename, err)
	}

	// create final OPSI package
	m, err := parse(bytes.NewReader(controlfile))
	if err != nil {
		return fmt.Errorf("error parsing controlfile: %s", err)
	}
	opsiPath := filepath.Join(into, filename(m))
	opsiFile, err := os.Create(opsiPath)
	if err != nil {
		return fmt.Errorf("error creating %s: %s", opsiPath, err)
	}
	otw := tar.NewWriter(opsiFile)
	if err := write(otw, opsiTarGzFilename, false); err != nil {
		return fmt.Errorf("error writing %s to %s: %s", opsiTarGzFilename, opsiPath, err)
	}
	if err := write(otw, clientTarGzFilename, false); err != nil {
		return fmt.Errorf("error writing %s to %s: %s", clientTarGzFilename, opsiPath, err)
	}
	if err := otw.Close(); err != nil {
		return fmt.Errorf("error closing %s: %s", opsiPath, err)
	}
	log.Printf("created OPSI package %s\n", opsiPath)
	return nil
}

// returns list of key value pairs processed so far in case of error
func parseArgs() (Metadata, error) {
	m := make(Metadata)
	for _, p := range flag.Args() {
		parts := strings.Split(p, "=")
		if len(parts) != 2 {
			return m, fmt.Errorf("want key=value, got %q", p)
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		m[key] = value
	}
	return m, nil
}

func removeDir(path string) {
	if err := os.RemoveAll(path); err != nil {
		log.Fatal(fmt.Errorf("cannot remove %s: %s", path, err))
	}
}

func resolve(controlfile string, m Metadata) bytes.Buffer {
	t := template.New("controlfile")
	tmpl, err := t.Parse(controlfile)
	if err != nil {
		log.Fatalf("error in control file: %s\n", err)
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, m)
	if err != nil {
		log.Fatalf("executing control file template: %s\n", err)
	}
	return buf
}

func tmpDir() (string, error) {
	return ioutil.TempDir("", "opsi-")
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: opsi-mkpkg [key1=value1]*\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func write(tw *tar.Writer, dir string, ignoreControlfile bool) error {
	return filepath.Walk(dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// OPSI uses flat structure only
			if info.IsDir() {
				return nil
			}

			if ignoreControlfile && info.Name() == "control" {
				log.Printf("skipping control file %s\n", path)
				return nil
			}
			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return err
			}

			if err := tw.WriteHeader(header); err != nil {
				return err
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			_, err = io.Copy(tw, file)
			if err := file.Close(); err != nil {
				log.Printf("ignoring error closing file %s: %s\n", path, err)
			}
			return err
		})
}

func writeControlfile(tw *tar.Writer, controlfile []byte) error {
	l := len(controlfile)
	log.Printf("writing %d bytes controlfile to tar\n", l)
	// synthetic, i.e. no underlying file information
	hdr := &tar.Header{
		Name:    "control",
		Mode:    0600,
		ModTime: time.Now(),
		Size:    int64(l),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("cannot append header for control file: %s", err)
	}
	n, err := tw.Write(controlfile)
	if err != nil {
		return fmt.Errorf("cannot write content of control file: %s", err)
	}
	log.Printf("wrote %d bytes\n", n)
	return nil
}
