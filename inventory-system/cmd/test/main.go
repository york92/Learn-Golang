package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"inventory/db"
	"inventory/handler"
	"inventory/pkg/notify"
	"inventory/repository"
	"inventory/service"
)

func buildServer(dbPath string) http.Handler {
	database, err := db.Init(dbPath)
	if err != nil {
		panic(err)
	}
	productRepo := repository.NewProductRepo(database)
	logRepo := repository.NewLogRepo(database)
	warningRepo := repository.NewWarningRepo(database)
	notifier := notify.NewLogNotifier()
	productService := service.NewProductService(productRepo)
	stockService := service.NewStockService(database, productRepo, logRepo, warningRepo, notifier)
	warningService := service.NewWarningService(warningRepo)
	productHandler := handler.NewProductHandler(productService)
	stockHandler := handler.NewStockHandler(stockService)
	warningHandler := handler.NewWarningHandler(warningService)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/products", productHandler.Create)
	mux.HandleFunc("GET /api/products", productHandler.List)
	mux.HandleFunc("GET /api/products/{id}", productHandler.GetByID)
	mux.HandleFunc("PUT /api/products/{id}", productHandler.Update)
	mux.HandleFunc("DELETE /api/products/{id}", productHandler.Delete)
	mux.HandleFunc("POST /api/stock/in", stockHandler.StockIn)
	mux.HandleFunc("POST /api/stock/out", stockHandler.StockOut)
	mux.HandleFunc("POST /api/stock/loss", stockHandler.StockLoss)
	mux.HandleFunc("GET /api/logs", stockHandler.ListLogs)
	mux.HandleFunc("GET /api/warnings", warningHandler.List)
	mux.HandleFunc("PUT /api/warnings/{id}/resolve", warningHandler.Resolve)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	return mux
}

type testResult struct {
	name    string
	passed  bool
	detail  string
}

func doRequest(srv *httptest.Server, method, path string, body interface{}) (int, map[string]interface{}) {
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, srv.URL+path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return resp.StatusCode, result
}

func check(results *[]testResult, name string, cond bool, detail string) {
	r := testResult{name: name, passed: cond, detail: detail}
	*results = append(*results, r)
	status := "✅ PASS"
	if !cond {
		status = "❌ FAIL"
	}
	fmt.Printf("  %s  %s", status, name)
	if detail != "" {
		fmt.Printf(" — %s", detail)
	}
	fmt.Println()
}

