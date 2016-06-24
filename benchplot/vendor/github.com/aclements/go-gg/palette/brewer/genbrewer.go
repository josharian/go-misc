// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func main() {
	log.SetPrefix("genbrewer: ")
	log.SetFlags(0)

	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, `usage: %s colorbrewer.json

Generates brewer.go from the colors in the named JSON file, which should be
retrieved from http://colorbrewer2.org/export/colorbrewer.json.`, os.Args[0])
		os.Exit(2)
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	var brewer map[string]map[string]interface{}
	err = json.NewDecoder(f).Decode(&brewer)
	if err != nil {
		log.Fatal(err)
	}

	o, err := os.Create("brewer.go")
	if err != nil {
		log.Fatal(err)
	}

	o.WriteString(`// Generated by genbrewer. DO NOT EDIT.
// Please see license at http://colorbrewer.org/export/LICENSE.txt

package brewer

import "image/color"

`)

	nameMap := []string{}
	for _, name := range sortedKeys(brewer) {
		rawVariants := brewer[name]

		// variantKeys are strings that are mostly numbers,
		// but also have some metadata. Extract just the
		// numbers and put them in order.
		variants := []int{}
		for variant := range rawVariants {
			if num, err := strconv.Atoi(variant); err == nil {
				variants = append(variants, num)
			}
		}
		sort.Ints(variants)

		variantMap := []string{}
		var defs bytes.Buffer
		for _, variant := range variants {
			colors := rawVariants[strconv.Itoa(variant)].([]interface{})
			vname := fmt.Sprintf("%s_%d", name, variant)
			fmt.Fprintf(&defs, "\t%s = []color.Color{", vname)
			for i, color := range colors {
				if i != 0 {
					fmt.Fprintf(&defs, ", ")
				}
				r, g, b := parse(color.(string))
				fmt.Fprintf(&defs, "color.RGBA{%d, %d, %d, 255}", r, g, b)
			}
			fmt.Fprintf(&defs, "}\n")

			variantMap = append(variantMap, fmt.Sprintf("%d: %s", variant, vname))
		}

		fmt.Fprintf(o, "var (\n")

		typ := rawVariants["type"].(string)
		fmt.Fprintf(o, "\t// %s is a %s palette.\n", name, niceType[typ])
		fmt.Fprintf(o, "\t%s = map[int][]color.Color{%s}\n", name, strings.Join(variantMap, ", "))
		fmt.Fprintf(o, "%s)\n\n", defs.String())

		nameMap = append(nameMap, fmt.Sprintf("%q: %s", name, name))
	}

	fmt.Fprintf(o, "// ByName is a map indexing all palettes by string name.\n")
	fmt.Fprintf(o, "var ByName = map[string]map[int][]color.Color{%s}\n", strings.Join(nameMap, ", "))

	if err = o.Close(); err != nil {
		log.Fatal(err)
	}
}

var niceType = map[string]string{
	"seq":  "sequential",
	"div":  "diverging",
	"qual": "qualitative",
}

var colorRe = regexp.MustCompile(`^rgb\s*\(\s*([0-9]+)\s*,\s*([0-9]+)\s*,\s*([0-9]+)\s*\)\s*$`)

func parse(cssColor string) (r, g, b uint8) {
	m := colorRe.FindStringSubmatch(cssColor)
	if m == nil {
		log.Fatalf("unknown color syntax: %q", cssColor)
	}
	p := func(x string) uint8 {
		n, err := strconv.ParseUint(x, 10, 8)
		if err != nil {
			log.Fatal(err)
		}
		return uint8(n)
	}
	return p(m[1]), p(m[2]), p(m[3])
}

func sortedKeys(m interface{}) []string {
	keys := []string{}
	for _, key := range reflect.ValueOf(m).MapKeys() {
		keys = append(keys, key.String())
	}
	sort.Strings(keys)
	return keys
}