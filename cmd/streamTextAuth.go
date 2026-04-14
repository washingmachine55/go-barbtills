package cmd

import (
	"math/rand"
	"net/http"
	"strings"
)


var sessionToken string

var wordlist = []string{
    "apple", "river", "stone", "cloud", "flame",
    "ghost", "tiger", "lunar", "frost", "ember",
    "cedar", "pixel", "storm", "haven", "drift",
    "crane", "noble", "swift", "amber", "vivid",
    // add more for better entropy
}

func generatePassphrase() string {
    words := make([]string, 3)
    for i := range words {
        words[i] = wordlist[rand.Intn(len(wordlist))]
    }
    return strings.Join(words, "-")
}

func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Skip auth for assets
        if strings.HasPrefix(r.URL.Path, "/assets/") {
            next.ServeHTTP(w, r)
            return
        }

        token := r.Header.Get("X-Session-Token")
        if token != sessionToken {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        next.ServeHTTP(w, r)
    })
}