func main() {
	dbPath := "/tmp/test_inventory.db"
	os.Remove(dbPath)
	defer os.Remove(dbPath)

	srv := httptest.NewServer(buildServer(dbPath))
	defer srv.Close()

	var results []testResult
	fmt.Println("\n╔══════════════════════════════════════════════════════╗")
	fmt.Println("║         库存管理系统 — 集成自检测试                      ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝\n")

	// ── 健康检查 ────────────────────────────────────────
	fmt.Println("【健康检查】")
	code, body := doRequest(srv, "GET", "/health", nil)
	check(&results, "GET /health 返回 200", code == 200, fmt.Sprintf("status=%d", code))

	// ── 商品管理 ────────────────────────────────────────
	fmt.Println("\n【商品管理 - CRUD】")
	code, body = doRequest(srv, "POST", "/api/products", map[string]interface{}{
		"name": "农夫山泉矿泉水", "spec": "550ml/瓶", "price": 2.5,
		"initial_stock": 20, "warning_threshold": 5,
	})
	check(&results, "POST /api/products 创建商品", code == 201, fmt.Sprintf("http=%d", code))
	var product1ID float64
	if data, ok := body["data"].(map[string]interface{}); ok {
		product1ID = data["id"].(float64)
		check(&results, "创建商品返回正确字段", data["name"] == "农夫山泉矿泉水" && data["stock"].(float64) == 20,
			fmt.Sprintf("name=%s stock=%.0f", data["name"], data["stock"]))
	}

	code, body = doRequest(srv, "POST", "/api/products", map[string]interface{}{
		"name": "瑞幸咖啡", "spec": "美式/杯", "price": 9.9,
		"initial_stock": 10, "warning_threshold": 3,
	})
	var product2ID float64
	if data, ok := body["data"].(map[string]interface{}); ok {
		product2ID = data["id"].(float64)
	}
	check(&results, "POST /api/products 创建第二个商品", code == 201, "")

	// 参数校验
	code, _ = doRequest(srv, "POST", "/api/products", map[string]interface{}{"name": "", "initial_stock": 10})
	check(&results, "创建商品名称为空 → 400", code == 400, fmt.Sprintf("http=%d", code))

	code, _ = doRequest(srv, "POST", "/api/products", map[string]interface{}{"name": "测试", "initial_stock": -1})
	check(&results, "创建商品负数库存 → 400", code == 400, fmt.Sprintf("http=%d", code))

	// 列表查询
	code, body = doRequest(srv, "GET", "/api/products?page=1&page_size=10", nil)
	check(&results, "GET /api/products 列表查询", code == 200, "")
	if data, ok := body["data"].(map[string]interface{}); ok {
		total := data["total"].(float64)
		check(&results, "列表返回 2 条记录", total == 2, fmt.Sprintf("total=%.0f", total))
	}

	// 关键词搜索
	code, body = doRequest(srv, "GET", "/api/products?keyword=咖啡", nil)
	if data, ok := body["data"].(map[string]interface{}); ok {
		total := data["total"].(float64)
		check(&results, "keyword=cofee returns 1", total == 1, fmt.Sprintf("total=%.0f", total))
	}

	// 商品详情
	code, body = doRequest(srv, "GET", fmt.Sprintf("/api/products/%.0f", product1ID), nil)
	check(&results, "GET /api/products/{id} 商品详情", code == 200, "")

	// 不存在的商品
	code, _ = doRequest(srv, "GET", "/api/products/9999", nil)
	check(&results, "GET /api/products/9999 → 404", code == 404, fmt.Sprintf("http=%d", code))

	// 更新商品
	newPrice := 3.0
	code, body = doRequest(srv, "PUT", fmt.Sprintf("/api/products/%.0f", product1ID), map[string]interface{}{
		"price": newPrice, "warning_threshold": 5,
	})
	check(&results, "PUT /api/products/{id} 更新价格", code == 200, "")
	if data, ok := body["data"].(map[string]interface{}); ok {
		check(&results, "更新后价格正确", data["price"].(float64) == 3.0, fmt.Sprintf("price=%.1f", data["price"]))
	}

	// ── 库存操作 ────────────────────────────────────────
	fmt.Println("\n【库存操作 - 入库/出库/损耗】")

	// 入库
	code, body = doRequest(srv, "POST", "/api/stock/in", map[string]interface{}{
		"product_id": product1ID, "quantity": 50,
		"remark": "采购单PO-001", "operator": "张三",
	})
	check(&results, "POST /api/stock/in 入库", code == 200, "")
	if data, ok := body["data"].(map[string]interface{}); ok {
		check(&results, "入库流水 before=20 after=70", data["before_stock"].(float64) == 20 && data["after_stock"].(float64) == 70,
			fmt.Sprintf("before=%.0f after=%.0f", data["before_stock"], data["after_stock"]))
	}

	// 出库
	code, body = doRequest(srv, "POST", "/api/stock/out", map[string]interface{}{
		"product_id": product1ID, "quantity": 10,
		"remark": "销售订单SO-001", "operator": "李四",
	})
	check(&results, "POST /api/stock/out 出库", code == 200, "")
	if data, ok := body["data"].(map[string]interface{}); ok {
		check(&results, "出库流水 before=70 after=60", data["before_stock"].(float64) == 70 && data["after_stock"].(float64) == 60,
			fmt.Sprintf("before=%.0f after=%.0f", data["before_stock"], data["after_stock"]))
	}

	// 损耗
	code, body = doRequest(srv, "POST", "/api/stock/loss", map[string]interface{}{
		"product_id": product2ID, "quantity": 2,
		"remark": "破损处理", "operator": "王五",
	})
	check(&results, "POST /api/stock/loss 损耗", code == 200, "")
	if data, ok := body["data"].(map[string]interface{}); ok {
		check(&results, "损耗流水 before=10 after=8", data["before_stock"].(float64) == 10 && data["after_stock"].(float64) == 8,
			fmt.Sprintf("before=%.0f after=%.0f", data["before_stock"], data["after_stock"]))
	}

	// 库存不足拒绝
	code, body = doRequest(srv, "POST", "/api/stock/out", map[string]interface{}{
		"product_id": product2ID, "quantity": 9999, "operator": "测试",
	})
	check(&results, "出库超量 → 400 库存不足", code == 400, fmt.Sprintf("http=%d msg=%v", code, body["message"]))

	// 无效数量
	code, _ = doRequest(srv, "POST", "/api/stock/out", map[string]interface{}{
		"product_id": product1ID, "quantity": -5, "operator": "测试",
	})
	check(&results, "出库负数数量 → 400", code == 400, fmt.Sprintf("http=%d", code))

	// 不存在的商品操作
	code, _ = doRequest(srv, "POST", "/api/stock/out", map[string]interface{}{
		"product_id": 9999, "quantity": 1, "operator": "测试",
	})
	check(&results, "出库不存在商品 → 404", code == 404, fmt.Sprintf("http=%d", code))

	// ── 预警机制 ────────────────────────────────────────
	fmt.Println("\n【预警机制】")

	// 让咖啡库存降到阈值3以下（当前8，出库6后=2）
	code, _ = doRequest(srv, "POST", "/api/stock/out", map[string]interface{}{
		"product_id": product2ID, "quantity": 6,
		"remark": "大量出库触发预警", "operator": "赵六",
	})
	check(&results, "出库后库存低于阈值(8-6=2 < 3)", code == 200, "")

	time.Sleep(200 * time.Millisecond) // 等待异步预警处理

	code, body = doRequest(srv, "GET", "/api/warnings?only_unresolved=true", nil)
	check(&results, "GET /api/warnings 预警列表有记录", code == 200, "")
	var warningID float64
	if data, ok := body["data"].(map[string]interface{}); ok {
		items := data["items"].([]interface{})
		check(&results, "触发预警记录数量 >= 1", len(items) >= 1, fmt.Sprintf("count=%d", len(items)))
		if len(items) > 0 {
			w := items[0].(map[string]interface{})
			warningID = w["id"].(float64)
			check(&results, "预警记录商品ID正确", w["product_id"].(float64) == product2ID,
				fmt.Sprintf("product_id=%.0f threshold=%.0f current=%.0f",
					w["product_id"], w["threshold"], w["current_stock"]))
		}
	}

	// 防重复预警：再次出库1，不应重复创建预警
	doRequest(srv, "POST", "/api/stock/out", map[string]interface{}{
		"product_id": product2ID, "quantity": 1, "operator": "赵六",
	})
	time.Sleep(200 * time.Millisecond)
	code, body = doRequest(srv, "GET", "/api/warnings?only_unresolved=true", nil)
	if data, ok := body["data"].(map[string]interface{}); ok {
		items := data["items"].([]interface{})
		check(&results, "防重复预警：再次低库存不重复创建", len(items) == 1, fmt.Sprintf("warning_count=%d", len(items)))
	}

	// 标记预警处理
	if warningID > 0 {
		code, body = doRequest(srv, "PUT", fmt.Sprintf("/api/warnings/%.0f/resolve", warningID), map[string]interface{}{})
		check(&results, "PUT /api/warnings/{id}/resolve 标记处理", code == 200, fmt.Sprintf("http=%d", code))
	}

	// 验证已处理
	code, body = doRequest(srv, "GET", "/api/warnings?only_unresolved=true", nil)
	if data, ok := body["data"].(map[string]interface{}); ok {
		total := data["total"].(float64)
		check(&results, "处理后 unresolved 预警为 0", total == 0, fmt.Sprintf("remaining=%.0f", total))
	}

	// ── 流水查询 ────────────────────────────────────────
	fmt.Println("\n【流水查询】")

	code, body = doRequest(srv, "GET", "/api/logs?page=1&page_size=20", nil)
	check(&results, "GET /api/logs 全量流水查询", code == 200, "")
	if data, ok := body["data"].(map[string]interface{}); ok {
		total := data["total"].(float64)
		check(&results, "流水总数 >= 5", total >= 5, fmt.Sprintf("total=%.0f", total))
	}

	// 按商品过滤
	code, body = doRequest(srv, "GET", fmt.Sprintf("/api/logs?product_id=%.0f", product2ID), nil)
	if data, ok := body["data"].(map[string]interface{}); ok {
		total := data["total"].(float64)
		check(&results, "按商品过滤流水", total >= 3, fmt.Sprintf("product2_logs=%.0f", total))
	}

	// 按类型过滤
	code, body = doRequest(srv, "GET", "/api/logs?change_type=IN", nil)
	if data, ok := body["data"].(map[string]interface{}); ok {
		items := data["items"].([]interface{})
		allIn := true
		for _, item := range items {
			if item.(map[string]interface{})["change_type"] != "IN" {
				allIn = false
			}
		}
		check(&results, "按类型过滤 change_type=IN", allIn, fmt.Sprintf("count=%d", len(items)))
	}

	// ── 软删除 ────────────────────────────────────────
	fmt.Println("\n【软删除】")
	code, body = doRequest(srv, "POST", "/api/products", map[string]interface{}{
		"name": "待删除商品", "initial_stock": 5,
	})
	var delID float64
	if data, ok := body["data"].(map[string]interface{}); ok {
		delID = data["id"].(float64)
	}
	code, _ = doRequest(srv, "DELETE", fmt.Sprintf("/api/products/%.0f", delID), nil)
	check(&results, "DELETE /api/products/{id} 软删除", code == 200, "")
	code, _ = doRequest(srv, "GET", fmt.Sprintf("/api/products/%.0f", delID), nil)
	check(&results, "软删除后查询 → 404", code == 404, fmt.Sprintf("http=%d", code))
	// 软删除不影响列表（只计未删除）
	code, body = doRequest(srv, "GET", "/api/products", nil)
	if data, ok := body["data"].(map[string]interface{}); ok {
		total := data["total"].(float64)
		check(&results, "软删除后列表仍为2条", total == 2, fmt.Sprintf("total=%.0f", total))
	}

	// ── 汇总 ────────────────────────────────────────
	passed, failed := 0, 0
	for _, r := range results {
		if r.passed {
			passed++
		} else {
			failed++
		}
	}
	fmt.Printf("\n╔══════════════════════════════════════════════════════╗\n")
	fmt.Printf("║  测试结果：%d 通过 / %d 失败 / %d 总计                    \n", passed, failed, len(results))
	fmt.Printf("╚══════════════════════════════════════════════════════╝\n")

	if failed > 0 {
		fmt.Println("\n❌ 存在失败用例，详情见上方输出")
		os.Exit(1)
	} else {
		fmt.Println("\n✅ 所有测试通过！")
	}
}
