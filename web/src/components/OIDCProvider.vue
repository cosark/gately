<template>
  <div class="oidc">
    <el-form-item :label="t('oidc.enable')">
      <el-switch v-model="model.enable" />
    </el-form-item>
    <el-form-item :label="t('oidc.issuer')">
      <el-input v-model="model.issuer" :placeholder="t('oidc.issuerPlaceholder')" />
      <div class="form-help">{{ t('oidc.issuerHelp') }}</div>
    </el-form-item>
    <el-form-item v-if="issuer" :label="t('oidc.discovery')">
      <div class="endpoints">
        <div class="ep main">
          <code ref="discoveryEl" class="mono">{{ issuer + OIDC_DISCOVERY }}</code>
          <el-button size="small" text :icon="Copy" :aria-label="t('oidc.copy')" @click="copyText(issuer + OIDC_DISCOVERY, { el: discoveryEl })" />
        </div>
        <div class="form-help">{{ t('oidc.discoveryHelp') }}</div>
        <el-collapse class="more">
          <el-collapse-item :title="t('oidc.endpoints')" name="ep">
            <div v-for="(path, key) in OIDC_ENDPOINTS" :key="key" class="ep">
              <span class="ep-name">{{ t(`oidc.ep.${key}`) }}</span>
              <code class="mono">{{ issuer + path }}</code>
              <el-button size="small" text :icon="Copy" :aria-label="t('oidc.copy')" @click="copyText(issuer + path)" />
            </div>
            <div class="form-help">{{ t('oidc.scopesHelp') }}</div>
          </el-collapse-item>
        </el-collapse>
      </div>
    </el-form-item>
    <el-form-item :label="t('oidc.clients')">
      <div class="clients">
        <!-- 列表只显示摘要，完整配置在编辑弹窗里修改，客户端多时设置对话框不会过长 -->
        <div v-if="!model.clients.length" class="empty">{{ t('oidc.noClients') }}</div>
        <div v-for="(c, i) in model.clients" :key="i" class="item">
          <div class="item-head">
            <span class="name" :class="{ unnamed: !c.name.trim() }">{{ c.name.trim() || t('oidc.unnamed') }}</span>
            <el-tag size="small" :type="c.public ? 'info' : 'primary'">{{ c.public ? t('oidc.public') : t('oidc.confidential') }}</el-tag>
            <el-tag v-if="!c.public && c.secret" size="small" type="warning">{{ t('oidc.secretUnsaved') }}</el-tag>
            <el-tag v-if="!c.redirectUris.length" size="small" type="danger">{{ t('oidc.noRedirect') }}</el-tag>
            <span class="item-actions">
              <el-button text size="small" :icon="Pencil" @click="openEditor(i)">{{ t('common.edit') }}</el-button>
              <el-button text size="small" type="danger" :icon="Trash2" @click="model.clients.splice(i, 1)">{{ t('common.delete') }}</el-button>
            </span>
          </div>
          <div class="meta">
            <span class="mono">Client ID: {{ c.id }}</span>
            <el-button size="small" text :icon="Copy" :aria-label="t('oidc.copy')" class="copy" @click="copyText(c.id)" />
            <span v-if="c.redirectUris.length"> · {{ t('oidc.redirectCount', { count: c.redirectUris.length }) }}</span>
            <span> · {{ c.allow.length ? t('oidc.allowCount', { count: c.allow.length }) : t('oidc.allowAll') }}</span>
            <span v-if="c.remark"> · {{ c.remark }}</span>
          </div>
        </div>
        <div class="actions">
          <el-button size="small" :icon="Plus" @click="openEditor(-1)">{{ t('oidc.addClient') }}</el-button>
          <a :href="docs" target="_blank" rel="noopener" class="doc-link">{{ t('oidc.docs') }}</a>
        </div>
      </div>
    </el-form-item>

    <el-dialog
      v-model="editorVisible"
      :title="editingIndex < 0 ? t('oidc.addClient') : t('oidc.editClient')"
      width="600px"
      append-to-body
      :fullscreen="isMobile"
    >
      <OIDCClientCard v-if="draft" v-model="draft" />
      <div class="form-help apply-help">{{ t('oidc.applyHelp') }}</div>
      <template #footer>
        <el-button @click="editorVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="applyEditor">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, toRaw } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Copy, Pencil, Plus, Trash2 } from 'lucide-vue-next'
