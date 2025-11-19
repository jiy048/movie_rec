package main

import (
    "log"
    "net/http"
    "net/http/httputil"
    "net/url"
)

func main() {
    // target: Google
    target, err := url.Parse("https://www.google.com")
    if err != nil {
        log.Fatal("failed to parse target:", err)
    }

    proxy := httputil.NewSingleHostReverseProxy(target)

    // all path /、/search、/url、/img、……
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        // change http request to google
        r.Host = target.Host
        r.URL.Scheme = target.Scheme
        r.URL.Host = target.Host

        log.Println("Proxying:", r.Method, r.URL.String())
        proxy.ServeHTTP(w, r)
    })

    addr := ":8080"
    log.Println("Google proxy listening on", addr)
    if err := http.ListenAndServe(addr, nil); err != nil {
        log.Fatal(err)
    }
}
