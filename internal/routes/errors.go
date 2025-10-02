package routes

import (
	"log"
	"net/http"
)

func serverError(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	log.Printf("received error: {%v}", err)
	http.Error(w, http.StatusText(code), code)
}

func badRequestError(w http.ResponseWriter, err error) {
	code := http.StatusBadRequest
	log.Printf("received error: {%v}", err)
	http.Error(w, http.StatusText(code), code)
}
