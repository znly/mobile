// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"go/build"
	"go/importer"
	"go/types"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/mobile/internal/importers"
	"golang.org/x/mobile/internal/importers/java"
	"golang.org/x/mobile/internal/importers/objc"
)

var (
	lang          = flag.String("lang", "", "target languages for bindings, either java, go, or objc. If empty, all languages are generated.")
	outdir        = flag.String("outdir", "", "result will be written to the directory instead of stdout.")
	javaPkg       = flag.String("javapkg", "", "custom Java package path prefix. Valid only with -lang=java.")
	prefix        = flag.String("prefix", "", "custom Objective-C name prefix. Valid only with -lang=objc.")
	bootclasspath = flag.String("bootclasspath", "", "Java bootstrap classpath.")
	classpath     = flag.String("classpath", "", "Java classpath.")
	tags          = flag.String("tags", "", "build tags.")
	goinstall     = flag.Bool("goinstall", true, "try to go install the package first.")
)

var usage = `The Gobind tool generates Java language bindings for Go.

For usage details, see doc.go.`

func main() {
	flag.Parse()

	run()
	os.Exit(exitStatus)
}

func run() {
	var langs []string
	if *lang != "" {
		langs = strings.Split(*lang, ",")
	} else {
		langs = []string{"go", "java", "objc"}
	}
	oldCtx := build.Default
	ctx := &build.Default
	if *tags != "" {
		ctx.BuildTags = append(ctx.BuildTags, strings.Split(*tags, ",")...)
	}
	var allPkg []*build.Package
	for _, path := range flag.Args() {
		pkg, err := ctx.Import(path, ".", build.ImportComment)
		if err != nil {
			log.Fatalf("package %q: %v", path, err)
		}
		allPkg = append(allPkg, pkg)
	}
	jrefs, err := importers.AnalyzePackages(allPkg, "Java/")
	if err != nil {
		log.Fatal(err)
	}
	orefs, err := importers.AnalyzePackages(allPkg, "ObjC/")
	if err != nil {
		log.Fatal(err)
	}
	var classes []*java.Class
	if len(jrefs.Refs) > 0 {
		jimp := &java.Importer{
			Bootclasspath: *bootclasspath,
			Classpath:     *classpath,
			JavaPkg:       *javaPkg,
		}
		classes, err = jimp.Import(jrefs)
		if err != nil {
			log.Fatal(err)
		}
	}
	var otypes []*objc.Named
	if len(orefs.Refs) > 0 {
		otypes, err = objc.Import(orefs)
		if err != nil {
			log.Fatal(err)
		}
	}
	if len(classes) > 0 || len(otypes) > 0 {
		// After generation, reverse bindings needs to be in the GOPATH
		// for user packages to build.
		tmpGopath, err := ioutil.TempDir(os.TempDir(), "gobind-")
		if err != nil {
			log.Fatal(err)
		}
		defer os.RemoveAll(tmpGopath)
		if ctx.GOPATH != "" {
			ctx.GOPATH += string(filepath.ListSeparator)
		}
		ctx.GOPATH += tmpGopath
		if len(classes) > 0 {
			if err := genJavaPackages(ctx, tmpGopath, classes, jrefs.Embedders); err != nil {
				log.Fatal(err)
			}
		}
		if len(otypes) > 0 {
			if err := genObjcPackages(ctx, tmpGopath, otypes, orefs.Embedders); err != nil {
				log.Fatal(err)
			}
		}
	}
	if *outdir != "" {
		d, err := filepath.Abs(*outdir)
		if err != nil {
			fmt.Printf("%v\n", err)
		}
		ctx.GOPATH += string(filepath.ListSeparator) + d
	}

	if *goinstall {
		// Make sure the export data for any imported packages are up to date.
		cmd := exec.Command("go", "install", "-tags", strings.Join(ctx.BuildTags, " "))
		cmd.Args = append(cmd.Args, flag.Args()...)
		cmd.Env = append(os.Environ(), "GOPATH="+ctx.GOPATH)
		cmd.Env = append(cmd.Env, "GOROOT="+ctx.GOROOT)
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "%s", out)
			exitStatus = 1
			return
		}
	}

	typePkgs := make([]*types.Package, len(allPkg))
	imp := importer.Default()
	for i, pkg := range allPkg {
		var err error
		typePkgs[i], err = imp.Import(pkg.ImportPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return
		}
	}
	build.Default = oldCtx
	for _, l := range langs {
		for _, pkg := range typePkgs {
			genPkg(l, pkg, typePkgs, classes, otypes)
		}
		// Generate the error package and support files
		genPkg(l, nil, typePkgs, classes, otypes)
	}
}

var exitStatus = 0

func errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprintln(os.Stderr)
	exitStatus = 1
}
