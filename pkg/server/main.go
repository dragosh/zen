package server

import (
	"log"
	"net/http"
	"os"
	"os/user"
)

func main() {

	// Serve static files
	http.Handle("/web/", fs)
	http.HandleFunc("/exit", func(w http.ResponseWriter, r *http.Request) {
		os.Exit(0)
	})
	// Handle all other requests
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		var path = req.URL.Path
		log.Println("Serving request for path", path)
		w.Header().Add("Content-Type", "text/html")
		u, _ := user.Current()

		// respond with the output of template execution
		t.Execute(w, struct {
			Title    string
			Response string
			User     *user.User
		}{Title: title, Response: path, User: u})

		err = t.ExecuteTemplate(w, "Markdown", buf.String())
		check(err)
	})

	log.Fatal(http.Serve(ln, nil))
}