import OIDCClientCard from './OIDCClientCard.vue'
import { copyText } from '../composables/useClipboard'
import { useFormLayout } from '../composables/useIsMobile'
import { docsUrl } from '../docsUrl'
import { newClient, OIDC_DISCOVERY, OIDC_ENDPOINTS, type OIDCClient, type OIDCSettings } from '../oidc'

const { t, locale } = useI18n()
const model = defineModel<OIDCSettings>({ required: true })
const { isMobile } = useFormLayout()
const discoveryEl = ref<HTMLElement | null>(null)
// 规范化后的签发者地址（去掉末尾斜杠），未填写时不显示端点
const issuer = computed(() => model.value.issuer.trim().replace(/\/+$/, ''))
const docs = computed(() => docsUrl(locale.value, '/docs/modules/oidc'))

// 编辑弹窗：draft 为正在编辑的副本，editingIndex 为原客户端在列表中的位置，-1 表示新增
const editorVisible = ref(false)
const editingIndex = ref(-1)
const draft = ref<OIDCClient>()

// openEditor 打开编辑弹窗，编辑的是副本，点「确定」才写回列表，取消不影响原数据
function openEditor(i: number) {
  editingIndex.value = i
  if (i < 0) {
    draft.value = newClient(model.value.clients.map((c) => c.id))
  } else {
    draft.value = JSON.parse(JSON.stringify(toRaw(model.value.clients[i])))
  }
  editorVisible.value = true
}

// applyEditor 校验 Client ID 后把副本写回列表；其余字段的合法性由保存设置时后端检查
function applyEditor() {
  const d = draft.value
  if (!d) {
    return
  }
  d.id = d.id.trim()
  if (!d.id) {
    ElMessage.warning(t('oidc.idRequired'))
    return
  }
  if (model.value.clients.some((c, i) => i !== editingIndex.value && c.id.trim() === d.id)) {
    ElMessage.warning(t('oidc.idDuplicate', { id: d.id }))
    return
  }
  if (editingIndex.value < 0) {
    model.value.clients.push(d)
  } else {
    Object.assign(model.value.clients[editingIndex.value], d)
  }
  editorVisible.value = false
}
</script>

<style scoped>
.oidc {
  width: 100%;
}
.endpoints,
.clients {
  width: 100%;
}
.ep {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.ep code {
  overflow-wrap: anywhere;
  font-size: 12.5px;
}
.ep.main code {
  font-size: 13px;
}
.ep-name {
  flex: none;
  width: 92px;
  color: var(--el-text-color-secondary);
  font-size: 12.5px;
}
.more {
  margin-top: 4px;
  border: none;
}
.empty {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  margin-bottom: 8px;
}
.item {
  border: 1px solid var(--g-border);
  border-radius: 8px;
  padding: 6px 10px;
  margin-bottom: 8px;
}
.item-head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.name {
  font-weight: 600;
  overflow-wrap: anywhere;
}
.name.unnamed {
  font-weight: normal;
  color: var(--el-text-color-placeholder);
}
.item-actions {
  margin-left: auto;
}
.meta {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.6;
  overflow-wrap: anywhere;
}
.meta .copy {
  height: 18px;
  padding: 0 4px;
}
.apply-help {
  margin-top: 10px;
}
.actions {
  display: flex;
  align-items: center;
  gap: 12px;
}
.doc-link {
  font-size: 13px;
  color: var(--el-color-primary);
}
</style>
