<script setup lang="ts">
import { reactive, ref, watch } from "vue";

import { StreamIssueCategory } from "@/gen/server/apiv2_pb";
import { ApiError } from "@/lib/api";
import { ISSUE_CATEGORIES, reportIssue } from "@/lib/watch-actions";
import ActionDialog from "./ActionDialog.vue";

/**
 * "Report a technical problem": categories, a required description and optional contact
 * details. Mount it only when `viewer.can_report_issue` (the action bar does). A 429
 * would mean the same report was throttled; it is shown as "wait" instead of an error.
 */
const props = defineProps<{ open: boolean; streamId: number; defaultName?: string }>();
const emit = defineEmits<{ close: []; sent: [] }>();

const form = reactive({
  categories: [] as StreamIssueCategory[],
  comment: "",
  name: "",
  email: "",
  phone: "",
});
const busy = ref(false);
const error = ref("");
const done = ref(false);

watch(
  () => props.open,
  (open) => {
    if (!open) return;
    Object.assign(form, { categories: [], comment: "", name: props.defaultName ?? "", email: "", phone: "" });
    error.value = "";
    done.value = false;
  },
);

async function submit(): Promise<void> {
  if (busy.value || !form.comment.trim()) return;
  busy.value = true;
  error.value = "";
  try {
    const result = await reportIssue(props.streamId, { ...form });
    if (result === "cooldown") {
      error.value = "Please wait a moment before sending another report.";
      return;
    }
    done.value = true;
    emit("sent");
  } catch (err) {
    error.value =
      err instanceof ApiError && err.status === 503
        ? "Problem reports are not available right now."
        : "The report could not be sent. Please try again.";
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <ActionDialog :open="open" title="Report a technical problem" @close="emit('close')">
    <p v-if="done" role="status" class="text-3 py-2 text-sm">
      Thank you, the report was sent to the operators.
    </p>
    <form v-else class="grid gap-3" @submit.prevent="submit">
      <fieldset>
        <legend class="text-3 text-sm font-bold">Relevant categories</legend>
        <div class="mt-2 grid grid-cols-2 gap-2 text-sm sm:grid-cols-3">
          <label v-for="c in ISSUE_CATEGORIES" :key="c.value" class="text-4 flex items-center gap-2">
            <input v-model="form.categories" type="checkbox" class="h-4 w-4" :value="c.value" />
            {{ c.label }}
          </label>
        </div>
      </fieldset>
      <div>
        <label class="text-5 block text-sm" for="issue-comment">Description of the issue</label>
        <textarea
          id="issue-comment"
          v-model="form.comment"
          required
          rows="4"
          placeholder="Add a short description of the problem here."
          class="tl-input mt-1 w-full resize-none"
        ></textarea>
      </div>
      <p class="text-3 text-sm font-bold">Your contact information (optional)</p>
      <div>
        <label class="text-5 block text-sm" for="issue-name">Name</label>
        <input id="issue-name" v-model="form.name" type="text" autocomplete="name" class="tl-input mt-1 w-full" />
      </div>
      <div>
        <label class="text-5 block text-sm" for="issue-email">Mail</label>
        <input id="issue-email" v-model="form.email" type="email" autocomplete="email" class="tl-input mt-1 w-full" />
      </div>
      <div>
        <label class="text-5 block text-sm" for="issue-phone">Phone</label>
        <input id="issue-phone" v-model="form.phone" type="tel" autocomplete="tel" class="tl-input mt-1 w-full" />
      </div>
      <p v-if="error" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>
      <div class="flex justify-end gap-2">
        <button type="button" class="rounded px-4 py-2 text-sm hover:bg-gray-100 dark:hover:bg-gray-600" @click="emit('close')">
          Cancel
        </button>
        <button
          type="submit"
          :disabled="busy || !form.comment.trim()"
          class="rounded bg-blue-500 px-4 py-2 text-sm text-white hover:bg-blue-600 disabled:opacity-50 dark:bg-indigo-600"
        >
          Send
        </button>
      </div>
    </form>
  </ActionDialog>
</template>
