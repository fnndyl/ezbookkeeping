<template>
    <v-menu v-model="open" :close-on-content-click="false" :disabled="disabled" max-width="520">
        <template #activator="{ props: activatorProps }">
            <div class="import-account-selection d-flex align-center">
                <slot name="leading" />
                <button type="button" class="import-account-activator d-flex align-center text-start"
                        v-bind="activatorProps" :disabled="disabled" :aria-label="label || tt('Account')" @click.stop>
                    <slot />
                    <v-icon class="ms-auto" size="18" :icon="open ? mdiMenuUp : mdiMenuDown" v-if="!hideDropdown" />
                </button>
            </div>
        </template>
        <div @click.stop @keydown.esc.stop.prevent="open = false">
            <v-card width="480">
                <v-text-field ref="searchInput" class="ma-2" density="compact" hide-details
                              :prepend-inner-icon="mdiMagnify" :placeholder="tt('Find account')" :aria-label="tt('Find account')"
                              v-model="search" @keydown="onSearchKeydown" />
                <div class="import-account-columns" v-if="filteredOptions.length">
                    <v-list class="import-account-list" :aria-label="tt('Account')">
                        <v-list-item v-for="group in filteredGroups" :key="group.category"
                                     :active="group.category === activeGroup" :title="tt(group.name)" :subtitle="group.displayBalance"
                                     @click="browseGroup(group.category)">
                            <template #prepend>
                                <ItemIcon class="me-2" icon-type="account" :icon-id="group.icon" />
                            </template>
                        </v-list-item>
                    </v-list>
                    <v-list ref="resultList" class="import-account-list" role="listbox" :aria-label="tt('Account')">
                        <v-list-item v-for="option in activeOptions" :key="option.id"
                                     :active="option.id === highlightedOption?.id" :title="option.account.name" :subtitle="option.account.displayBalance"
                                     role="option" :aria-selected="option.id === highlightedOption?.id" @click="select(option.id)">
                            <template #prepend>
                                <ItemIcon class="me-2" :icon-type="getAccountIconType(option.account.iconType)"
                                          :icon-id="option.account.icon" :color="option.account.color" />
                            </template>
                        </v-list-item>
                    </v-list>
                </div>
                <div class="text-body-large mx-4 mt-3 mb-2" v-else>{{ tt('No available account') }}</div>
            </v-card>
        </div>
    </v-menu>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, useTemplateRef, watch } from 'vue';
import { VTextField } from 'vuetify/components/VTextField';
import { VList } from 'vuetify/components/VList';
import { useI18n } from '@/locales/helpers.ts';
import { mdiMagnify, mdiMenuDown, mdiMenuUp } from '@mdi/js';
import { NormalizedText } from '@/core/text.ts';
import { getAccountIconType } from '@/lib/icon.ts';
import type { CategorizedAccountWithDisplayBalance } from '@/models/account.ts';

const props = defineProps<{
    groups: CategorizedAccountWithDisplayBalance[];
    disabled?: boolean;
    hideDropdown?: boolean;
    label?: string;
}>();
const emit = defineEmits<{
    (e: 'select', id: string): void;
    (e: 'update:open', open: boolean): void;
}>();
const { tt } = useI18n();
const open = ref(false);
const search = ref('');
const highlightedIndex = ref(-1);
const searchInput = useTemplateRef<VTextField>('searchInput');
const resultList = useTemplateRef<VList>('resultList');
const filteredOptions = computed(() => {
    const query = NormalizedText.normalizeForSearch(search.value);
    return props.groups.flatMap(group => group.accounts.map(account => ({
        id: account.id,
        group: group.category,
        account,
        searchText: `${tt(group.name)} ${account.name}`
    }))).filter(option => NormalizedText.normalizeForSearch(option.searchText).includes(query));
});
const highlightedOption = computed(() => filteredOptions.value[highlightedIndex.value]);
const activeGroup = computed(() => highlightedOption.value?.group);
const filteredGroups = computed(() => props.groups.filter(group => filteredOptions.value.some(option => option.group === group.category)));
const activeOptions = computed(() => filteredOptions.value.filter(option => option.group === activeGroup.value));

function browseGroup(category: number): void {
    const index = filteredOptions.value.findIndex(option => option.group === category);
    if (index >= 0) {
        highlightedIndex.value = index;
    }
}

function select(id: string): void {
    if (props.disabled || !filteredOptions.value.some(option => option.id === id)) {
        return;
    }
    emit('select', id);
    open.value = false;
}

function onSearchKeydown(event: KeyboardEvent): void {
    if (event.isComposing) {
        return;
    }
    if (event.key === 'Enter') {
        event.preventDefault();
        event.stopPropagation();
        const option = filteredOptions.value[highlightedIndex.value];
        if (option) {
            select(option.id);
        }
    } else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
        event.preventDefault();
        event.stopPropagation();
        const count = filteredOptions.value.length;
        if (count) {
            highlightedIndex.value = highlightedIndex.value < 0
                ? (event.key === 'ArrowDown' ? 0 : count - 1)
                : (highlightedIndex.value + (event.key === 'ArrowDown' ? 1 : count - 1)) % count;
        }
    }
}

watch(filteredOptions, () => { highlightedIndex.value = search.value.trim() ? 0 : -1; });
watch(highlightedIndex, async () => {
    await nextTick();
    resultList.value?.$el.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' });
});
onBeforeUnmount(() => { if (open.value) emit('update:open', false); });
watch(() => props.disabled, disabled => { if (disabled) open.value = false; });
watch(open, async value => {
    emit('update:open', value);
    if (value) {
        search.value = '';
        highlightedIndex.value = -1;
        await nextTick();
        // Lazy menu content may mount after the first tick.
        setTimeout(() => { if (open.value) searchInput.value?.focus(); }, 50);
    }
});
</script>

<style scoped>
.import-account-selection {
    width: 260px;
}

.import-account-activator {
    appearance: none;
    border: 0;
    background: transparent;
    color: inherit;
    font: inherit;
    padding: 0;
    min-height: 28px;
    gap: 8px;
    flex: 1;
    min-width: 0;
    cursor: pointer;
}

.import-account-activator:disabled {
    cursor: default;
}

.import-account-activator:focus-visible {
    outline: 2px solid rgb(var(--v-theme-primary));
    outline-offset: 2px;
}

.import-account-columns {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
}

.import-account-list {
    max-height: 310px;
    overflow-y: auto;
}
</style>
