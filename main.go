package main

import (
	"github.com/glgaspar/front_desk/router"
	"log"
	"net/http"

	"github.com/glgaspar/front_desk/controller"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Printf("error reading .env %s", err.Error())
	}

	http.HandleFunc("/", router.Root)
	http.HandleFunc("/paychecker", router.ShowPayChecker)
	http.HandleFunc("/paychecker/flipTrack/:billId", controller.FlipPayChecker)
	http.HandleFunc("/paychecker/new", controller.NewPayChecker)

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	log.Fatal(http.ListenAndServe(":8080", nil))
}
