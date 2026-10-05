<script setup lang="ts">
  import type { EntitySummary, EntityOut } from "~/lib/api/types/data-contracts";
  import { Card } from "@/components/ui/card";
  import MdiMapMarkerOutline from "~icons/mdi/map-marker-outline";
  const props = defineProps<{
    location: EntitySummary | EntityOut;
    dense?: boolean;
  }>();
  const api = useUserApi();
  const flatLocations = useFlatLocations();
  const path = computed(() => flatLocations.value.find(l => l.id === props.location.id)?.treeString);
  const { data: detail } = useAsyncData(
    () => `location-tile-${props.location.id}`,
    async () => (await api.items.getLocation(props.location.id)).data
  );
  const photo = computed(
    () =>
      detail.value?.attachments?.find(a => a.type === "photo" && a.primary) ||
      detail.value?.attachments?.find(a => a.type === "photo")
  );
  const image = computed(() =>
    photo.value
      ? api.authURL(`/entities/${props.location.id}/attachments/${photo.value.thumbnail?.id || photo.value.id}`)
      : undefined
  );
</script>
<template>
  <Card class="overflow-hidden">
    <NuxtLink :to="`/location/${location.id}`" class="block">
      <img v-if="image && !dense" :src="image" :alt="location.name" loading="lazy" class="h-36 w-full object-cover" />
      <div v-else-if="!dense" class="flex h-36 items-center justify-center bg-muted">
        <MdiMapMarkerOutline class="size-12 text-muted-foreground" />
      </div>
      <h2 class="break-words p-3 text-lg font-medium">{{ location.name }}</h2>
      <p v-if="path && path !== location.name" class="break-words px-3 pb-3 text-xs text-muted-foreground">
        {{ path }}
      </p>
    </NuxtLink>
  </Card>
</template>
