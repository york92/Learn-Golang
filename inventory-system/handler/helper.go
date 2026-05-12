package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// pathID 从 URL 路径中提取 {key} 参数
// 兼容 Go 1.22 标准库 net/http 的新路由语法 {id}
func pathID(r *http.Request, key string) (int64, error) {
	val := r.PathValue(key)
	if val == "" {
		// fallback: 手动从路径解析最后一段数字
		parts := strings.Split(strings.TrimRight(r.URL.Path, "/"), "/")
		if len(parts) == 0 {
			return 0, fmt.Errorf("missing path param: %s", key)
		}
		val = parts[len(parts)-1]
	}
	id, err := strconv.ParseInt(val, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid id: %s", val)
	}
	return id, nil
}
