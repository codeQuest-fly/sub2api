/**
 * Admin Signatures API endpoints
 */

import { apiClient } from '../client'
import type { BasePaginationResponse } from '@/types'

// Signature 类型定义
export interface Signature {
  id: number
  value: string
  hash: string
  model: string | null
  source: 'collected' | 'imported' | 'manual'
  status: 'active' | 'disabled' | 'expired'
  use_count: number
  last_used_at: string | null
  last_verified_at: string | null
  notes: string | null
  collected_from_account_id: number | null
  collected_from_account_name: string | null
  created_at: string
  updated_at: string
}

export interface SignatureStats {
  total: number
  active: number
  disabled: number
  expired: number
  total_usage: number
  recently_used: number
  pool_size: number
}

export interface BatchImportResult {
  total: number
  imported: number
  duplicated: number
  failed: number
}

export interface CreateSignatureRequest {
  value: string
  model?: string
  notes?: string
}

export interface BatchImportSignaturesRequest {
  signatures: string[]
  model?: string
  source?: 'imported' | 'manual'
}

export interface UpdateSignatureRequest {
  status: 'active' | 'disabled' | 'expired'
  model?: string | null
  notes?: string | null
}

export interface SignatureListFilters {
  status?: string
  source?: string
  model?: string
  search?: string
  account_name?: string
  collected_from_account_id?: number
}

export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters?: SignatureListFilters
): Promise<BasePaginationResponse<Signature>> {
  const { data } = await apiClient.get<BasePaginationResponse<Signature>>('/admin/signatures', {
    params: { page, page_size: pageSize, ...filters }
  })
  return data
}

export async function getById(id: number): Promise<Signature> {
  const { data } = await apiClient.get<Signature>(`/admin/signatures/${id}`)
  return data
}

export async function create(request: CreateSignatureRequest): Promise<Signature> {
  const { data } = await apiClient.post<Signature>('/admin/signatures', request)
  return data
}

export async function batchImport(request: BatchImportSignaturesRequest): Promise<BatchImportResult> {
  const { data } = await apiClient.post<BatchImportResult>('/admin/signatures/batch-import', request)
  return data
}

export async function update(id: number, request: UpdateSignatureRequest): Promise<void> {
  await apiClient.put(`/admin/signatures/${id}`, request)
}

export async function deleteSignature(id: number): Promise<void> {
  await apiClient.delete(`/admin/signatures/${id}`)
}

export async function batchDelete(ids: number[]): Promise<{ deleted: number }> {
  const { data } = await apiClient.post<{ deleted: number }>('/admin/signatures/batch-delete', { ids })
  return data
}

export async function deleteByAccountId(accountId: number): Promise<{ deleted: number }> {
  const { data } = await apiClient.delete<{ deleted: number }>(`/admin/signatures/by-account/${accountId}`)
  return data
}

export async function getStats(): Promise<SignatureStats> {
  const { data } = await apiClient.get<SignatureStats>('/admin/signatures/stats')
  return data
}

export async function getRandom(): Promise<{ signature: string }> {
  const { data } = await apiClient.get<{ signature: string }>('/admin/signatures/random')
  return data
}

const signaturesAPI = {
  list,
  getById,
  create,
  batchImport,
  update,
  delete: deleteSignature,
  batchDelete,
  deleteByAccountId,
  getStats,
  getRandom
}

export default signaturesAPI
