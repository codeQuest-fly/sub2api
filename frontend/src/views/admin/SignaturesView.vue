<template>
  <AppLayout>
    <TablePageLayout>
      <template #actions>
        <div class="flex justify-end gap-3">
          <button
            @click="loadSignatures"
            :disabled="loading"
            class="btn btn-secondary"
            :title="t('common.refresh')"
          >
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
          <button @click="showImportDialog = true" class="btn btn-secondary">
            <Icon name="upload" size="md" class="mr-1" />
            {{ t('admin.signatures.batchImport') }}
          </button>
          <button @click="showCreateDialog = true" class="btn btn-primary">
            <Icon name="plus" size="md" class="mr-1" />
            {{ t('admin.signatures.create') }}
          </button>
        </div>
      </template>

      <template #filters>
        <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <!-- Stats Cards -->
          <div class="flex gap-4 text-sm">
            <div class="flex items-center gap-2">
              <span class="text-gray-500 dark:text-gray-400">{{ t('admin.signatures.total') }}:</span>
              <span class="font-medium text-gray-900 dark:text-white">{{ stats.total }}</span>
            </div>
            <div class="flex items-center gap-2">
              <span class="text-gray-500 dark:text-gray-400">{{ t('admin.signatures.active') }}:</span>
              <span class="font-medium text-green-600 dark:text-green-400">{{ stats.active }}</span>
            </div>
            <div class="flex items-center gap-2">
              <span class="text-gray-500 dark:text-gray-400">{{ t('admin.signatures.poolSize') }}:</span>
              <span class="font-medium text-blue-600 dark:text-blue-400">{{ stats.pool_size }}</span>
            </div>
          </div>

          <div class="flex gap-2">
            <div class="max-w-xs flex-1">
              <input
                v-model="searchQuery"
                type="text"
                :placeholder="t('admin.signatures.search')"
                class="input"
                @input="handleSearch"
              />
            </div>
            <div class="max-w-xs flex-1">
              <input
                v-model="accountNameQuery"
                type="text"
                :placeholder="t('admin.signatures.accountName')"
                class="input"
                @blur="handleAccountNameSearch"
              />
            </div>
            <Select
              v-model="filters.status"
              :options="statusOptions"
              class="w-32"
              @change="loadSignatures"
            />
            <Select
              v-model="filters.source"
              :options="sourceOptions"
              class="w-32"
              @change="loadSignatures"
            />
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="signatures" :loading="loading">
          <template #cell-value="{ value }">
            <div class="flex items-center space-x-2">
              <code class="max-w-xs truncate font-mono text-xs text-gray-900 dark:text-gray-100">
                {{ truncateValue(value) }}
              </code>
              <button
                @click="copyToClipboard(value)"
                :class="[
                  'flex items-center transition-colors',
                  copiedValue === value
                    ? 'text-green-500'
                    : 'text-gray-400 hover:text-gray-600 dark:hover:text-gray-300'
                ]"
                :title="t('common.copy')"
              >
                <Icon v-if="copiedValue !== value" name="copy" size="sm" :stroke-width="2" />
                <svg v-else class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
              </button>
            </div>
          </template>

          <template #cell-model="{ value }">
            <span v-if="value" class="badge badge-info">{{ value }}</span>
            <span v-else class="text-gray-400">-</span>
          </template>

          <template #cell-collected_from_account_name="{ value }">
            <span v-if="value" class="text-sm text-gray-700 dark:text-gray-300">{{ value }}</span>
            <span v-else class="text-gray-400">-</span>
          </template>

          <template #cell-source="{ value }">
            <span :class="['badge', getSourceClass(value)]">
              {{ t(`admin.signatures.source.${value}`) }}
            </span>
          </template>

          <template #cell-status="{ value }">
            <span :class="['badge', getStatusClass(value)]">
              {{ t(`admin.signatures.status.${value}`) }}
            </span>
          </template>

          <template #cell-use_count="{ value }">
            <span class="text-sm text-gray-600 dark:text-gray-300">{{ value }}</span>
          </template>

          <template #cell-last_used_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">
              {{ value ? formatDateTime(value) : '-' }}
            </span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">
              {{ formatDateTime(value) }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center space-x-1">
              <button
                @click="handleEdit(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-600 dark:hover:text-gray-300"
                :title="t('common.edit')"
              >
                <Icon name="edit" size="sm" />
              </button>
              <button
                @click="handleDelete(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                :title="t('common.delete')"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- Create Dialog -->
    <BaseDialog
      :show="showCreateDialog"
      :title="t('admin.signatures.create')"
      width="normal"
      @close="showCreateDialog = false"
    >
      <form id="create-signature-form" @submit.prevent="handleCreate" class="space-y-4">
        <div>
          <label class="input-label">{{ t('admin.signatures.value') }}</label>
          <textarea
            v-model="createForm.value"
            required
            rows="4"
            class="input font-mono text-xs"
            :placeholder="t('admin.signatures.valuePlaceholder')"
          />
        </div>
        <div>
          <label class="input-label">
            {{ t('admin.signatures.model') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('common.optional') }})</span>
          </label>
          <input
            v-model="createForm.model"
            type="text"
            class="input"
            :placeholder="t('admin.signatures.modelPlaceholder')"
          />
        </div>
        <div>
          <label class="input-label">
            {{ t('admin.signatures.notes') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('common.optional') }})</span>
          </label>
          <input
            v-model="createForm.notes"
            type="text"
            class="input"
          />
        </div>
      </form>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="showCreateDialog = false">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="create-signature-form"
          class="btn btn-primary"
          :disabled="createLoading"
        >
          {{ createLoading ? t('common.creating') : t('common.create') }}
        </button>
      </template>
    </BaseDialog>

    <!-- Edit Dialog -->
    <BaseDialog
      :show="showEditDialog"
      :title="t('admin.signatures.edit')"
      width="normal"
      @close="showEditDialog = false"
    >
      <form id="edit-signature-form" @submit.prevent="handleUpdate" class="space-y-4">
        <div>
          <label class="input-label">{{ t('admin.signatures.status.label') }}</label>
          <Select
            v-model="editForm.status"
            :options="editStatusOptions"
            class="w-full"
          />
        </div>
        <div>
          <label class="input-label">
            {{ t('admin.signatures.model') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('common.optional') }})</span>
          </label>
          <input
            v-model="editForm.model"
            type="text"
            class="input"
            :placeholder="t('admin.signatures.modelPlaceholder')"
          />
        </div>
        <div>
          <label class="input-label">
            {{ t('admin.signatures.notes') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('common.optional') }})</span>
          </label>
          <input
            v-model="editForm.notes"
            type="text"
            class="input"
          />
        </div>
      </form>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="showEditDialog = false">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="edit-signature-form"
          class="btn btn-primary"
          :disabled="editLoading"
        >
          {{ editLoading ? t('common.saving') : t('common.save') }}
        </button>
      </template>
    </BaseDialog>

    <!-- Import Dialog -->
    <BaseDialog
      :show="showImportDialog"
      :title="t('admin.signatures.batchImport')"
      width="wide"
      @close="showImportDialog = false"
    >
      <form id="import-signature-form" @submit.prevent="handleImport" class="space-y-4">
        <div>
          <label class="input-label">{{ t('admin.signatures.importValues') }}</label>
          <textarea
            v-model="importForm.signatures"
            required
            rows="10"
            class="input font-mono text-xs"
            :placeholder="t('admin.signatures.importPlaceholder')"
          />
          <p class="mt-1 text-xs text-gray-500">{{ t('admin.signatures.importHint') }}</p>
        </div>
        <div>
          <label class="input-label">
            {{ t('admin.signatures.model') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('common.optional') }})</span>
          </label>
          <input
            v-model="importForm.model"
            type="text"
            class="input"
            :placeholder="t('admin.signatures.modelPlaceholder')"
          />
        </div>
      </form>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="showImportDialog = false">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="import-signature-form"
          class="btn btn-primary"
          :disabled="importLoading"
        >
          {{ importLoading ? t('admin.signatures.importing') : t('admin.signatures.import') }}
        </button>
      </template>
    </BaseDialog>

    <!-- Delete Confirmation -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.signatures.deleteConfirm')"
      :message="t('admin.signatures.deleteMessage')"
      :confirm-text="t('common.delete')"
      :loading="deleteLoading"
      variant="danger"
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import signaturesAPI, { type Signature, type SignatureStats } from '@/api/admin/signatures'

