// Package repository 实现数据访问层（Repository Pattern）。
package repository

import (
	"context"
	"database/sql"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbsignature "github.com/Wei-Shaw/sub2api/ent/signature"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// signatureRepository 实现 service.SignatureRepository 接口。
type signatureRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

// NewSignatureRepository 创建签名仓储实例。
func NewSignatureRepository(client *dbent.Client, sqlDB *sql.DB) service.SignatureRepository {
	return &signatureRepository{client: client, sql: sqlDB}
}

// Create 创建单条签名记录。
func (r *signatureRepository) Create(ctx context.Context, sig *service.Signature) error {
	if sig == nil {
		return service.ErrSignatureNilInput
	}

	builder := r.client.Signature.Create().
		SetValue(sig.Value).
		SetHash(sig.Hash).
		SetSource(dbsignature.Source(sig.Source)).
		SetStatus(dbsignature.Status(sig.Status)).
		SetUseCount(sig.UseCount)

	if sig.Model != nil {
		builder.SetModel(*sig.Model)
	}
	if sig.Notes != nil {
		builder.SetNotes(*sig.Notes)
	}
	if sig.LastUsedAt != nil {
		builder.SetLastUsedAt(*sig.LastUsedAt)
	}
	if sig.LastVerifiedAt != nil {
		builder.SetLastVerifiedAt(*sig.LastVerifiedAt)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrSignatureNotFound, nil)
	}

	sig.ID = created.ID
	sig.CreatedAt = created.CreatedAt
	sig.UpdatedAt = created.UpdatedAt
	return nil
}

// BatchCreate 批量创建签名记录。
func (r *signatureRepository) BatchCreate(ctx context.Context, sigs []*service.Signature) (int, error) {
	if len(sigs) == 0 {
		return 0, nil
	}

	builders := make([]*dbent.SignatureCreate, 0, len(sigs))
	for _, sig := range sigs {
		builder := r.client.Signature.Create().
			SetValue(sig.Value).
			SetHash(sig.Hash).
			SetSource(dbsignature.Source(sig.Source)).
			SetStatus(dbsignature.Status(sig.Status)).
			SetUseCount(sig.UseCount)

		if sig.Model != nil {
			builder.SetModel(*sig.Model)
		}
		if sig.Notes != nil {
			builder.SetNotes(*sig.Notes)
		}
		if sig.CollectedFromAccountID != nil {
			builder.SetCollectedFromAccountID(*sig.CollectedFromAccountID)
		}

		builders = append(builders, builder)
	}

	created, err := r.client.Signature.CreateBulk(builders...).Save(ctx)
	if err != nil {
		return 0, translatePersistenceError(err, service.ErrSignatureNotFound, nil)
	}

	return len(created), nil
}

// GetByID 根据 ID 获取签名。
func (r *signatureRepository) GetByID(ctx context.Context, id int64) (*service.Signature, error) {
	m, err := r.client.Signature.Query().
		Where(dbsignature.IDEQ(id)).
		Where(dbsignature.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrSignatureNotFound, nil)
	}
	return r.signatureToService(m), nil
}

// GetByHash 根据哈希获取签名（用于去重检查）。
func (r *signatureRepository) GetByHash(ctx context.Context, hash string) (*service.Signature, error) {
	m, err := r.client.Signature.Query().
		Where(dbsignature.HashEQ(hash)).
		Where(dbsignature.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrSignatureNotFound, nil)
	}
	return r.signatureToService(m), nil
}

// ExistsByHash 检查哈希是否存在。
func (r *signatureRepository) ExistsByHash(ctx context.Context, hash string) (bool, error) {
	exists, err := r.client.Signature.Query().
		Where(dbsignature.HashEQ(hash)).
		Where(dbsignature.DeletedAtIsNil()).
		Exist(ctx)
	if err != nil {
		return false, translatePersistenceError(err, service.ErrSignatureNotFound, nil)
	}
	return exists, nil
}

// ExistsByHashes 批量检查哈希是否存在，返回已存在的哈希集合。
func (r *signatureRepository) ExistsByHashes(ctx context.Context, hashes []string) (map[string]bool, error) {
	if len(hashes) == 0 {
		return map[string]bool{}, nil
	}

	existing, err := r.client.Signature.Query().
		Where(dbsignature.HashIn(hashes...)).
		Where(dbsignature.DeletedAtIsNil()).
		Select(dbsignature.FieldHash).
		Strings(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrSignatureNotFound, nil)
	}

	result := make(map[string]bool, len(existing))
	for _, h := range existing {
		result[h] = true
	}
	return result, nil
}

