// Package service 提供 SignatureStreamProcessor 流式响应签名处理器。
package service

import (
	"context"
	"encoding/json"
	"log"
	"regexp"
	"sync"

	"github.com/tidwall/sjson"
)

// sseDataRegex 匹配 SSE data 行的正则
var sseDataRegex = regexp.MustCompile(`^data:\s*`)

// SignatureStreamState 流式响应中 signature 处理的状态追踪
type SignatureStreamState struct {
	mu sync.Mutex

	// thinking 块追踪
	thinkingBlocks map[int]*ThinkingBlockState // index -> state

	// 配置
	config *SignatureConfig

	// 签名池引用
	signaturePool SignaturePoolService

	// 签名采集器（可选，启用采集时非 nil）
	collector *SignatureCollector

	// 上下文
	ctx context.Context

	// 账户ID（用于日志）
	accountID int64
}

// ThinkingBlockState 单个 thinking 块的状态
type ThinkingBlockState struct {
	Index             int
	Started           bool   // 是否已收到 content_block_start
	HasSignatureDelta bool   // 是否已收到 signature_delta
	ReceivedSignature string // 收到的签名值
	Stopped           bool   // 是否已收到 content_block_stop
}

// NewSignatureStreamState 创建新的流式状态追踪器
func NewSignatureStreamState(ctx context.Context, config *SignatureConfig, pool SignaturePoolService, accountID int64, collector *SignatureCollector) *SignatureStreamState {
	return &SignatureStreamState{
		thinkingBlocks: make(map[int]*ThinkingBlockState),
		config:         config,
		signaturePool:  pool,
		ctx:            ctx,
		accountID:      accountID,
		collector:      collector,
	}
}

// ProcessSSELine 处理单行 SSE 数据
// 返回: (处理后的行, 需要注入的额外行, 错误)
func (s *SignatureStreamState) ProcessSSELine(line string) (string, []string, error) {
	// 非 data 行直接返回
	if !sseDataRegex.MatchString(line) {
		return line, nil, nil
	}

	data := sseDataRegex.ReplaceAllString(line, "")
	if data == "" || data == "[DONE]" {
		return line, nil, nil
	}

	// 解析事件类型
	var event struct {
		Type         string          `json:"type"`
		Index        int             `json:"index"`
		Delta        json.RawMessage `json:"delta,omitempty"`
		ContentBlock json.RawMessage `json:"content_block,omitempty"`
	}

	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return line, nil, nil // 解析失败则透传
	}

	var extraLines []string
	var modifiedLine string

	switch event.Type {
	case "content_block_start":
		modifiedLine = s.handleContentBlockStart(line, data, event.Index, event.ContentBlock)

	case "content_block_delta":
		modifiedLine = s.handleContentBlockDelta(line, data, event.Index, event.Delta)

	case "content_block_stop":
		modifiedLine, extraLines = s.handleContentBlockStop(line, event.Index)

	default:
		modifiedLine = line
	}

	return modifiedLine, extraLines, nil
}

