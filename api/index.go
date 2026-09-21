package handler

import (
	"io"
	"net/http"
	"sync"

	"aviator/internal/server"
)

var (
	appOnce sync.Once
	app     *server.FiberServer
)

func getApp() *server.FiberServer {
	appOnce.Do(func() {
		app = server.New()
		app.RegisterFiberRoutes()
	})
	return app
}

func Handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"service":"aviator","status":"online"}`)
		return
	}

	response, err := getApp().Test(r, -1)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer response.Body.Close()

	for key, values := range response.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}

var _ http.Handler = http.HandlerFunc(Handler)
