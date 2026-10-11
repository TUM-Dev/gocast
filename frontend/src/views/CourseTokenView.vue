<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute } from "vue-router";

import {
  MISSING_TOKEN_MESSAGE,
  courseTokenErrorMessage,
  fetchCourseByToken,
  optInCourseByToken,
  optOutCourseByToken,
  tokenFromQuery,
  type CourseByToken,
} from "@/lib/course-token";

/**
 * The page a lecturer lands on from the mail sent when their course was imported:
 * `/edit-course?token=` to switch it on, `/edit-course/opt-out?token=` to have it
 * deleted. One view, told which by the route; the token in the link is the
 * credential, so neither needs a session.
 */
const props = defineProps<{ mode: "opt-in" | "opt-out" }>();
const route = useRoute();

const token = computed(() => tokenFromQuery(route.query));
const course = ref<CourseByToken | null>(null);
const loadError = ref("");
const working = ref(false);
const done = ref(false);
const error = ref("");

onMounted(async () => {
  if (!token.value) {
    loadError.value = MISSING_TOKEN_MESSAGE;
    return;
  }
  try {
    course.value = await fetchCourseByToken(token.value);
  } catch (err) {
    loadError.value = courseTokenErrorMessage(err);
  }
});

async function confirm(): Promise<void> {
  working.value = true;
  error.value = "";
  try {
    if (props.mode === "opt-in") await optInCourseByToken(token.value);
    else await optOutCourseByToken(token.value);
    done.value = true;
  } catch (err) {
    error.value = courseTokenErrorMessage(err);
  } finally {
    working.value = false;
  }
}
</script>

<template>
  <section class="text-3 mx-auto grid w-full max-w-2xl content-start gap-y-4 p-6">
    <header>
      <h1 class="text-1 text-2xl font-bold">
        {{ course ? course.name : mode === "opt-in" ? "Enable your course" : "Opt out of your course" }}
      </h1>
      <p v-if="course" class="text-5 text-sm">{{ course.term === "W" ? "Winter" : "Summer" }} {{ course.year }}</p>
    </header>

    <p v-if="loadError" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">{{ loadError }}</p>
    <p v-else-if="!course" class="text-5 text-sm">Looking up your course…</p>

    <template v-else-if="mode === 'opt-in'">
      <p v-if="done" class="text-success font-semibold" role="status">
        Your course is enabled. You can now head over to the administration page to edit it.
      </p>
      <p v-else-if="!course.optedOut">
        This course was already enabled. Head over to the administration page to edit it.
      </p>
      <template v-else>
        <h2 class="text-2 text-xl font-medium">Enable livestreaming and video on demand</h2>
        <p>
          After enabling streaming, your course appears in your administration tab on this website,
          provided you sign in with the account we sent the notification email to.
        </p>
        <p>You can then edit the course's properties there, or leave everything as it is.</p>
        <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">{{ error }}</p>
        <div class="flex justify-center pt-2">
          <button type="button" class="tum-live-button tum-live-button-primary px-4 py-2" :disabled="working" @click="confirm">
            Enable streaming 🎉
          </button>
        </div>
      </template>
    </template>

    <template v-else>
      <p v-if="done" class="font-semibold" role="status">Your course was deleted successfully.</p>
      <p v-else-if="course.optedOut">
        This course is already opted out. If that is not what you expected, please reach out to us.
      </p>
      <template v-else>
        <p>
          Please confirm deleting the course <span class="font-semibold">{{ course.name }}</span>. This disables
          livestreaming and recordings of your lectures. Opening the enable link again restores it.
        </p>
        <p v-if="error" class="rounded-lg bg-danger/25 px-2 py-2 text-sm" role="alert">{{ error }}</p>
        <div>
          <button type="button" class="rounded-md bg-red-600 px-5 py-2 text-sm font-semibold text-white hover:bg-red-700 disabled:opacity-50 dark:bg-red-700 dark:hover:bg-red-600" :disabled="working" @click="confirm">
            Confirm
          </button>
        </div>
      </template>
    </template>
  </section>
</template>
