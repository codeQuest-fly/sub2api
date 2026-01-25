// Package service 提供 Signature 相关的业务逻辑。
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// Signature 错误定义
var (
	ErrSignatureNotFound  = infraerrors.NotFound("SIGNATURE_NOT_FOUND", "signature not found")
	ErrSignatureNilInput  = infraerrors.BadRequest("SIGNATURE_NIL_INPUT", "signature input is nil")
	ErrSignatureDuplicate = infraerrors.Conflict("SIGNATURE_DUPLICATE", "signature already exists")
	ErrSignaturePoolEmpty = errors.New("signature pool is empty")
)

// Signature 签名实体
type Signature struct {
	ID                     int64
	Value                  string     // Base64 编码的签名值
	Hash                   string     // SHA256 哈希（用于去重）
	Model                  *string    // 关联模型名称
	Source                 string     // 来源：collected, imported, manual
	Status                 string     // 状态：active, disabled, expired
	UseCount               int64      // 使用次数
	LastUsedAt             *time.Time // 最后使用时间
	LastVerifiedAt         *time.Time // 最后验证时间
	Notes                  *string    // 备注
	CollectedFromAccountID *int64     // 采集来源账号ID
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// SignatureFilter 签名筛选条件
type SignatureFilter struct {
	Status                 string  // 状态筛选
	Source                 string  // 来源筛选
	Model                  *string // 模型筛选
	Search                 string  // 搜索关键词
	AccountNamePrefix      string  // 账号名称前缀搜索
	CollectedFromAccountID *int64  // 按采集来源账号筛选
}

// SignatureStats 签名池统计信息
type SignatureStats struct {
	Total        int64 `json:"total"`
	Active       int64 `json:"active"`
	Disabled     int64 `json:"disabled"`
	Expired      int64 `json:"expired"`
	TotalUsage   int64 `json:"total_usage"`
	RecentlyUsed int64 `json:"recently_used"` // 24h 内使用过
}

// BatchImportResult 批量导入结果
type BatchImportResult struct {
	Total      int `json:"total"`
	Imported   int `json:"imported"`
	Duplicated int `json:"duplicated"`
	Failed     int `json:"failed"`
}

// SignatureConfig 账户的 signature 处理配置
type SignatureConfig struct {
	Enabled          bool                 `json:"enabled"`
	Strategy         string               `json:"strategy"` // always_replace, fill_missing, disabled
	PoolFilter       *SignaturePoolFilter `json:"pool_filter,omitempty"`
	EnableCollection bool                 `json:"enable_collection"` // 是否启用签名采集
	MinLength        int                  `json:"min_length"`        // 最小签名长度过滤（默认 350）
}

// SignaturePoolFilter 签名池过滤条件
type SignaturePoolFilter struct {
	Models []string `json:"models,omitempty"`
}

// SignatureRepository 签名仓储接口
type SignatureRepository interface {
	Create(ctx context.Context, sig *Signature) error
	BatchCreate(ctx context.Context, sigs []*Signature) (int, error)
	GetByID(ctx context.Context, id int64) (*Signature, error)
	GetByHash(ctx context.Context, hash string) (*Signature, error)
	ExistsByHash(ctx context.Context, hash string) (bool, error)
	ExistsByHashes(ctx context.Context, hashes []string) (map[string]bool, error)
	Update(ctx context.Context, sig *Signature) error
	Delete(ctx context.Context, id int64) error
	BatchDelete(ctx context.Context, ids []int64) (int, error)
	DeleteByAccountID(ctx context.Context, accountID int64) (int, error)
	List(ctx context.Context, filter *SignatureFilter, page *pagination.PaginationParams) ([]Signature, int, error)
	ListActive(ctx context.Context, limit int) ([]Signature, error)
	IncrementUseCount(ctx context.Context, id int64) error
	GetStats(ctx context.Context) (*SignatureStats, error)
}

// SignatureService 签名服务接口
type SignatureService interface {
	Create(ctx context.Context, value string, model *string, notes *string) (*Signature, error)
	BatchImport(ctx context.Context, values []string, model *string, source string) (*BatchImportResult, error)
	BatchImportWithAccountID(ctx context.Context, values []string, model *string, source string, accountID int64) (*BatchImportResult, error)
	GetByID(ctx context.Context, id int64) (*Signature, error)
	Update(ctx context.Context, id int64, status string, model *string, notes *string) error
	Delete(ctx context.Context, id int64) error
	BatchDelete(ctx context.Context, ids []int64) (int, error)
	DeleteByAccountID(ctx context.Context, accountID int64) (int, error)
	List(ctx context.Context, filter *SignatureFilter, page *pagination.PaginationParams) ([]Signature, int, error)
	GetStats(ctx context.Context) (*SignatureStats, error)
}

// signatureService 签名服务实现
type signatureService struct {
	repo SignatureRepository
}

// NewSignatureService 创建签名服务实例
func NewSignatureService(repo SignatureRepository) SignatureService {
	return &signatureService{repo: repo}
}

// Create 创建单条签名
func (s *signatureService) Create(ctx context.Context, value string, model *string, notes *string) (*Signature, error) {
	hash := computeSignatureHash(value)

	// 检查是否已存在
	exists, err := s.repo.ExistsByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrSignatureDuplicate
	}

	sig := &Signature{
		Value:    value,
		Hash:     hash,
		Model:    model,
		Source:   "manual",
		Status:   "active",
		UseCount: 0,
		Notes:    notes,
	}

	if err := s.repo.Create(ctx, sig); err != nil {
		return nil, err
	}

	return sig, nil
}

