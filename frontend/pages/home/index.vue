<script setup lang="ts">
  import { useI18n } from "vue-i18n";
  import { useDialog } from "@/components/ui/dialog-provider";
  import { DialogID } from "~/components/ui/dialog-provider/utils";
  import { Button } from "@/components/ui/button";
  import { itemsTable } from "./table";
  import { useTagStore } from "~/stores/tags";
  import { useLocationStore } from "~~/stores/locations";
  import BaseContainer from "@/components/Base/Container.vue";
  import BaseCard from "@/components/Base/Card.vue";
  import Subtitle from "~/components/global/Subtitle.vue";
  import ItemCard from "~/components/Item/Card.vue";
  import LocationCard from "~/components/Location/Card.vue";
  import TagChip from "~/components/Tag/Chip.vue";
  import Table from "~/components/Item/View/Table.vue";

  const { t } = useI18n();

  definePageMeta({
    middleware: ["auth"],
  });
  useHead({
    title: "HomeBox | " + t("menu.home"),
  });

  const api = useUserApi();
  const breakpoints = useBreakpoints();

  const locationStore = useLocationStore();
  const locations = computed(() =>
    [...locationStore.parentLocations].sort((a, b) => a.name.localeCompare(b.name, "de", { numeric: true }))
  );

  const tagsStore = useTagStore();
  const tags = computed(() => tagsStore.tags);

  const itemTable = itemsTable(api);
  const search = ref("");
  const { openDialog } = useDialog();
  function create(baseType: "item" | "location") {
    openDialog(DialogID.CreateEntity, { params: { baseType } });
  }
</script>

<template>
  <div>
    <BaseContainer class="flex flex-col gap-4">
      <form class="flex min-w-0 gap-2" @submit.prevent="navigateTo(`/items?q=${encodeURIComponent(search)}`)">
        <input
          v-model="search"
          :aria-label="$t('menu.search')"
          :placeholder="$t('menu.search')"
          class="min-h-11 min-w-0 flex-1 rounded border bg-background p-2"
        />
        <Button type="submit">{{ $t("menu.search") }}</Button>
      </form>
      <div class="flex flex-wrap gap-2">
        <Button @click="create('item')">{{ $t("components.location.create_item") }}</Button
        ><Button variant="outline" @click="create('location')">{{ $t("locations.create_location") }}</Button>
      </div>
      <section>
        <Subtitle> {{ $t("home.storage_locations") }} </Subtitle>
        <p v-if="locations.length === 0" class="ml-2 text-sm">
          {{ $t("locations.no_results") }}
        </p>
        <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3">
          <LocationCard v-for="location in locations" :key="location.id" :location="location" />
        </div>
      </section>

      <section>
        <Subtitle> {{ $t("custom.recent") }} </Subtitle>

        <p v-if="itemTable.items.length === 0" class="ml-2 text-sm">
          {{ $t("items.no_results") }}
        </p>
        <BaseCard v-else-if="breakpoints.lg">
          <Table :items="itemTable.items" />
        </BaseCard>
        <div v-else class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <ItemCard v-for="item in itemTable.items" :key="item.id" :item="item" />
        </div>
      </section>

      <section>
        <Subtitle> {{ $t("home.tags") }} </Subtitle>
        <p v-if="tags.length === 0" class="ml-2 text-sm">
          {{ $t("tags.no_results") }}
        </p>
        <div v-else class="flex flex-wrap gap-4">
          <TagChip v-for="tag in tags" :key="tag.id" size="lg" :tag="tag" class="shadow-md" />
        </div>
      </section>
    </BaseContainer>
  </div>
</template>
