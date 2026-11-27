package future

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"time"
)

// 并发抓取实战 —— 对应 java-tutorials/callable/WebFetchExample。
//
// Java 版用真实 URL（含一个 404 站点）+ invokeAll(tasks, 5, SECONDS) 整体超时 +
// 逐个判断 isCancelled()。Go 版用 httptest 起本地服务离线复刻同样的三个形态：
// 正常 200、404、慢响应（超时被取消）—— 确定性、无外网依赖。
//
// 注意 client 构造：Transport.Proxy 置 nil，防止系统代理截走 127.0.0.1 请求
// （本地并发测试的经典坑：代理会"成功建立连接"然后无限等待）。

// fetcher httptest 服务器 + 无代理客户端的抓取器。
type fetcher struct {
	server *httptest.Server
	client *http.Client
}

// newFetcher 启动本地测试服务器（教学演示生命周期由测试负责关闭）。
func newFetcher() *fetcher {
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello concurrent"))
	})
	mux.HandleFunc("/missing", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("nope"))
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond) // 远超抓取超时 → 一定会被取消
		_, _ = w.Write([]byte("late"))
	})
	server := httptest.NewServer(mux)
	return &fetcher{
		server: server,
		client: &http.Client{Transport: &http.Transport{Proxy: nil}},
	}
}

// FetchAll 对应 WebFetchExample：并发抓取 + 单任务超时 + 整体收口。
// 返回每个 URL 的抓取结果（含 Cancelled 标记，对应 Java 的 isCancelled() 分支）。
// 每个任务写自己固定下标的槽位 —— 不同下标的并发写是安全的，无需加锁。
func FetchAll(ctx context.Context, urls []string, perFetch time.Duration) []WebFetchShape {
	f := newFetcher()
	defer f.server.Close()

	results := make([]WebFetchShape, len(urls))
	g := &Group{}
	for i, u := range urls {
		i, u := i, u
		g.Go(func() error {
			taskCtx, cancel := context.WithTimeout(ctx, perFetch)
			defer cancel()

			req, err := http.NewRequestWithContext(taskCtx, http.MethodGet, f.server.URL+u, nil)
			if err != nil {
				results[i] = WebFetchShape{URL: u, Err: err}
				return nil // 单个失败不算整体失败（对应 Java 逐个 catch 汇总）
			}
			resp, err := f.client.Do(req)
			if err != nil {
				results[i] = WebFetchShape{URL: u, Cancelled: taskCtx.Err() != nil, Err: err}
				return nil
			}
			defer func() { _ = resp.Body.Close() }()
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
			results[i] = WebFetchShape{URL: u, Status: resp.StatusCode, Bytes: len(body)}
			return nil
		})
	}
	_ = g.Wait() // 整体收口（对应 invokeAll 返回）
	return results
}
