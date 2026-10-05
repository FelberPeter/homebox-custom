<script setup lang="ts">
  import { DialogRoot as Dialog } from "reka-ui";
  import { useI18n } from "vue-i18n";
  import { DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogDescription } from "@/components/ui/dialog";
  import { Button } from "@/components/ui/button";
  import type { LocationOperationResult } from "~/lib/api/classes/items";
  import { useFlatLocations } from "~/composables/use-location-helpers";
  const props = defineProps<{ id: string; name: string }>();
  const emit = defineEmits<{ done: [] }>();
  const { t } = useI18n();
  const api = useUserApi();
  const locations = useFlatLocations();
  const open = ref(false);
  const busy = ref(false);
  const error = ref("");
  const preview = ref<LocationOperationResult>();
  const allowConflicts = ref(false);
  const options = reactive({
    action: "generate",
    requestId: "",
    name: "",
    parentId: "00000000-0000-0000-0000-000000000000",
    depth: -1,
    items: false,
    photos: true,
    attachments: false,
    serialNumbers: false,
    purchaseWarranty: false,
    mode: "number",
    count: 12,
    start: 1,
    digits: 0,
    rows: 3,
    columns: 4,
    rowStart: "A",
    pattern: "Fach *",
    description: "",
  });
  function newRequestId() {
    const bytes = crypto.getRandomValues(new Uint8Array(16));
    bytes[6] = (bytes[6]! & 15) | 64;
    bytes[8] = (bytes[8]! & 63) | 128;
    const hex = Array.from(bytes, b => b.toString(16).padStart(2, "0")).join("");
    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
  }
  let submitted = false;
  function begin(action: string) {
    Object.assign(options, {
      action,
      requestId: newRequestId(),
      name: props.name,
      parentId: "00000000-0000-0000-0000-000000000000",
    });
    submitted = false;
    preview.value = undefined;
    error.value = "";
    allowConflicts.value = false;
    open.value = true;
    useLocationStore().refreshTree();
  }
  watch(
    () => JSON.stringify(options),
    () => {
      preview.value = undefined;
      allowConflicts.value = false;
      if (submitted) {
        options.requestId = newRequestId();
        submitted = false;
      }
    }
  );
  watch(
    () => options.mode,
    mode => {
      options.pattern = mode === "grid" ? "Fach {row}{col}" : "Fach *";
    }
  );
  async function run(isPreview: boolean) {
    if (busy.value) return;
    busy.value = true;
    error.value = "";
    if (!isPreview) submitted = true;
    try {
      const result = await api.items.locationOperation(props.id, {
        ...options,
        preview: isPreview,
        allowConflicts: allowConflicts.value,
      });
      if (result.error) {
        error.value = t("custom.operation_failed");
        return;
      }
      if (isPreview) preview.value = result.data;
      else {
        open.value = false;
        await Promise.all([
          useLocationStore().refreshChildren(),
          useLocationStore().refreshParents(),
          useLocationStore().refreshTree(),
        ]);
        emit("done");
        if (options.action !== "generate") await navigateTo(`/location/${result.data.rootId}`);
      }
    } catch {
      error.value = t("custom.operation_failed");
    } finally {
      busy.value = false;
    }
  }
