package main

import (
    "io"
    "log"
    "net/http"
    "net/url"
    "strings"
)

// 最简单的首页：给你一个搜索框，提交到 /search?q=xxx
func indexHandler(w http.ResponseWriter, r *http.Request) {
    html := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Mini Google Relay</title>
</head>
<body>
    <h1>Mini Google Relay</h1>
    <form action="/search" method="get">
        <input type="text" name="q" placeholder="Type your query" style="width: 300px;">
        <button type="submit">Search</button>
    </form>
</body>
</html>`
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    _, _ = io.WriteString(w, html)
}

// /search?q=xxx -> 转发到 https://www.google.com/search?q=xxx
func googleSearchRelayHandler(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query().Get("q")
    if strings.TrimSpace(q) == "" {
        http.Error(w, "missing query parameter q", http.StatusBadRequest)
        return
    }

    // 构造 Google 搜索 URL，需要 URL encode
    googleURL := "https://www.google.com/search?q=" + url.QueryEscape(q)
    log.Println("Relaying search to:", googleURL)

    // 用单独的 client，方便以后加超时等配置
    client := &http.Client{}

    // 构造发给 Google 的请求
    req, err := http.NewRequest("GET", googleURL, nil)
    if err != nil {
        log.Println("failed to build request:", err)
        http.Error(w, "failed to build request", http.StatusInternalServerError)
        return
    }

    // 设置一个 User-Agent，避免被部分站点直接拒绝
    req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; MiniGoogleRelay/0.1)")

    // 发送请求到 Google
    resp, err := client.Do(req)
    if err != nil {
        log.Println("failed to call google:", err)
        http.Error(w, "failed to call google", http.StatusBadGateway)
        return
    }
    defer resp.Body.Close()

    // 把 Google 的响应 header 转发给客户端
    for k, vv := range resp.Header {
        // 简单粗暴地整体拷贝，大部分情况下够用
        for _, v := range vv {
            w.Header().Add(k, v)
        }
    }

    // 写入相同的状态码
    w.WriteHeader(resp.StatusCode)

    // 把 body 直接拷贝过去
    if _, err := io.Copy(w, resp.Body); err != nil {
        log.Println("failed to copy body:", err)
    }
}

func main() {
    http.HandleFunc("/", indexHandler)
    http.HandleFunc("/search", googleSearchRelayHandler)

    addr := ":8080"
    log.Println("Server running on", addr)
    if err := http.ListenAndServe(addr, nil); err != nil {
        log.Fatal(err)
    }
}
