package main

import (
	"log"

	"hpad-app/internal/desktopapp"
)

func main() {
	if err := desktopapp.Run(); err != nil {
		log.Fatal(err)
	}
}