// handleContentBlockStart 处理 content_block_start 事件
func (s *SignatureStreamState) handleContentBlockStart(line, data string, index int, contentBlockRaw json.RawMessage) string {
	// 解析 content_block 以检查是否为 thinking 类型
	var contentBlock struct {
		Type      string `json:"type"`
		Thinking  string `json:"thinking"`
		Signature string `json:"signature"`
	}

	if err := json.Unmarshal(contentBlockRaw, &contentBlock); err != nil {
		return line
	}

	// 仅处理 thinking 类型的块
	if contentBlock.Type != "thinking" {
		return line
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 记录 thinking 块状态
	s.thinkingBlocks[index] = &ThinkingBlockState{
		Index:   index,
		Started: true,
	}

	log.Printf("[SignatureStream] Account %d: thinking block %d started", s.accountID, index)

	// content_block_start 中的 signature 通常为空字符串，不需要处理
	return line
}

// handleContentBlockDelta 处理 content_block_delta 事件
func (s *SignatureStreamState) handleContentBlockDelta(line, data string, index int, deltaRaw json.RawMessage) string {
	// 解析 delta 以检查是否为 signature_delta
	var delta struct {
		Type      string `json:"type"`
		Signature string `json:"signature"`
	}

	if err := json.Unmarshal(deltaRaw, &delta); err != nil {
		return line
	}

	if delta.Type != "signature_delta" {
		return line
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 更新 thinking 块状态
	if block, exists := s.thinkingBlocks[index]; exists {
		block.HasSignatureDelta = true
		block.ReceivedSignature = delta.Signature

		// 如果启用采集，采集签名
		if s.collector != nil && delta.Signature != "" {
			s.collector.Collect(delta.Signature)
		}
	}

	// 根据策略决定是否替换
	switch s.config.Strategy {
	case "always_replace":
		return s.replaceSignatureInLine(line, index)
	case "fill_missing":
		// 已有签名，不替换
		log.Printf("[SignatureStream] Account %d: signature_delta received for block %d, keeping original (fill_missing strategy)", s.accountID, index)
		return line
	default:
		return line
	}
}

// handleContentBlockStop 处理 content_block_stop 事件
// 返回: (处理后的行, 需要在此行之前注入的额外行)
func (s *SignatureStreamState) handleContentBlockStop(line string, index int) (string, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	block, exists := s.thinkingBlocks[index]
	if !exists {
		return line, nil
	}

	block.Stopped = true

	// 检查是否需要注入 signature_delta
	needsInjection := false
	switch s.config.Strategy {
	case "always_replace":
		// 如果已经收到并替换过 signature_delta，则不再注入
		// 如果没有收到 signature_delta，需要注入
		needsInjection = !block.HasSignatureDelta
	case "fill_missing":
		needsInjection = !block.HasSignatureDelta // 仅在缺失时注入
	}

	if needsInjection {
		injectedLine := s.generateSignatureDeltaLine(index)
		if injectedLine != "" {
			log.Printf("[SignatureStream] Account %d: injecting signature_delta for block %d before content_block_stop", s.accountID, index)
			return line, []string{injectedLine}
		}
		log.Printf("[SignatureStream] Account %d: failed to generate signature_delta for block %d (pool empty?)", s.accountID, index)
	}

	return line, nil
}

// replaceSignatureInLine 替换行中的签名
func (s *SignatureStreamState) replaceSignatureInLine(line string, index int) string {
	// 从池中获取签名
	signature, err := s.signaturePool.GetRandomSignature(s.ctx, s.config.PoolFilter)
	if err != nil || signature == "" {
		log.Printf("[SignatureStream] Account %d: failed to get signature from pool: %v", s.accountID, err)
		return line // 获取失败则透传原始行
	}

	// 提取 data 部分
	data := sseDataRegex.ReplaceAllString(line, "")

	// 使用 sjson 替换签名值
	newData, err := sjson.Set(data, "delta.signature", signature)
	if err != nil {
		log.Printf("[SignatureStream] Account %d: failed to set signature in JSON: %v", s.accountID, err)
		return line
	}

	log.Printf("[SignatureStream] Account %d: replaced signature for block %d", s.accountID, index)
	return "data: " + newData
}

// generateSignatureDeltaLine 生成 signature_delta 事件行
func (s *SignatureStreamState) generateSignatureDeltaLine(index int) string {
	// 从池中获取签名
	signature, err := s.signaturePool.GetRandomSignature(s.ctx, s.config.PoolFilter)
	if err != nil || signature == "" {
		return ""
	}

	event := map[string]any{
		"type":  "content_block_delta",
		"index": index,
		"delta": map[string]any{
			"type":      "signature_delta",
			"signature": signature,
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		return ""
	}

	return "data: " + string(data)
}

// GetThinkingBlockCount 获取追踪的 thinking 块数量
func (s *SignatureStreamState) GetThinkingBlockCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.thinkingBlocks)
}

// GetStats 获取处理统计
func (s *SignatureStreamState) GetStats() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats := map[string]int{
		"thinking_blocks":   len(s.thinkingBlocks),
		"with_signature":    0,
		"without_signature": 0,
	}

	for _, block := range s.thinkingBlocks {
		if block.HasSignatureDelta {
			stats["with_signature"]++
		} else {
			stats["without_signature"]++
		}
	}

	return stats
}
