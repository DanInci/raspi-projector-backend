package server

import (
	http "net/http"
)

const (
	DEFAULT_CORS_ALLOW_ORIGIN  = "*"
	DEFAULT_CORS_ALLOW_METHODS = "POST, GET, OPTIONS, PUT, DELETE"
	DEFAULT_CORS_ALLOW_HEADERS = "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Access-Token"
)

func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", DEFAULT_CORS_ALLOW_ORIGIN)
		w.Header().Set("Access-Control-Allow-Methods", DEFAULT_CORS_ALLOW_METHODS)
		w.Header().Set("Access-Control-Allow-Headers", DEFAULT_CORS_ALLOW_HEADERS)

		// If this was a preflight request, stop further middleware execution
		if r.Method == "OPTIONS" {
			return
		}

		next.ServeHTTP(w, r)
	})
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Logger.Infof("%s \"%s %s %s\"", r.RemoteAddr, r.Method, r.RequestURI, r.Proto)
		next.ServeHTTP(w, r)
	})
}