// BatchImport 批量导入签名
func (s *signatureService) BatchImport(ctx context.Context, values []string, model *string, source string) (*BatchImportResult, error) {
	if len(values) == 0 {
		return &BatchImportResult{}, nil
	}

	if source == "" {
		source = "imported"
	}

	result := &BatchImportResult{
		Total: len(values),
	}

	// 计算所有哈希
	hashes := make([]string, len(values))
	valueToHash := make(map[string]string, len(values))
	for i, v := range values {
		hash := computeSignatureHash(v)
		hashes[i] = hash
		valueToHash[v] = hash
	}

	// 批量检查已存在的哈希
	existingHashes, err := s.repo.ExistsByHashes(ctx, hashes)
	if err != nil {
		return nil, err
	}

	// 过滤出新签名
	newSigs := make([]*Signature, 0, len(values))
	for _, v := range values {
		hash := valueToHash[v]
		if existingHashes[hash] {
			result.Duplicated++
			continue
		}

		newSigs = append(newSigs, &Signature{
			Value:    v,
			Hash:     hash,
			Model:    model,
			Source:   source,
			Status:   "active",
			UseCount: 0,
		})
	}

	// 批量插入
	if len(newSigs) > 0 {
		imported, err := s.repo.BatchCreate(ctx, newSigs)
		if err != nil {
			result.Failed = len(newSigs)
			return result, err
		}
		result.Imported = imported
	}

	return result, nil
}

// GetByID 根据 ID 获取签名
func (s *signatureService) GetByID(ctx context.Context, id int64) (*Signature, error) {
	return s.repo.GetByID(ctx, id)
}

// Update 更新签名
func (s *signatureService) Update(ctx context.Context, id int64, status string, model *string, notes *string) error {
	sig, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	sig.Status = status
	sig.Model = model
	sig.Notes = notes

	return s.repo.Update(ctx, sig)
}

// Delete 删除签名
func (s *signatureService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// BatchDelete 批量删除签名
func (s *signatureService) BatchDelete(ctx context.Context, ids []int64) (int, error) {
	return s.repo.BatchDelete(ctx, ids)
}

// DeleteByAccountID 删除指定账号采集的所有签名
func (s *signatureService) DeleteByAccountID(ctx context.Context, accountID int64) (int, error) {
	return s.repo.DeleteByAccountID(ctx, accountID)
}

// BatchImportWithAccountID 批量导入签名，记录采集来源账号
func (s *signatureService) BatchImportWithAccountID(ctx context.Context, values []string, model *string, source string, accountID int64) (*BatchImportResult, error) {
	if len(values) == 0 {
		return &BatchImportResult{}, nil
	}

	if source == "" {
		source = "collected"
	}

	result := &BatchImportResult{
		Total: len(values),
	}

	// 计算所有哈希
	hashes := make([]string, len(values))
	valueToHash := make(map[string]string, len(values))
	for i, v := range values {
		hash := computeSignatureHash(v)
		hashes[i] = hash
		valueToHash[v] = hash
	}

	// 批量检查已存在的哈希
	existingHashes, err := s.repo.ExistsByHashes(ctx, hashes)
	if err != nil {
		return nil, err
	}

	// 过滤出新签名
	newSigs := make([]*Signature, 0, len(values))
	for _, v := range values {
		hash := valueToHash[v]
		if existingHashes[hash] {
			result.Duplicated++
			continue
		}

		newSigs = append(newSigs, &Signature{
			Value:                  v,
			Hash:                   hash,
			Model:                  model,
			Source:                 source,
			Status:                 "active",
			UseCount:               0,
			CollectedFromAccountID: &accountID,
		})
	}

	// 批量插入
	if len(newSigs) > 0 {
		imported, err := s.repo.BatchCreate(ctx, newSigs)
		if err != nil {
			result.Failed = len(newSigs)
			return result, err
		}
		result.Imported = imported
	}

	return result, nil
}

// List 分页查询签名
func (s *signatureService) List(ctx context.Context, filter *SignatureFilter, page *pagination.PaginationParams) ([]Signature, int, error) {
	return s.repo.List(ctx, filter, page)
}

// GetStats 获取统计信息
func (s *signatureService) GetStats(ctx context.Context) (*SignatureStats, error) {
	return s.repo.GetStats(ctx)
}

// computeSignatureHash 计算签名值的 SHA256 哈希
func computeSignatureHash(value string) string {
	h := sha256.Sum256([]byte(value))
	return hex.EncodeToString(h[:])
}
