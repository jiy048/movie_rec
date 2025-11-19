package main

import (
    "bytes"
    "io"
    "log"
    "net/http"
    "net/http/httputil"
    "net/url"
    "strconv"
    "strings"
)

func main() {
    // 目标上游是 Google
    target, err := url.Parse("https://www.google.com")
    if err != nil {
        log.Fatal("failed to parse target:", err)
    }

    proxy := httputil.NewSingleHostReverseProxy(target)

    // ===== 1. 自定义 Director：决定「发给上游的请求长什么样」 =====
    originalDirector := proxy.Director
    proxy.Director = func(req *http.Request) {
        // 先让默认 Director 干它自己的事情（拷贝原请求等）
        originalDirector(req)

        // 强制目标是 Google
        req.URL.Scheme = target.Scheme
        req.URL.Host = target.Host
        req.Host = target.Host

        // 为了方便我们改 body，这里把 Accept-Encoding 删掉，
        // 让上游尽量返回未压缩的文本（否则会是 gzip，不能直接字符串替换）
        req.Header.Del("Accept-Encoding")
    }

    // ===== 2. 自定义 ModifyResponse：改写 Google 的响应 =====
    proxy.ModifyResponse = func(resp *http.Response) error {
        // 2.1 先处理重定向 Location（302 / 301）
        if loc := resp.Header.Get("Location"); loc != "" {
            // 把跳转目标从 https://www.google.com/... 改成相对路径
            // 例如： Location: https://www.google.com/search?q=xxx
            // 变成： Location: /search?q=xxx
            newLoc := strings.ReplaceAll(loc, "https://www.google.com", "")
            newLoc = strings.ReplaceAll(newLoc, "http://www.google.com", "")
            if newLoc == "" {
                newLoc = "/"
            }
            resp.Header.Set("Location", newLoc)
        }

        // 2.2 只改写 HTML 内容（text/html），其它类型直接放行
        ct := resp.Header.Get("Content-Type")
        if !strings.HasPrefix(ct, "text/html") {
            return nil
        }

        // 读出原始 body
        bodyBytes, err := io.ReadAll(resp.Body)
        if err != nil {
            return err
        }
        _ = resp.Body.Close()

        bodyStr := string(bodyBytes)

        // *** 这里是你要的关键操作 ***
        // 把所有 "https://www.google.com/search" 替换成 "/search"
        bodyStr = strings.ReplaceAll(bodyStr, "https://www.google.com/search", "/search")
        bodyStr = strings.ReplaceAll(bodyStr, "http://www.google.com/search", "/search")

        // 你想更激进一点，也可以把整个域名都替掉：
        // bodyStr = strings.ReplaceAll(bodyStr, "https://www.google.com", "")
        // bodyStr = strings.ReplaceAll(bodyStr, "http://www.google.com", "")

        // 写回新的 body
        newBody := []byte(bodyStr)
        resp.Body = io.NopCloser(bytes.NewReader(newBody))
        resp.ContentLength = int64(len(newBody))
        resp.Header.Set("Content-Length", strconv.Itoa(len(newBody)))

        return nil
    }

    // ===== 3. HTTP handler：所有路径都交给 proxy =====
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        log.Println("Proxying:", r.Method, r.URL.String())
        proxy.ServeHTTP(w, r)
    })

    // ===== 4. 启动 server =====
    addr := ":8080"
    log.Println("Google proxy with rewrite listening on", addr)
    if err := http.ListenAndServe(addr, nil); err != nil {
        log.Fatal(err)
    }
}
