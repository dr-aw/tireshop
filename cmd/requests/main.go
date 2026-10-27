package main

import (
	"log"
	"tireshop/internal/runtimeapp"
)

func main() {
	if err := runtimeapp.Run("requests"); err != nil {
		log.Fatal(err)
	}
}
