<template>
    <v-menu v-model="open" :close-on-content-click="false" :disabled="disabled" max-width="520">
        <template #activator="{ props: activatorProps }">
            <div class="import-category-selection d-flex align-center">
                <slot name="leading" />
                <button type="button" class="import-category-activator d-flex align-center text-start"
                        v-bind="activatorProps" :disabled="disabled" :aria-label="tt('Category')" @click.stop>
                    <slot />
                    <v-icon class="ms-auto" size="18" :icon="open ? mdiMenuUp : mdiMenuDown" v-if="!hideDropdown" />
                </button>
            </div>
        </template>
        <div @click.stop @keydown.esc.stop.prevent="open = false">
            <v-card width="480">
                <v-text-field ref="searchInput" class="ma-2" density="compact" hide-details
                              :prepend-inner-icon="mdiMagnify" :placeholder="tt('Find category')" :aria-label="tt('Find category')"
                              v-model="search" @keydown="onSearchKeydown" />
                <div class="import-category-columns" v-if="filteredOptions.length">
                    <v-list class="import-category-list" :aria-label="tt('Category')">
                        <v-list-item v-for="parent in filteredParents" :key="parent.id"
                                     :active="parent.id === activeParentId" :title="parent.name"
                                     @click="browseParent(parent.id)">
                            <template #prepend>
                                <ItemIcon class="me-2" :icon-type="getCategoryIconType(parent.iconType)"
                                          :icon-id="parent.icon" :color="parent.color" />
                            </template>
                        </v-list-item>
                    </v-list>
                    <v-list ref="resultList" class="import-category-list" role="listbox" :aria-label="tt('Category')">
                        <v-list-item v-for="option in activeOptions" :key="option.id"
                                     :active="option.id === highlightedOption?.id" :title="option.category.name"
                                     role="option" :aria-selected="option.id === highlightedOption?.id"
                                     @click="select(option.id)">
                            <template #prepend>
                                <ItemIcon class="me-2" :icon-type="getCategoryIconType(option.category.iconType)"
                                          :icon-id="option.category.icon" :color="option.category.color" />
                            </template>
                        </v-list-item>
                    </v-list>
                </div>
                <div class="text-body-large mx-4 mt-3 mb-2" v-else>{{ tt('No available category') }}</div>
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
import { getCategoryIconType } from '@/lib/icon.ts';
import { NormalizedText } from '@/core/text.ts';
import type { TransactionCategory } from '@/models/transaction_category.ts';

const props = defineProps<{
    categories: TransactionCategory[];
    disabled?: boolean;
    hideDropdown?: boolean;
}>();
const emit = defineEmits<{
    (e: 'select', id: string): void;
    (e: 'update:open', open: boolean): void;
}>();
const { tt } = useI18n();
const open = ref(false);
const search = ref('');
const highlightedIndex = ref(0);
const searchInput = useTemplateRef<VTextField>('searchInput');
const resultList = useTemplateRef<VList>('resultList');
const filteredOptions = computed(() => {
    const query = NormalizedText.normalizeForSearch(search.value);
    return props.categories.filter(parent => !parent.hidden).flatMap(parent =>
        (parent.subCategories || []).filter(category => !category.hidden).map(category => ({
            id: category.id,
            parentId: parent.id,
            category,
            title: `${parent.name} → ${category.name}`
        }))
    ).filter(option => NormalizedText.normalizeForSearch(option.title).includes(query));
});

const highlightedOption = computed(() => filteredOptions.value[highlightedIndex.value]);
const activeParentId = computed(() => highlightedOption.value?.parentId);
const filteredParents = computed(() => props.categories.filter(parent =>
    filteredOptions.value.some(option => option.parentId === parent.id)));
const activeOptions = computed(() => filteredOptions.value.filter(option => option.parentId === activeParentId.value));

function browseParent(parentId: string): void {
    const index = filteredOptions.value.findIndex(option => option.parentId === parentId);
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
            highlightedIndex.value = (highlightedIndex.value + (event.key === 'ArrowDown' ? 1 : count - 1)) % count;
        }
    }
}

watch(filteredOptions, () => { highlightedIndex.value = 0; });
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
        highlightedIndex.value = 0;
        await nextTick();
        // Lazy menu content may mount after the first tick.
        setTimeout(() => { if (open.value) searchInput.value?.focus(); }, 50);
    }
});
</script>

<style scoped>
.import-category-selection {
    width: 234px;
}

.import-category-activator {
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

.import-category-activator:disabled {
    cursor: default;
}

.import-category-activator:focus-visible {
    outline: 2px solid rgb(var(--v-theme-primary));
    outline-offset: 2px;
}

.import-category-columns {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
}

.import-category-list {
    max-height: 310px;
    overflow-y: auto;
}
</style>
