<template>
  <!-- 排序模式工具栏：显示操作提示、已选数量，并把选中项批量移到某个分组 -->
  <div class="sort-bar">
    <div class="sort-hint">{{ isMobile ? t('groupSort.hintMobile') : t('groupSort.hint') }}</div>
    <div class="sort-ops">
      <span class="sort-count">{{ t('groupSort.selectedCount', { count: selectedCount }) }}</span>
      <el-button link type="primary" :disabled="!total || selectedCount === total" @click="emit('selectAll')">{{ t('groupSort.selectAll') }}</el-button>
      <el-button link :disabled="!selectedCount" @click="emit('clear')">{{ t('groupSort.clearSelection') }}</el-button>
      <div class="sort-group">
        <el-autocomplete
          v-model="target"
          size="small"
          clearable
          class="sort-group-input"
          :maxlength="64"
          :fetch-suggestions="(q: string, cb: (v: { value: string }[]) => void) => cb(groupSuggestions(groups, q))"
          :placeholder="t('groupSort.targetPlaceholder')"
          @keyup.enter="apply"
        />
        <el-button size="small" type="primary" :icon="FolderInput" :disabled="!selectedCount || busy" @click="apply">
          {{ target.trim() ? t('groupSort.moveToGroup') : t('groupSort.removeFromGroup') }}
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { FolderInput } from 'lucide-vue-next'
import { groupSuggestions } from '../groupSort'
import { useIsMobile } from '../composables/useIsMobile'

// groups 为已有分组名称，用于输入提示；busy 为保存中，期间不能重复提交
const props = defineProps<{ selectedCount: number; total: number; groups: string[]; busy?: boolean }>()
const emit = defineEmits<{ selectAll: []; clear: []; setGroup: [name: string] }>()
const { t } = useI18n()
const isMobile = useIsMobile()
const target = ref('')

function apply() {
  if (!props.selectedCount || props.busy) {
    return
  }
  emit('setGroup', target.value.trim())
}
</script>

<style scoped>
.sort-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px 16px;
  margin-bottom: 12px;
  padding: 8px 12px;
  border: 1px dashed var(--el-color-primary-light-5);
  border-radius: var(--g-radius);
  background: var(--el-color-primary-light-9);
  font-size: 13px;
}
.sort-hint {
  color: var(--el-text-color-regular);
}
.sort-ops {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 12px;
}
.sort-ops :deep(.el-button + .el-button) {
  margin-left: 0;
}
.sort-count {
  color: var(--el-text-color-secondary);
}
.sort-group {
  display: flex;
  align-items: center;
  gap: 6px;
}
.sort-group-input {
  width: 180px;
}
@media (max-width: 767.98px) {
  .sort-group {
    width: 100%;
  }
  .sort-group-input {
    flex: 1;
  }
}
</style>
