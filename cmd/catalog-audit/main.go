// catalog-audit only reads source files. It never connects to the database.
package main

import (
	"encoding/json/v2"
	"fmt"
	"os"
	"tireshop/internal/catalogsource"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: catalog-audit WORKBOOK.xlsx [...]")
		os.Exit(2)
	}
	books := []catalogsource.Workbook{}
	for _, file := range os.Args[1:] {
		book, err := catalogsource.Read(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		books = append(books, book)
	}
	b, err := json.Marshal(books)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err = os.Stdout.Write(append(b, '\n')); err != nil {
		os.Exit(1)
	}
}