const { t } = useI18n()
const appStore = useAppStore()

// Toast helper
const toast = {
  success: (msg: string) => appStore.showSuccess(msg),
  error: (msg: string) => appStore.showError(msg)
}

// State
const loading = ref(false)
const signatures = ref<Signature[]>([])
const stats = ref<SignatureStats>({
  total: 0,
  active: 0,
  disabled: 0,
  expired: 0,
  total_usage: 0,
  recently_used: 0,
  pool_size: 0
})
const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0
})
const filters = reactive({
  status: '',
  source: '',
  search: '',
  account_name: '',
  collected_from_account_id: null as number | null
})
const searchQuery = ref('')
const accountNameQuery = ref('')
const copiedValue = ref('')

// Dialogs
const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const showImportDialog = ref(false)
const showDeleteDialog = ref(false)
const createLoading = ref(false)
const editLoading = ref(false)
const importLoading = ref(false)
const deleteLoading = ref(false)

// Forms
const createForm = reactive({
  value: '',
  model: '',
  notes: ''
})

const editForm = reactive({
  id: 0,
  status: 'active',
  model: '',
  notes: ''
})

const importForm = reactive({
  signatures: '',
  model: ''
})

const deleteTarget = ref<Signature | null>(null)

// Options
const statusOptions = computed(() => [
  { value: '', label: t('admin.signatures.allStatus') },
  { value: 'active', label: t('admin.signatures.status.active') },
  { value: 'disabled', label: t('admin.signatures.status.disabled') },
  { value: 'expired', label: t('admin.signatures.status.expired') }
])

