package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

/*
最小可用目标：
1) Go 服务在容器内监听 PORT（默认 8080）
2) 读 ES_URLS（你的 compose 里会给 http://es-coord:9200）
3) 提供 /autocomplete?q= 前缀查询，转发到 ES 的 completion suggester
4) 返回 JSON 里带 host，用来验证 Nginx round-robin 到哪个 web 容器
*/

// -------------------------
// 读取运行时配置（来自 docker-compose 的 environment）
// -------------------------

func getPort() string {
	// 1) 从环境变量 PORT 读取端口号（例如 "8080"）
	p := strings.TrimSpace(os.Getenv("PORT"))
	if p == "" {
		// 2) 没设置就用默认 8080（方便本地跑）
		p = "8080"
	}
	// 3) 允许用户写成 ":8080" 或 "8080"，统一返回 "8080"
	if strings.HasPrefix(p, ":") {
		return p[1:]
	}
	return p
}

func getESBaseURL() string {
	// 1) 从环境变量 ES_URLS 读取 ES 地址（你现在只放一个：http://es-coord:9200）
	raw := strings.TrimSpace(os.Getenv("ES_URLS"))
	if raw == "" {
		// 2) 本地调试默认走 localhost
		return "http://localhost:9200"
	}
	// 3) ES_URLS 可能是逗号分隔，这里“最小版”只取第一个
	first := strings.Split(raw, ",")[0]
	return strings.TrimRight(strings.TrimSpace(first), "/")
}

func getHost() string {
	// 返回当前容器的 hostname（用来验证 Nginx RR 分流）
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

// -------------------------
// /autocomplete：调用 ES completion suggester
// -------------------------

// ES 返回的 suggest 结构（只解析我们需要的部分）
type esSuggestResponse struct {
	Suggest map[string][]struct {
		Options []struct {
			Text string `json:"text"`
		} `json:"options"`
	} `json:"suggest"`
}

func autocompleteHandler(w http.ResponseWriter, r *http.Request) {
	// 1) 读取 query 参数 q（prefix）
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		http.Error(w, "missing query parameter ?q=", http.StatusBadRequest)
		return
	}

	// 2) 拼出 ES endpoint：/movies/_search
	esURL := getESBaseURL() + "/movies/_search"

	// 3) 构造 completion suggest 的请求体
	//    - prefix: 用户输入
	//    - completion.field: "suggest"（你 ingest 脚本建的字段）
	body := map[string]any{
		"suggest": map[string]any{
			"movie-suggest": map[string]any{
				"prefix": q,
				"completion": map[string]any{
					"field":           "suggest",
					"size":            5,
					"skip_duplicates": true,
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		http.Error(w, "failed to encode request body", http.StatusInternalServerError)
		return
	}

	// 4) POST 到 ES
	req, err := http.NewRequest(http.MethodPost, esURL, &buf)
	if err != nil {
		http.Error(w, "failed to create ES request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "failed to call Elasticsearch: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 5) ES 非 2xx 就认为失败
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		http.Error(w, "Elasticsearch returned "+resp.Status+": "+string(raw), http.StatusBadGateway)
		return
	}

	// 6) 解析 ES 返回
	var esResp esSuggestResponse
	if err := json.NewDecoder(resp.Body).Decode(&esResp); err != nil {
		http.Error(w, "failed to decode Elasticsearch response", http.StatusInternalServerError)
		return
	}

	// 7) 抽取 suggestions
	suggestions := make([]string, 0, 5)
	entries := esResp.Suggest["movie-suggest"]
	if len(entries) > 0 {
		for _, opt := range entries[0].Options {
			suggestions = append(suggestions, opt.Text)
		}
	}

	// 8) 返回 JSON（带 host 用于验证 LB 分流）
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"host":        getHost(),
		"query":       q,
		"suggestions": suggestions,
	})
}

// -------------------------
// /：最小 HTML 页面（输入框 + 调 /autocomplete）
// -------------------------

func indexHandler(w http.ResponseWriter, r *http.Request) {
	// 一个极简页面：输入时调用 /autocomplete?q=
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `
<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <title>Autocomplete MVP</title>
</head>
<body>
  <h1>Autocomplete MVP</h1>
  <input id="q" placeholder="type prefix..." />
  <pre id="out"></pre>

<script>
const input = document.getElementById("q");
const out = document.getElementById("out");

let t = null;
input.addEventListener("input", () => {
  const q = input.value.trim();
  if (t) clearTimeout(t);
  if (!q) { out.textContent = ""; return; }

  // 简单 debounce：避免每次按键都打后端
  t = setTimeout(async () => {
    const r = await fetch("/autocomplete?q=" + encodeURIComponent(q));
    const j = await r.json();
    out.textContent = JSON.stringify(j, null, 2);
  }, 120);
});
</script>
</body>
</html>
`)
}

// -------------------------
// main：注册路由并监听端口
// -------------------------

func main() {
	// 1) 启动时打印配置，方便你排查是否读到了正确的环境变量
	port := getPort()
	log.Println("host =", getHost())
	log.Println("listening on :", port)
	log.Println("es =", getESBaseURL())

	// 2) 注册路由
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/autocomplete", autocompleteHandler)

	// 3) 监听端口（容器内部监听 8080；Nginx 会转发到 web1/2/3:8080）
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
