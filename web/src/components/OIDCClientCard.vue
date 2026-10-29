<template>
  <div class="client">
    <div class="label first">{{ t('common.name') }}</div>
    <el-input v-model="model.name" :placeholder="t('oidc.namePlaceholder')" />
    <div class="row">
      <el-input v-model="model.id" :placeholder="t('oidc.idPlaceholder')" class="mono grow">
        <template #prepend>Client ID</template>
        <template #append>
          <el-button :icon="Copy" :aria-label="t('oidc.copy')" @click="copyText(model.id)" />
        </template>
      </el-input>
    </div>
    <div class="row">
      <el-checkbox v-model="model.public">{{ t('oidc.public') }}</el-checkbox>
      <span class="form-help inline">{{ model.public ? t('oidc.publicHelp') : t('oidc.confidentialHelp') }}</span>
    </div>
    <template v-if="!model.public">
      <div class="label">Client Secret</div>
      <div class="row">
        <el-input
          v-model="model.secret"
          class="mono grow"
          autocomplete="new-password"
          :type="model.secret ? 'text' : 'password'"
          :placeholder="model.hasSecret ? t('oidc.secretKeep') : t('oidc.secretPlaceholder')"
        >
          <template #append>
            <el-button :icon="Copy" :disabled="!model.secret" :aria-label="t('oidc.copy')" @click="copyText(model.secret)" />
          </template>
        </el-input>
        <el-button :icon="RefreshCw" @click="model.secret = randomSecret()">{{ t('oidc.generate') }}</el-button>
      </div>
      <el-alert v-if="model.secret" type="warning" :closable="false" show-icon :title="t('oidc.secretOnce')" class="once" />
    </template>
    <div class="label">{{ t('oidc.redirectUris') }}</div>
    <el-input v-model="redirectText" type="textarea" :rows="2" class="mono" :placeholder="t('oidc.redirectPlaceholder')" />
    <div class="form-help">{{ t('oidc.redirectHelp') }}</div>
    <div class="label">{{ t('oidc.allow') }}</div>
    <el-input v-model="allowText" type="textarea" :rows="2" class="mono" :placeholder="t('oidc.allowPlaceholder')" />
    <div class="form-help">{{ t('oidc.allowHelp') }}</div>
    <div class="label">{{ t('common.remark') }}</div>
    <el-input v-model="model.remark" :placeholder="t('common.remark')" />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Copy, RefreshCw } from 'lucide-vue-next'
import { copyText } from '../composables/useClipboard'
import { splitLines, useListText } from '../composables/useListText'
import { randomSecret, type OIDCClient } from '../oidc'

const { t } = useI18n()
const model = defineModel<OIDCClient>({ required: true })
const redirectText = useListText(() => model.value.redirectUris, (v) => (model.value.redirectUris = v), { split: splitLines })
const allowText = useListText(() => model.value.allow, (v) => (model.value.allow = v), { split: splitLines })
</script>

<style scoped>
/* 客户端的完整编辑表单，放在 OIDCProvider 的编辑弹窗里 */
.client {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.grow {
  flex: 1;
  min-width: 200px;
}
@media (max-width: 640px) {
  .grow {
    flex-basis: 100%;
  }
}
.label {
  margin-top: 4px;
  font-size: 13px;
  color: var(--el-text-color-regular);
}
.form-help.inline {
  margin: 0;
}
.once {
  padding: 4px 10px;
}
.label.first {
  margin-top: 0;
}
</style>
