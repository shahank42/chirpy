package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
)

type apiConfig struct {
	fileServerHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	body := fmt.Sprintf(`
<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, cfg.fileServerHits.Load())
	w.Write([]byte(body))
}

func (cfg *apiConfig) handlerResetMetrics(w http.ResponseWriter, r *http.Request) {
	cfg.fileServerHits.Store(0)
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type errorResponse struct {
		Error string `json:"error"`
	}

	res := errorResponse{
		Error: msg,
	}

	dat, err := json.Marshal(res)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

func filterProfanity(s string) string {
	profaneWords := []string{
		"kerfuffle",
		"sharbert",
		"fornax",
	}

	splitStrings := strings.Split(s, " ")

	for idx, word := range splitStrings {
		for _, profaneWord := range profaneWords {
			if strings.ToLower(word) == profaneWord {
				splitStrings[idx] = "****"
			}
		}
	}

	return strings.Join(splitStrings, " ")
}

func main() {
	apiCfg := apiConfig{}
	mux := http.NewServeMux()

	mux.Handle("/app/",
		apiCfg.middlewareMetricsInc(
			http.StripPrefix("/app/",
				http.FileServer(http.Dir(".")),
			),
		),
	)

	mux.HandleFunc("GET /api/healthz",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		},
	)

	mux.HandleFunc("POST /api/validate_chirp",
		func(w http.ResponseWriter, r *http.Request) {
			type parameters struct {
				Body string `json:"body"`
			}

			decoder := json.NewDecoder(r.Body)
			params := parameters{}
			err := decoder.Decode(&params)
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, "Something went wrong")
				return
			}

			if len(params.Body) > 140 {
				respondWithError(w, http.StatusBadRequest, "Chirp is too long")
				return
			}

			CleanedBody := filterProfanity(params.Body)

			type Response struct {
				CleanedBody string `json:"cleaned_body"`
			}

			res := Response{
				CleanedBody: CleanedBody,
			}

			respondWithJSON(w, http.StatusOK, res)
		},
	)

	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)

	mux.HandleFunc("POST /admin/reset", apiCfg.handlerResetMetrics)

	server := http.Server{Addr: ":8080", Handler: mux}
	server.ListenAndServe()
}