const sourceOptions = computed(() => [
  { value: '', label: t('admin.signatures.allSource') },
  { value: 'collected', label: t('admin.signatures.source.collected') },
  { value: 'imported', label: t('admin.signatures.source.imported') },
  { value: 'manual', label: t('admin.signatures.source.manual') }
])

const editStatusOptions = computed(() => [
  { value: 'active', label: t('admin.signatures.status.active') },
  { value: 'disabled', label: t('admin.signatures.status.disabled') },
  { value: 'expired', label: t('admin.signatures.status.expired') }
])

// Columns
const columns = computed(() => [
  { key: 'id', label: 'ID', width: '80px' },
  { key: 'value', label: t('admin.signatures.value'), width: '280px' },
  { key: 'model', label: t('admin.signatures.model'), width: '150px' },
  { key: 'collected_from_account_name', label: t('admin.signatures.account'), width: '120px' },
  { key: 'source', label: t('admin.signatures.source.label'), width: '100px' },
  { key: 'status', label: t('admin.signatures.status.label'), width: '100px' },
  { key: 'use_count', label: t('admin.signatures.useCount'), width: '100px' },
  { key: 'last_used_at', label: t('admin.signatures.lastUsedAt'), width: '160px' },
  { key: 'created_at', label: t('common.createdAt'), width: '160px' },
  { key: 'actions', label: t('common.actions'), width: '100px' }
])

