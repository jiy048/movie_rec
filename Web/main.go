package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"io"
	"net/http"
	"os"
)


type esHit struct {
	Source struct {
		Text string `json:"text"`
	} `json:"_source"`
}

type esHits struct {
	Hits []esHit `json:"hits"`
}

type esSearchResponse struct {
	Hits esHits `json:"hits"`
}


func getESURL() string {
	url := os.Getenv("ELASTICSEARCH_URL")
	if url == "" {
		url = "http://localhost:9200"
	}
	return url
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, "missing query parameter ?q=", http.StatusBadRequest)
		return
	}

	esURL := getESURL()

	searchURL := fmt.Sprintf("%s/movies/_search", esURL)


	body := map[string]interface{}{
		"query": map[string]interface{}{
			"match": map[string]interface{}{
				"text": map[string]interface{}{
					"query": q,
				},
			},
		},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		http.Error(w, "failed to encode query body", http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequest(http.MethodPost, searchURL, bytes.NewReader(buf.Bytes()))
	if err != nil {
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	log.Println("[Go->ES] Method:", req.Method)
	log.Println("[Go->ES] URL:", req.URL.String())
	log.Println("[Go->ES] Content-Type:", req.Header.Get("Content-Type"))
	log.Println("[Go->ES] Body:", string(buf.Bytes()))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "failed to call Elasticsearch: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		http.Error(w, "Elasticsearch returned status "+resp.Status, http.StatusBadGateway)
		return
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "failed to read Elasticsearch response body", http.StatusInternalServerError)
		return
	}

	// 打印 ES 返回的原始 JSON（可读性更好）
	log.Println("[ES] Raw response status:", resp.Status)
	log.Println("[ES] Raw response body:\n" + string(raw))

	// 再用 raw 去 decode（因为 resp.Body 已经被 ReadAll 读空了）
	var esResp esSearchResponse
	if err := json.Unmarshal(raw, &esResp); err != nil {
		http.Error(w, "failed to decode Elasticsearch response", http.StatusInternalServerError)
		return
	}



	var results []string
	for _, h := range esResp.Hits.Hits {
		results = append(results, h.Source.Text)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"query":   q,
		"results": results,
	}); err != nil {
		log.Println("failed to write response:", err)
	}
}

func autocompleteHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, "missing query parameter ?q=", http.StatusBadRequest)
		return
	}

	esURL := getESURL()
	searchURL := fmt.Sprintf("%s/movies/_search", esURL)

	body := map[string]interface{}{
		"suggest": map[string]interface{}{
			"movie-suggest": map[string]interface{}{ 
				"prefix": q,
				"completion": map[string]interface{}{
					"field":           "suggest",
					"size":            5,
					"skip_duplicates": true,
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		http.Error(w, "failed to encode suggest body", http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequest(http.MethodPost, searchURL, &buf)
	if err != nil {
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "failed to call Elasticsearch: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		http.Error(w, "Elasticsearch returned status "+resp.Status, http.StatusBadGateway)
		return
	}

	var esResp esSuggestResponse
	if err := json.NewDecoder(resp.Body).Decode(&esResp); err != nil {
		http.Error(w, "failed to decode Elasticsearch response", http.StatusInternalServerError)
		return
	}

	entries := esResp.Suggest["movie-suggest"]
	suggestions := make([]string, 0, 5)
	if len(entries) > 0 {
		for _, opt := range entries[0].Options {
			suggestions = append(suggestions, opt.Text)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"query":       q,
		"suggestions": suggestions,
	})
}



func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>ES Search Demo</title>
  <style>
    body { font-family: Arial, sans-serif; }
    .box { position: relative; width: 420px; }
    #q { width: 100%%; padding: 10px; font-size: 16px; }
    #suggestions {
      position: absolute;
      top: 42px;
      left: 0;
      right: 0;
      border: 1px solid #ddd;
      background: #fff;
      list-style: none;
      margin: 0;
      padding: 0;
      max-height: 220px;
      overflow-y: auto;
      display: none;
      z-index: 10;
    }
    #suggestions li {
      padding: 8px 10px;
      cursor: pointer;
    }
    #suggestions li:hover {
      background: #f2f2f2;
    }
    .row { display: flex; gap: 8px; margin-top: 10px; }
    button { padding: 10px 14px; font-size: 16px; }
  </style>
</head>
<body>
  <h1>Elasticsearch Search Demo</h1>

  <div class="box">
    <form id="searchForm" action="/search" method="get" autocomplete="off">
      <input id="q" type="text" name="q" placeholder="Type search term..." />
      <ul id="suggestions"></ul>
      <div class="row">
        <button type="submit">Search</button>
      </div>
    </form>
  </div>

<script>
(function() {
  const input = document.getElementById("q");
  const list  = document.getElementById("suggestions");
  const form  = document.getElementById("searchForm");

  let debounceTimer = null;

  function hideList() {
    list.style.display = "none";
    list.innerHTML = "";
  }

  function showList(items) {
    list.innerHTML = "";
    if (!items || items.length === 0) {
      hideList();
      return;
    }

    for (const s of items) {
      const li = document.createElement("li");
      li.textContent = s;
      li.addEventListener("mousedown", (e) => {
        // mousedown 比 click 更稳：避免 input blur 先触发导致列表消失
        e.preventDefault();
        input.value = s;
        hideList();
        form.submit(); // 选中建议后直接触发 search（想不自动搜索就删掉这行）
      });
      list.appendChild(li);
    }
    list.style.display = "block";
  }

  async function fetchSuggestions(q) {
    const resp = await fetch("/autocomplete?q=" + encodeURIComponent(q));
    if (!resp.ok) return [];
    const data = await resp.json();
    return data.suggestions || [];
  }

  input.addEventListener("input", () => {
    const q = input.value.trim();
    if (debounceTimer) clearTimeout(debounceTimer);

    if (q.length === 0) {
      hideList();
      return;
    }

    debounceTimer = setTimeout(async () => {
      const items = await fetchSuggestions(q);
      showList(items);
    }, 120); // 120ms debounce，避免每个按键都打爆后端
  });

  // 点击输入框外面就关闭 suggestions
  document.addEventListener("click", (e) => {
    if (!list.contains(e.target) && e.target !== input) {
      hideList();
    }
  });

  // 按下 ESC 关闭列表
  input.addEventListener("keydown", (e) => {
    if (e.key === "Escape") hideList();
  });
})();
</script>

</body>
</html>
`)
}


func main() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/autocomplete", autocompleteHandler) 

	addr := ":8080"
	log.Println("Starting web server on", addr)
	log.Println("Elasticsearch URL:", getESURL())
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}


type esSuggestOption struct {
	Text string `json:"text"`
}

type esSuggestEntry struct {
	Options []esSuggestOption `json:"options"`
}

type esSuggestResponse struct {
	Suggest map[string][]esSuggestEntry `json:"suggest"`
}





