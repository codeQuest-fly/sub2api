// Package service 提供 SignaturePool 签名池服务。
package service

import (
	"context"
	"log"
	"math/rand"
	"sync"
	"time"
)

// SignaturePoolService 签名池服务接口
type SignaturePoolService interface {
	// GetRandomSignature 获取随机可用签名
	GetRandomSignature(ctx context.Context, filter *SignaturePoolFilter) (string, error)
	// MarkUsed 标记签名已使用（异步更新计数）
	MarkUsed(ctx context.Context, signatureID int64)
	// InvalidateCache 使缓存失效
	InvalidateCache()
	// GetPoolSize 获取当前池大小
	GetPoolSize() int
}

// CachedSignature 缓存的签名
type CachedSignature struct {
	ID    int64
	Value string
	Model *string
}

// signaturePoolService 签名池服务实现
type signaturePoolService struct {
	repo SignatureRepository

	// 内存缓存
	cacheMu     sync.RWMutex
	cachedSigs  []CachedSignature
	cacheExpiry time.Time
	cacheTTL    time.Duration

	// 随机数生成器
	rng *rand.Rand
}

// NewSignaturePoolService 创建签名池服务
func NewSignaturePoolService(repo SignatureRepository) SignaturePoolService {
	return &signaturePoolService{
		repo:     repo,
		cacheTTL: 5 * time.Minute,
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GetRandomSignature 获取随机可用签名
func (s *signaturePoolService) GetRandomSignature(ctx context.Context, filter *SignaturePoolFilter) (string, error) {
	sigs := s.getCachedSignatures(ctx)
	if len(sigs) == 0 {
		return "", ErrSignaturePoolEmpty
	}

	// 应用过滤条件
	filtered := s.filterSignatures(sigs, filter)
	if len(filtered) == 0 {
		return "", ErrSignaturePoolEmpty
	}

	// 随机选择
	s.cacheMu.Lock()
	idx := s.rng.Intn(len(filtered))
	s.cacheMu.Unlock()

	selected := filtered[idx]

	// 异步更新使用计数
	go s.MarkUsed(context.Background(), selected.ID)

	return selected.Value, nil
}

// getCachedSignatures 获取缓存的签名，如过期则重新加载
func (s *signaturePoolService) getCachedSignatures(ctx context.Context) []CachedSignature {
	s.cacheMu.RLock()
	if len(s.cachedSigs) > 0 && time.Now().Before(s.cacheExpiry) {
		sigs := s.cachedSigs
		s.cacheMu.RUnlock()
		return sigs
	}
	s.cacheMu.RUnlock()

	// 缓存为空或过期，重新加载
	return s.reloadCache(ctx)
}

// reloadCache 从数据库重新加载缓存
func (s *signaturePoolService) reloadCache(ctx context.Context) []CachedSignature {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	// 双重检查
	if len(s.cachedSigs) > 0 && time.Now().Before(s.cacheExpiry) {
		return s.cachedSigs
	}

	// 从数据库加载活跃签名
	signatures, err := s.repo.ListActive(ctx, 1000) // 最多加载 1000 条
	if err != nil {
		log.Printf("[SignaturePool] Failed to load signatures from DB: %v", err)
		return s.cachedSigs // 返回旧缓存
	}

	// 转换为缓存格式
	s.cachedSigs = make([]CachedSignature, len(signatures))
	for i, sig := range signatures {
		s.cachedSigs[i] = CachedSignature{
			ID:    sig.ID,
			Value: sig.Value,
			Model: sig.Model,
		}
	}
	s.cacheExpiry = time.Now().Add(s.cacheTTL)

	log.Printf("[SignaturePool] Loaded %d signatures into cache", len(s.cachedSigs))
	return s.cachedSigs
}

// filterSignatures 应用过滤条件
func (s *signaturePoolService) filterSignatures(sigs []CachedSignature, filter *SignaturePoolFilter) []CachedSignature {
	if filter == nil || len(filter.Models) == 0 {
		return sigs
	}

	// 构建模型集合用于快速查找
	modelSet := make(map[string]struct{}, len(filter.Models))
	for _, m := range filter.Models {
		modelSet[m] = struct{}{}
	}

	var result []CachedSignature
	for _, sig := range sigs {
		// 如果签名没有指定模型，或者模型匹配过滤条件
		if sig.Model == nil {
			result = append(result, sig)
			continue
		}
		if _, ok := modelSet[*sig.Model]; ok {
			result = append(result, sig)
		}
	}

	// 如果过滤后为空，返回原始列表（降级策略）
	if len(result) == 0 {
		return sigs
	}

	return result
}

// MarkUsed 异步标记签名已使用
func (s *signaturePoolService) MarkUsed(ctx context.Context, signatureID int64) {
	if err := s.repo.IncrementUseCount(ctx, signatureID); err != nil {
		log.Printf("[SignaturePool] Failed to increment use count for signature %d: %v", signatureID, err)
	}
}

// InvalidateCache 使缓存失效
func (s *signaturePoolService) InvalidateCache() {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cacheExpiry = time.Time{} // 设置为零值，下次获取时会重新加载
	log.Printf("[SignaturePool] Cache invalidated")
}

// GetPoolSize 获取当前池大小
func (s *signaturePoolService) GetPoolSize() int {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	return len(s.cachedSigs)
}
