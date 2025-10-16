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

func unauthorizedHandler(w http.ResponseWriter, r *http.Request) {
	code := http.StatusUnauthorized
	log.Print("Unauthorized error handled")
	http.Error(w, http.StatusText(code), code)
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	code := http.StatusNotFound
	http.Error(w, http.StatusText(code), code)
}
