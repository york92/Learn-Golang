package notify

import (
	"fmt"
	"log"
	"time"
)

// Notifier 通知接口，可扩展为邮件/钉钉/企业微信
type Notifier interface {
	SendWarning(productID int64, productName string, currentStock, threshold int64)
}

// LogNotifier 默认实现：输出到日志
type LogNotifier struct{}

func NewLogNotifier() *LogNotifier {
	return &LogNotifier{}
}

func (n *LogNotifier) SendWarning(productID int64, productName string, currentStock, threshold int64) {
	log.Printf("[STOCK WARNING] %s | 商品ID:%d | 当前库存:%d | 预警阈值:%d | 时间:%s",
		productName, productID, currentStock, threshold, time.Now().Format("2006-01-02 15:04:05"))
}

// WebhookNotifier 示例：可接入钉钉/企业微信 webhook
type WebhookNotifier struct {
	URL string
}

func (n *WebhookNotifier) SendWarning(productID int64, productName string, currentStock, threshold int64) {
	// 实际项目中替换为 http.Post 调用
	fmt.Printf("[WEBHOOK] would POST to %s: product=%s stock=%d threshold=%d\n",
		n.URL, productName, currentStock, threshold)
}
