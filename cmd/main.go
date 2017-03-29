package main

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
)

type Person struct {
	Name string
}

func main() {

	defer func() {
		if recover() != nil {
			fmt.Println("An unhandled exception was caught.")
		}
	}()

	buf := &bytes.Buffer{}

	e := make(map[string]string)

	e["USERNAME"] = "$( who | awk '{ print $1 }' )"
	e["HOSTNAME"] = "$( hostname -s )"

	t, err := template.ParseFiles(os.Args[1])
	if err != nil {
		fmt.Println("# There was an error:", err)
		os.Exit(1)
	}

	err = t.Execute(buf, e)
	if err != nil {
		fmt.Println("# There was an error:", err)
	} else {
		buf.WriteTo(os.Stdout)
	}
}
