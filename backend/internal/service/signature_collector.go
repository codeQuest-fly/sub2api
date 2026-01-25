package service

import (
	"log"
	"sync"
)

// SignatureCollector 签名采集器，用于从流式响应中采集签名。
// 线程安全，支持长度过滤和批量获取。
type SignatureCollector struct {
	mu         sync.Mutex
	signatures []string // 采集到的签名值
	minLength  int      // 最小长度过滤
	accountID  int64    // 采集来源账户ID
	model      *string  // 当前请求的模型
}

// NewSignatureCollector 创建签名采集器
func NewSignatureCollector(accountID int64, model *string, minLength int) *SignatureCollector {
	if minLength <= 0 {
		minLength = 350 // 默认最小长度
	}
	return &SignatureCollector{
		signatures: make([]string, 0),
		minLength:  minLength,
		accountID:  accountID,
		model:      model,
	}
}

// Collect 采集签名（线程安全）
// 只有长度大于 minLength 的签名才会被采集
func (c *SignatureCollector) Collect(signature string) {
	// 长度过滤
	if len(signature) <= c.minLength {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.signatures = append(c.signatures, signature)
	log.Printf("[SignatureCollector] Account %d: collected signature (length=%d)", c.accountID, len(signature))
}

// GetCollected 获取采集到的签名列表
func (c *SignatureCollector) GetCollected() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := make([]string, len(c.signatures))
	copy(result, c.signatures)
	return result
}

// Count 获取采集数量
func (c *SignatureCollector) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.signatures)
}

// GetAccountID 获取采集来源账户ID
func (c *SignatureCollector) GetAccountID() int64 {
	return c.accountID
}

// GetModel 获取关联的模型
func (c *SignatureCollector) GetModel() *string {
	return c.model
}
