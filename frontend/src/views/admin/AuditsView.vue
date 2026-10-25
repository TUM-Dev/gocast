<script setup lang="ts">
import { onMounted, ref } from "vue";

import AdminLayout from "@/components/admin/AdminLayout.vue";
import AuditLog from "@/components/admin/AuditLog.vue";
import { redirectToLogin, useAuthStore } from "@/stores/auth";

/**
 * The server-wide audit log. Read-only, so there is nothing here beyond paging
 * through it -- see AuditLog.vue, which course settings also uses for one course.
 */
const auth = useAuthStore();
const ready = ref(false);

onMounted(async () => {
  // Only reachable anonymously by a client-side navigation; a full load is refused.
  const user = await auth.load().catch(() => null);
  if (!user) {
    redirectToLogin();
    return;
  }

  ready.value = true;
});
</script>

<template>
  <AdminLayout>
    <section class="mx-auto flex max-w-5xl flex-col gap-6">
      <div class="flex items-center justify-between">
        <h1 class="text-1 text-2xl font-bold">Audits</h1>
      </div>

      <div class="rounded-lg border p-4 dark:border-gray-800">
        <AuditLog v-if="ready" />
      </div>
    </section>
  </AdminLayout>
</template>