// Methods
async function loadSignatures() {
  loading.value = true
  try {
    const response = await signaturesAPI.list(pagination.page, pagination.page_size, {
      status: filters.status || undefined,
      source: filters.source || undefined,
      search: filters.search || undefined,
      account_name: filters.account_name || undefined,
      collected_from_account_id: filters.collected_from_account_id || undefined
    })
    signatures.value = response.items
    pagination.total = response.total
  } catch (error: any) {
    toast.error(error.message || t('common.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function loadStats() {
  try {
    const response = await signaturesAPI.getStats()
    stats.value = response
  } catch (error) {
    // Silently fail for stats
  }
}

function handleSearch() {
  filters.search = searchQuery.value
  pagination.page = 1
  loadSignatures()
}

function handleAccountNameSearch() {
  filters.account_name = accountNameQuery.value
  pagination.page = 1
  loadSignatures()
}

function handlePageChange(page: number) {
  pagination.page = page
  loadSignatures()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  loadSignatures()
}

function truncateValue(value: string): string {
  if (value.length <= 50) return value
  return value.substring(0, 25) + '...' + value.substring(value.length - 20)
}

async function copyToClipboard(value: string) {
  try {
    await navigator.clipboard.writeText(value)
    copiedValue.value = value
    setTimeout(() => {
      copiedValue.value = ''
    }, 2000)
  } catch {
    toast.error(t('common.copyFailed'))
  }
}

function getStatusClass(status: string): string {
  switch (status) {
    case 'active':
      return 'badge-success'
    case 'disabled':
      return 'badge-warning'
    case 'expired':
      return 'badge-error'
    default:
      return 'badge-secondary'
  }
}

function getSourceClass(source: string): string {
  switch (source) {
    case 'collected':
      return 'badge-info'
    case 'imported':
      return 'badge-primary'
    case 'manual':
      return 'badge-secondary'
    default:
      return 'badge-secondary'
  }
}

async function handleCreate() {
  if (!createForm.value.trim()) {
    toast.error(t('admin.signatures.valueRequired'))
    return
  }

  createLoading.value = true
  try {
    await signaturesAPI.create({
      value: createForm.value.trim(),
      model: createForm.model.trim() || undefined,
      notes: createForm.notes.trim() || undefined
    })
    toast.success(t('admin.signatures.createSuccess'))
    showCreateDialog.value = false
    createForm.value = ''
    createForm.model = ''
    createForm.notes = ''
    loadSignatures()
    loadStats()
  } catch (error: any) {
    toast.error(error.message || t('admin.signatures.createFailed'))
  } finally {
    createLoading.value = false
  }
}

function handleEdit(row: Signature) {
  editForm.id = row.id
  editForm.status = row.status
  editForm.model = row.model || ''
  editForm.notes = row.notes || ''
  showEditDialog.value = true
}

async function handleUpdate() {
  editLoading.value = true
  try {
    await signaturesAPI.update(editForm.id, {
      status: editForm.status as 'active' | 'disabled' | 'expired',
      model: editForm.model.trim() || null,
      notes: editForm.notes.trim() || null
    })
    toast.success(t('admin.signatures.updateSuccess'))
    showEditDialog.value = false
    loadSignatures()
    loadStats()
  } catch (error: any) {
    toast.error(error.message || t('admin.signatures.updateFailed'))
  } finally {
    editLoading.value = false
  }
}

async function handleImport() {
  const lines = importForm.signatures
    .split('\n')
    .map(line => line.trim())
    .filter(line => line.length > 0)

  if (lines.length === 0) {
    toast.error(t('admin.signatures.importEmpty'))
    return
  }

  if (lines.length > 1000) {
    toast.error(t('admin.signatures.importTooMany'))
    return
  }

  importLoading.value = true
  try {
    const result = await signaturesAPI.batchImport({
      signatures: lines,
      model: importForm.model.trim() || undefined,
      source: 'imported'
    })
    toast.success(
      t('admin.signatures.importSuccess', {
        imported: result.imported,
        duplicated: result.duplicated,
        failed: result.failed
      })
    )
    showImportDialog.value = false
    importForm.signatures = ''
    importForm.model = ''
    loadSignatures()
    loadStats()
  } catch (error: any) {
    toast.error(error.message || t('admin.signatures.importFailed'))
  } finally {
    importLoading.value = false
  }
}

function handleDelete(row: Signature) {
  deleteTarget.value = row
  showDeleteDialog.value = true
}

async function confirmDelete() {
  if (!deleteTarget.value) return

  deleteLoading.value = true
  try {
    await signaturesAPI.delete(deleteTarget.value.id)
    toast.success(t('admin.signatures.deleteSuccess'))
    showDeleteDialog.value = false
    deleteTarget.value = null
    loadSignatures()
    loadStats()
  } catch (error: any) {
    toast.error(error.message || t('admin.signatures.deleteFailed'))
  } finally {
    deleteLoading.value = false
  }
}

// Lifecycle
onMounted(() => {
  loadSignatures()
  loadStats()
})
</script>
