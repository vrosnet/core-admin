package main

import (
	"bitbucket.org/coreos/core-update/types"
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
)

var cmdNewVersion = &Command{
	UsageLine: "new-version -k [key] -a [app-id] -v [version] -t [track] -p [url path] [filename]",
	Short:     "update the version database for a given file",
	Long: `
Takes a file path and some meta data and update the information used in the datastore.
	`,
}

var dryRun = cmdNewVersion.Flag.Bool("d", false, "dry run, print out the xml payload")
var key = cmdNewVersion.Flag.String("k", "", "api key for the admin user")

var appId = cmdNewVersion.Flag.String("a", "", "application id")
var version = cmdNewVersion.Flag.String("v", "", "version ")
var track = cmdNewVersion.Flag.String("t", "", "track")
var path = cmdNewVersion.Flag.String("p", "", "url path")

func init() {
	cmdNewVersion.Run = runNewVersion
}

func calculateHashes(filename string, pkg *types.Package) {
	var (
		writers []io.Writer
		hashes  []hash.Hash
	)

	push := func(h hash.Hash) {
		writers = append(writers, h)
		hashes = append(hashes, h)
	}

	push(sha256.New())
	push(sha1.New())

	in, err := os.Open(filename)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	io.Copy(io.MultiWriter(writers...), in)

	formatHash := func(hash hash.Hash) (out string) {
		return fmt.Sprintf("%x", hash.Sum(nil))
	}

	pkg.Sha256Sum = formatHash(hashes[0])
	pkg.Sha1Sum = formatHash(hashes[1])
}

func runNewVersion(cmd *Command, args []string) {
	if *dryRun == false && *key == "" {
		fmt.Printf("key or dry-run required")
		os.Exit(-1)
	}

	if *appId == "" || *version == "" || *track == "" || *path == "" {
		fmt.Printf("one of the required fields was not present\n")
		os.Exit(-1)
	}

	file := args[0]
	fileBase := filepath.Base(file)
	fi, err := os.Stat(file)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(-1)
	}

	fileSize := strconv.FormatInt(fi.Size(), 10)

	app := types.App{Id: *appId, Version: *version, Track: *track}
	pkg := types.Package{Name: fileBase, Size: fileSize, Path: *path}
	ver := types.Version{App: &app, Package: &pkg}
	calculateHashes(file, ver.Package)

	raw, err := xml.MarshalIndent(ver, "", " ")
	if err != nil {
		fmt.Printf(err.Error())
		os.Exit(-1)
	}

	body := []byte(xml.Header)
	body = append(body, raw...)

	adminURL, _ := url.Parse(updateURL.String())
	adminURL.Path = "/admin"

	req, _ := http.NewRequest("POST", adminURL.String(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "text/xml")
	req.SetBasicAuth("admin", *key)

	if *dryRun || *debug {
		req.Write(os.Stdout)
	}

	if *dryRun {
		return
	}

	client := http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		fmt.Printf(err.Error())
		os.Exit(-1)
	}

	body, _ = ioutil.ReadAll(resp.Body)
	os.Stdout.Write(body)
	fmt.Printf("\n")

	if resp.StatusCode != 200 {
		fmt.Printf("Error: bad return code %s\n", resp.Status)
		os.Exit(-1)
	}

	return
}
