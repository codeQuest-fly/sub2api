// Package admin provides HTTP handlers for administrative operations.
package admin

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// SignatureHandler handles admin signature management
type SignatureHandler struct {
	signatureService     service.SignatureService
	signaturePoolService service.SignaturePoolService
	accountRepo          service.AccountRepository
}

// NewSignatureHandler creates a new admin signature handler
func NewSignatureHandler(
	signatureService service.SignatureService,
	signaturePoolService service.SignaturePoolService,
	accountRepo service.AccountRepository,
) *SignatureHandler {
	return &SignatureHandler{
		signatureService:     signatureService,
		signaturePoolService: signaturePoolService,
		accountRepo:          accountRepo,
	}
}

// CreateSignatureRequest represents create signature request
type CreateSignatureRequest struct {
	Value string  `json:"value" binding:"required"`
	Model *string `json:"model"`
	Notes *string `json:"notes"`
}

// BatchImportSignaturesRequest represents batch import request
type BatchImportSignaturesRequest struct {
	Signatures []string `json:"signatures" binding:"required,min=1,max=1000"`
	Model      *string  `json:"model"`
	Source     string   `json:"source" binding:"omitempty,oneof=imported manual"`
}

// UpdateSignatureRequest represents update signature request
type UpdateSignatureRequest struct {
	Status string  `json:"status" binding:"required,oneof=active disabled expired"`
	Model  *string `json:"model"`
	Notes  *string `json:"notes"`
}

// BatchDeleteSignaturesRequest represents batch delete request
type BatchDeleteSignaturesRequest struct {
	IDs []int64 `json:"ids" binding:"required,min=1"`
}

// List handles GET /api/admin/signatures
func (h *SignatureHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)

	filter := &service.SignatureFilter{
		Status:            c.Query("status"),
		Source:            c.Query("source"),
		Search:            c.Query("search"),
		AccountNamePrefix: c.Query("account_name"),
	}
	if model := c.Query("model"); model != "" {
		filter.Model = &model
	}
	if accountIDStr := c.Query("collected_from_account_id"); accountIDStr != "" {
		if accountID, err := strconv.ParseInt(accountIDStr, 10, 64); err == nil {
			filter.CollectedFromAccountID = &accountID
		}
	}

	paginationParams := &pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}

	signatures, total, err := h.signatureService.List(c.Request.Context(), filter, paginationParams)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	// 收集所有需要查询的账户ID
	accountIDs := make([]int64, 0)
	accountIDSet := make(map[int64]bool)
	for _, sig := range signatures {
		if sig.CollectedFromAccountID != nil && !accountIDSet[*sig.CollectedFromAccountID] {
			accountIDs = append(accountIDs, *sig.CollectedFromAccountID)
			accountIDSet[*sig.CollectedFromAccountID] = true
		}
	}

	// 批量查询账户信息
	accountNameMap := make(map[int64]string)
	if len(accountIDs) > 0 {
		accounts, err := h.accountRepo.GetByIDs(c.Request.Context(), accountIDs)
		if err == nil {
			for _, acc := range accounts {
				accountNameMap[acc.ID] = acc.Name
			}
		}
	}

	// 转换为 DTO
	items := make([]dto.Signature, len(signatures))
	for i, sig := range signatures {
		items[i] = dto.SignatureFromService(&sig)
		// 填充账户名称
		if sig.CollectedFromAccountID != nil {
			if name, ok := accountNameMap[*sig.CollectedFromAccountID]; ok {
				items[i].CollectedFromAccountName = &name
			}
		}
	}

	response.Paginated(c, items, int64(total), page, pageSize)
}

// GetByID handles GET /api/admin/signatures/:id
func (h *SignatureHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	sig, err := h.signatureService.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == service.ErrSignatureNotFound {
			response.NotFound(c, "signature not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, dto.SignatureFromService(sig))
}

// Create handles POST /api/admin/signatures
func (h *SignatureHandler) Create(c *gin.Context) {
	var req CreateSignatureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	sig, err := h.signatureService.Create(c.Request.Context(), req.Value, req.Model, req.Notes)
	if err != nil {
		if err == service.ErrSignatureDuplicate {
			response.Error(c, http.StatusConflict, "signature already exists")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	// 使缓存失效
	h.signaturePoolService.InvalidateCache()

	response.Created(c, dto.SignatureFromService(sig))
}

// BatchImport handles POST /api/admin/signatures/batch-import
func (h *SignatureHandler) BatchImport(c *gin.Context) {
	var req BatchImportSignaturesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	source := req.Source
	if source == "" {
		source = "imported"
	}

	result, err := h.signatureService.BatchImport(c.Request.Context(), req.Signatures, req.Model, source)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	// 使缓存失效
	h.signaturePoolService.InvalidateCache()

	response.Success(c, result)
}

// Update handles PUT /api/admin/signatures/:id
func (h *SignatureHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	var req UpdateSignatureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := h.signatureService.Update(c.Request.Context(), id, req.Status, req.Model, req.Notes); err != nil {
		if err == service.ErrSignatureNotFound {
			response.NotFound(c, "signature not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	// 使缓存失效
	h.signaturePoolService.InvalidateCache()

	response.Success(c, nil)
}

// Delete handles DELETE /api/admin/signatures/:id
func (h *SignatureHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if err := h.signatureService.Delete(c.Request.Context(), id); err != nil {
		if err == service.ErrSignatureNotFound {
			response.NotFound(c, "signature not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	// 使缓存失效
	h.signaturePoolService.InvalidateCache()

	response.Success(c, nil)
}

// BatchDelete handles POST /api/admin/signatures/batch-delete
func (h *SignatureHandler) BatchDelete(c *gin.Context) {
	var req BatchDeleteSignaturesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	affected, err := h.signatureService.BatchDelete(c.Request.Context(), req.IDs)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	// 使缓存失效
	h.signaturePoolService.InvalidateCache()

	response.Success(c, gin.H{"deleted": affected})
}

// GetStats handles GET /api/admin/signatures/stats
func (h *SignatureHandler) GetStats(c *gin.Context) {
	stats, err := h.signatureService.GetStats(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	// 添加池大小
	poolSize := h.signaturePoolService.GetPoolSize()

	response.Success(c, gin.H{
		"total":         stats.Total,
		"active":        stats.Active,
		"disabled":      stats.Disabled,
		"expired":       stats.Expired,
		"total_usage":   stats.TotalUsage,
		"recently_used": stats.RecentlyUsed,
		"pool_size":     poolSize,
	})
}

// GetRandom handles GET /api/admin/signatures/random
func (h *SignatureHandler) GetRandom(c *gin.Context) {
	sig, err := h.signaturePoolService.GetRandomSignature(c.Request.Context(), nil)
	if err != nil {
		if err == service.ErrSignaturePoolEmpty {
			response.NotFound(c, "signature pool is empty")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"signature": sig,
	})
}

// DeleteByAccountID handles DELETE /api/admin/signatures/by-account/:account_id
func (h *SignatureHandler) DeleteByAccountID(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("account_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid account_id")
		return
	}

	affected, err := h.signatureService.DeleteByAccountID(c.Request.Context(), accountID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	// 使缓存失效
	h.signaturePoolService.InvalidateCache()

	response.Success(c, gin.H{"deleted": affected})
}
