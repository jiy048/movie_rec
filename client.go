package main

import (
    "io"
    "log"
    "net/http"
    "net/url"
)

// 上游目标：Google
var targetBase = &url.URL{
    Scheme: "https",
    Host:   "www.google.com",
}

// 自己维护一个 http.Client，用 CheckRedirect 让它「不要自己跟随 3xx」
// 这样行为更接近 ReverseProxy：重定向由浏览器自己处理。
var client = &http.Client{
    CheckRedirect: func(req *http.Request, via []*http.Request) error {
        // 告诉 client：遇到 3xx 就停在这一步，把这个响应直接返回给我们
        return http.ErrUseLastResponse
    },
}

func main() {
    // 所有路径都走同一个 handler
    http.HandleFunc("/", proxyHandler)

    addr := ":8080"
    log.Println("Google proxy (http.Client version) listening on", addr)
    if err := http.ListenAndServe(addr, nil); err != nil {
        log.Fatal(err)
    }
}

// proxyHandler 把打到 localhost:8080 的请求转发到 https://www.google.com
func proxyHandler(w http.ResponseWriter, r *http.Request) {
    // 1. 组装上游 URL：保留 path + query，但 host/scheme 换成 Google
    upstreamURL := *targetBase // 复制一份 base
    upstreamURL.Path = r.URL.Path
    upstreamURL.RawQuery = r.URL.RawQuery

    log.Println("Proxying:", r.Method, upstreamURL.String())

    // 2. 构造发给 Google 的新请求
    //    注意：body 直接复用 r.Body（对 GET/HEAD 来说本来就是 nil）
    req, err := http.NewRequest(r.Method, upstreamURL.String(), r.Body)
    if err != nil {
        log.Println("failed to build upstream request:", err)
        http.Error(w, "failed to build upstream request", http.StatusInternalServerError)
        return
    }

    // 2.1 拷贝客户端的 header 到上游请求（简单粗暴一把梭）
    //     保持 User-Agent / Cookies 等，大部分情况下够用
    for k, vv := range r.Header {
        for _, v := range vv {
            req.Header.Add(k, v)
        }
    }

    // 2.2 Host 也改成上游的 host
    req.Host = targetBase.Host

    // 3. 发请求给 Google
    resp, err := client.Do(req)
    if err != nil {
        log.Println("failed to call upstream:", err)
        http.Error(w, "failed to call upstream", http.StatusBadGateway)
        return
    }
    defer resp.Body.Close()

    // 4. 把上游响应的 header 复制给浏览器
    for k, vv := range resp.Header {
        for _, v := range vv {
            w.Header().Add(k, v)
        }
    }

    // 5. 写回相同的状态码（200 / 302 / 404 等）
    w.WriteHeader(resp.StatusCode)

    // 6. 把 body 流式拷贝回去（不做任何修改）
    if _, err := io.Copy(w, resp.Body); err != nil {
        log.Println("failed to copy response body:", err)
    }
}
