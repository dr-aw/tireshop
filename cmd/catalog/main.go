package main

import (
	"log"
	"tireshop/internal/runtimeapp"
)

func main() {
	if err := runtimeapp.Run("catalog"); err != nil {
		log.Fatal(err)
	}
}
