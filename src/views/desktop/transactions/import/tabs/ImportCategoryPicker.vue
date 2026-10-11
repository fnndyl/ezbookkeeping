<template>
    <v-menu v-model="open" :close-on-content-click="false" :disabled="disabled" max-width="400">
        <template #activator="{ props: activatorProps }">
            <button type="button" class="d-flex align-center text-start" style="color: inherit"
                    v-bind="activatorProps" :disabled="disabled" :aria-label="tt('Category')" @click.stop>
                <slot />
            </button>
        </template>
        <v-card width="360" @click.stop @keydown.esc.stop.prevent="open = false">
            <v-text-field ref="searchInput" class="ma-2" density="compact" hide-details
                          :placeholder="tt('Find category')" :aria-label="tt('Find category')"
                          v-model="search" @keydown="onSearchKeydown" />
            <v-list ref="resultList" style="max-height: 310px; overflow-y: auto" role="listbox" :aria-label="tt('Category')">
                <v-list-item v-for="(option, index) in filteredOptions" :key="option.id"
                             :active="index === highlightedIndex" :title="option.title"
                             role="option" :aria-selected="index === highlightedIndex"
                             @click="select(option.id)" />
                <v-list-item v-if="!filteredOptions.length" :title="tt('No available category')" />
            </v-list>
        </v-card>
    </v-menu>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, useTemplateRef, watch } from 'vue';
import { VTextField } from 'vuetify/components/VTextField';
import { VList } from 'vuetify/components/VList';
import { useI18n } from '@/locales/helpers.ts';
import { NormalizedText } from '@/core/text.ts';
import type { TransactionCategory } from '@/models/transaction_category.ts';

const props = defineProps<{
    categories: TransactionCategory[];
    disabled?: boolean;
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
            title: `${parent.name} → ${category.name}`
        }))
    ).filter(option => NormalizedText.normalizeForSearch(option.title).includes(query));
});

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