// Update 更新签名。
func (r *signatureRepository) Update(ctx context.Context, sig *service.Signature) error {
	if sig == nil {
		return service.ErrSignatureNilInput
	}

	builder := r.client.Signature.UpdateOneID(sig.ID).
		SetStatus(dbsignature.Status(sig.Status)).
		SetUseCount(sig.UseCount)

	if sig.Model != nil {
		builder.SetModel(*sig.Model)
	} else {
		builder.ClearModel()
	}
	if sig.Notes != nil {
		builder.SetNotes(*sig.Notes)
	} else {
		builder.ClearNotes()
	}
	if sig.LastUsedAt != nil {
		builder.SetLastUsedAt(*sig.LastUsedAt)
	}
	if sig.LastVerifiedAt != nil {
		builder.SetLastVerifiedAt(*sig.LastVerifiedAt)
	}

	_, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrSignatureNotFound, nil)
	}
	return nil
}

// Delete 软删除签名。
func (r *signatureRepository) Delete(ctx context.Context, id int64) error {
	now := time.Now()
	_, err := r.client.Signature.UpdateOneID(id).
		SetDeletedAt(now).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrSignatureNotFound, nil)
	}
	return nil
}

// BatchDelete 批量软删除签名。
func (r *signatureRepository) BatchDelete(ctx context.Context, ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	now := time.Now()
	affected, err := r.client.Signature.Update().
		Where(dbsignature.IDIn(ids...)).
		Where(dbsignature.DeletedAtIsNil()).
		SetDeletedAt(now).
		Save(ctx)
	if err != nil {
		return 0, translatePersistenceError(err, service.ErrSignatureNotFound, nil)
	}
	return affected, nil
}

// DeleteByAccountID 删除指定账号采集的所有签名。
func (r *signatureRepository) DeleteByAccountID(ctx context.Context, accountID int64) (int, error) {
	now := time.Now()
	affected, err := r.client.Signature.Update().
		Where(dbsignature.CollectedFromAccountIDEQ(accountID)).
		Where(dbsignature.DeletedAtIsNil()).
		SetDeletedAt(now).
		Save(ctx)
	if err != nil {
		return 0, translatePersistenceError(err, service.ErrSignatureNotFound, nil)
	}
	return affected, nil
}

// List 分页查询签名列表。
func (r *signatureRepository) List(ctx context.Context, filter *service.SignatureFilter, page *pagination.PaginationParams) ([]service.Signature, int, error) {
	query := r.client.Signature.Query().
		Where(dbsignature.DeletedAtIsNil())

	// 应用筛选条件
	if filter != nil {
		if filter.Status != "" {
			query = query.Where(dbsignature.StatusEQ(dbsignature.Status(filter.Status)))
		}
		if filter.Source != "" {
			query = query.Where(dbsignature.SourceEQ(dbsignature.Source(filter.Source)))
		}
		if filter.Model != nil {
			query = query.Where(dbsignature.ModelEQ(*filter.Model))
		}
		if filter.Search != "" {
			query = query.Where(dbsignature.Or(
				dbsignature.ValueContains(filter.Search),
				dbsignature.NotesContains(filter.Search),
			))
		}
		if filter.AccountNamePrefix != "" {
			// 查询匹配前缀的账号IDs
			matchedAccountIDs := r.findAccountIDsByNamePrefix(ctx, filter.AccountNamePrefix)
			if len(matchedAccountIDs) > 0 {
				query = query.Where(dbsignature.CollectedFromAccountIDIn(matchedAccountIDs...))
			} else {
				// 没有匹配的账号，返回空结果
				return []service.Signature{}, 0, nil
			}
		}
		if filter.CollectedFromAccountID != nil {
			query = query.Where(dbsignature.CollectedFromAccountIDEQ(*filter.CollectedFromAccountID))
		}
	}

	// 获取总数
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, translatePersistenceError(err, service.ErrSignatureNotFound, nil)
	}

	// 应用分页和排序
	if page != nil {
		query = query.Offset(page.Offset()).Limit(page.Limit())
	}
	query = query.Order(dbent.Desc(dbsignature.FieldCreatedAt))

	// 执行查询
	models, err := query.All(ctx)
	if err != nil {
		return nil, 0, translatePersistenceError(err, service.ErrSignatureNotFound, nil)
	}

	// 转换为 service 类型
	result := make([]service.Signature, len(models))
	for i, m := range models {
		result[i] = *r.signatureToService(m)
	}

	return result, total, nil
}

