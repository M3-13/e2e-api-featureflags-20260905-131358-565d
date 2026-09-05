package main

import "net/http"

func Logging(next http.Handler) http.Handler {
	return next
}