</script>
<template>
  <div class="my-4 flex flex-wrap gap-2">
    <Button v-for="action in ['generate', 'copy', 'move']" :key="action" variant="outline" @click="begin(action)">{{
      t(`custom.${action}`)
    }}</Button>
  </div>
  <Dialog v-model:open="open">
    <DialogContent
      :disable-close="busy"
      class="max-h-[90dvh] w-[calc(100%_-_1rem)] max-w-lg overflow-y-auto overflow-x-hidden"
      @escape-key-down="busy && $event.preventDefault()"
      @interact-outside="busy && $event.preventDefault()"
    >
      <DialogHeader
        ><DialogTitle>{{ t(`custom.${options.action}`) }}</DialogTitle>
        <DialogDescription>{{t("custom.limit")}}</DialogDescription></DialogHeader
      >
      <fieldset :disabled="busy" class="grid min-w-0 gap-3">
        <p class="text-sm">{{ t("custom.limit") }}</p>
        <template v-if="options.action === 'generate'">
          <label
            >{{ t("custom.mode")
            }}<select v-model="options.mode" class="block min-h-11 w-full min-w-0 rounded-md border bg-background p-2 text-foreground">
              <option value="number">{{ t("custom.number") }}</option>
              <option value="grid">{{ t("custom.grid") }}</option>
            </select></label
          >
          <div v-if="options.mode === 'number'" class="grid grid-cols-2 gap-3">
            <label
              >{{ t("custom.count")
              }}<input v-model.number="options.count" type="number" min="1" max="200" class="block min-h-11 w-full min-w-0 rounded-md border bg-background p-2 text-foreground"
            /></label>
            <label
              >{{ t("custom.digits")
              }}<input v-model.number="options.digits" type="number" min="0" max="8" class="block min-h-11 w-full min-w-0 rounded-md border bg-background p-2 text-foreground"
            /></label>
          </div>
          <div v-else class="grid grid-cols-2 gap-3">
            <label
              >{{ t("custom.rows")
              }}<input v-model.number="options.rows" type="number" min="1" max="26" class="block min-h-11 w-full min-w-0 rounded-md border bg-background p-2 text-foreground"
            /></label>
            <label
              >{{ t("custom.columns")
              }}<input v-model.number="options.columns" type="number" min="1" max="200" class="block min-h-11 w-full min-w-0 rounded-md border bg-background p-2 text-foreground"
            /></label>
            <label
              >{{ t("custom.row_start") }}<input v-model="options.rowStart" maxlength="1" class="block min-h-11 w-full min-w-0 rounded-md border bg-background p-2 text-foreground"
            /></label>
          </div>
          <label
            >{{ t("custom.start") }}<input v-model.number="options.start" type="number" min="0" class="block min-h-11 w-full min-w-0 rounded-md border bg-background p-2 text-foreground"
          /></label>
          <label>{{ t("custom.pattern") }}<input v-model="options.pattern" class="block min-h-11 w-full min-w-0 rounded-md border bg-background p-2 text-foreground" /></label>
          <label
            >{{ t("global.description")
            }}<textarea v-model="options.description" maxlength="1000" class="block min-h-11 w-full min-w-0 rounded-md border bg-background p-2 text-foreground" />
          </label>
        </template>
        <template v-else>
          <label>{{ t("global.name") }}<input v-model="options.name" maxlength="255" class="block min-h-11 w-full min-w-0 rounded-md border bg-background p-2 text-foreground" /></label>
          <label
            >{{ t("custom.destination")
            }}<select v-model="options.parentId" class="block min-h-11 w-full min-w-0 rounded-md border bg-background p-2 text-foreground">
              <option value="00000000-0000-0000-0000-000000000000">
                {{ t("custom.root") }}
              </option>
              <option v-for="loc in locations" :key="loc.id" :value="loc.id">
                {{ loc.treeString }}
              </option>
            </select></label
          >
          <template v-if="options.action === 'copy'">
            <label
              >{{ t("custom.depth")
              }}<select v-model.number="options.depth" class="block min-h-11 w-full min-w-0 rounded-md border bg-background p-2 text-foreground">
                <option :value="-1">{{ t("custom.all") }}</option>
                <option :value="0">0</option>
                <option :value="1">1</option>
                <option :value="2">2</option>
                <option :value="3">3</option>
              </select></label
            >
            <label
              v-for="key in ['items', 'photos', 'attachments', 'serialNumbers', 'purchaseWarranty'] as const"
              :key="key"
              class="flex min-h-11 items-center gap-3"
              ><input v-model="options[key]" type="checkbox" class="size-5" />{{ t(`custom.${key}`) }}</label
            >
            <p class="text-sm text-muted-foreground">
              {{ t("custom.fields_notice") }}
            </p>
          </template>
        </template>
        <Button variant="outline" :disabled="busy" @click="run(true)">{{ t("custom.preview") }}</Button>
        <p v-if="error" role="alert" class="text-destructive">{{ error }}</p>
        <div v-if="preview" class="min-w-0 space-y-2">
          <p>
            {{
              t("custom.summary", {
                locations: preview.locations,
                items: preview.items,
                excluded: preview.excluded,
              })
            }}
          </p>
          <ol class="max-h-48 overflow-y-auto rounded border p-2 text-sm">
            <li
              v-for="(node, i) in preview.nodes"
              :key="i"
              class="break-words"
              :style="{ paddingLeft: `${Math.min(node.depth, 6) * 12}px` }"
            >
              {{ node.name }}
            </li>
          </ol>
          <details v-if="preview.excludedNodes?.length">
            <summary>{{ t("custom.excluded") }}</summary>
            <ul class="max-h-32 overflow-y-auto text-sm">
              <li v-for="node in preview.excludedNodes" :key="node.id" class="break-words">
                {{ node.name }}
              </li>
            </ul>
          </details>
          <div v-if="preview.conflicts.length" class="rounded border border-orange-500 p-2">
            <p>{{ t("custom.conflicts") }}: {{ preview.conflicts.join(", ") }}</p>
            <label class="flex min-h-11 items-center gap-2"
              ><input v-model="allowConflicts" type="checkbox" class="size-5" />{{ t("custom.allow_conflicts") }}</label
            >
          </div>
        </div>
      </fieldset>
      <DialogFooter
        ><Button :disabled="busy || !preview || (!!preview.conflicts.length && !allowConflicts)" @click="run(false)">{{
          busy ? t("custom.busy") : t("custom.execute")
        }}</Button></DialogFooter
      >
    </DialogContent>
  </Dialog>
</template>