// ListActive 获取所有活跃签名（用于缓存加载）。
func (r *signatureRepository) ListActive(ctx context.Context, limit int) ([]service.Signature, error) {
	query := r.client.Signature.Query().
		Where(dbsignature.StatusEQ(dbsignature.StatusActive)).
		Where(dbsignature.DeletedAtIsNil()).
		Order(dbent.Desc(dbsignature.FieldUseCount))

	if limit > 0 {
		query = query.Limit(limit)
	}

	models, err := query.All(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrSignatureNotFound, nil)
	}

	result := make([]service.Signature, len(models))
	for i, m := range models {
		result[i] = *r.signatureToService(m)
	}

	return result, nil
}

// IncrementUseCount 增加使用计数并更新最后使用时间。
func (r *signatureRepository) IncrementUseCount(ctx context.Context, id int64) error {
	now := time.Now()
	_, err := r.client.Signature.UpdateOneID(id).
		AddUseCount(1).
		SetLastUsedAt(now).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrSignatureNotFound, nil)
	}
	return nil
}

// GetStats 获取签名池统计信息。
func (r *signatureRepository) GetStats(ctx context.Context) (*service.SignatureStats, error) {
	// 总数统计
	total, err := r.client.Signature.Query().
		Where(dbsignature.DeletedAtIsNil()).
		Count(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrSignatureNotFound, nil)
	}

	// 按状态统计
	active, err := r.client.Signature.Query().
		Where(dbsignature.StatusEQ(dbsignature.StatusActive)).
		Where(dbsignature.DeletedAtIsNil()).
		Count(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrSignatureNotFound, nil)
	}

	disabled, err := r.client.Signature.Query().
		Where(dbsignature.StatusEQ(dbsignature.StatusDisabled)).
		Where(dbsignature.DeletedAtIsNil()).
		Count(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrSignatureNotFound, nil)
	}

	expired, err := r.client.Signature.Query().
		Where(dbsignature.StatusEQ(dbsignature.StatusExpired)).
		Where(dbsignature.DeletedAtIsNil()).
		Count(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrSignatureNotFound, nil)
	}

	// 计算总使用量（使用原生 SQL）
	var totalUsage int64
	rows, err := r.sql.QueryContext(ctx, `
		SELECT COALESCE(SUM(use_count), 0)
		FROM signatures
		WHERE deleted_at IS NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		if err := rows.Scan(&totalUsage); err != nil {
			return nil, err
		}
	}

	// 24小时内使用过的数量
	yesterday := time.Now().Add(-24 * time.Hour)
	recentlyUsed, err := r.client.Signature.Query().
		Where(dbsignature.LastUsedAtGTE(yesterday)).
		Where(dbsignature.DeletedAtIsNil()).
		Count(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrSignatureNotFound, nil)
	}

	return &service.SignatureStats{
		Total:        int64(total),
		Active:       int64(active),
		Disabled:     int64(disabled),
		Expired:      int64(expired),
		TotalUsage:   totalUsage,
		RecentlyUsed: int64(recentlyUsed),
	}, nil
}

// signatureToService 将数据库模型转换为服务层模型。
func (r *signatureRepository) signatureToService(m *dbent.Signature) *service.Signature {
	if m == nil {
		return nil
	}

	sig := &service.Signature{
		ID:                     m.ID,
		Value:                  m.Value,
		Hash:                   m.Hash,
		Model:                  m.Model,
		Source:                 string(m.Source),
		Status:                 string(m.Status),
		UseCount:               m.UseCount,
		Notes:                  m.Notes,
		CollectedFromAccountID: m.CollectedFromAccountID,
		CreatedAt:              m.CreatedAt,
		UpdatedAt:              m.UpdatedAt,
	}

	if m.LastUsedAt != nil {
		sig.LastUsedAt = m.LastUsedAt
	}
	if m.LastVerifiedAt != nil {
		sig.LastVerifiedAt = m.LastVerifiedAt
	}

	return sig
}

// findAccountIDsByNamePrefix 根据账号名称前缀查询匹配的账号IDs。
func (r *signatureRepository) findAccountIDsByNamePrefix(ctx context.Context, prefix string) []int64 {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT id FROM accounts
		WHERE name LIKE $1 AND deleted_at IS NULL
		LIMIT 100
	`, prefix+"%")
	if err != nil {
		return nil
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}
