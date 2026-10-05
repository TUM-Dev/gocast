<script setup lang="ts">
import { computed } from "vue";

import CreateLectureSeries from "@/components/admin/lectures/create/CreateLectureSeries.vue";
import {
  MAX_LECTURES,
  durationMinutes,
  formatMinutes,
  type CreateLectureForm,
} from "@/lib/create-lecture";
import { SELF_STREAMED, type ScheduleLectureHall } from "@/lib/schedule";

/**
 * The wizard's second step: title, times, hall, chat and, for a scheduled
 * livestream, the series. Times are native datetime-local inputs, as in the lecture
 * editor; v1's flatpickr is not carried over.
 */
const props = defineProps<{
  form: CreateLectureForm;
  halls: ScheduleLectureHall[];
  /**
   * Only server administrators are offered a hall, as in the lecture editor; everyone
   * else creates self-streamed lectures and asks the RBG for a hall.
   */
  canChooseHall: boolean;
  /** v1 offered the chat only in a course that has it on. */
  courseChatEnabled: boolean;
}>();

// Recomputed with the inputs; "now" only matters to an ad-hoc end, to the minute.
const duration = computed(() => formatMinutes(durationMinutes(props.form, new Date()) ?? 0));

const field = (name: string) => `create-lecture-${name}`;
</script>

<template>
  <div class="flex flex-col gap-4 text-sm">
    <div class="flex flex-col gap-1">
      <label :for="field('title')" class="text-2">Title <span class="text-red-500" aria-hidden="true">*</span></label>
      <input
        :id="field('title')"
        v-model="form.title"
        type="text"
        autocomplete="off"
        required
        placeholder="L01: Binary Trees"
        class="tum-live-input w-full"
      />
    </div>

    <div v-if="form.mode === 'adhoc'" class="flex flex-col gap-1 sm:max-w-xs">
      <label :for="field('end-time')" class="text-2">
        Ends at <span class="text-red-500" aria-hidden="true">*</span>
        <span v-if="duration" class="text-5 ml-1 font-normal">({{ duration }})</span>
      </label>
      <input :id="field('end-time')" v-model="form.endTime" type="time" required class="tum-live-input w-full" />
      <p class="text-5 text-xs">The livestream starts in two minutes. A time that has passed today means tomorrow.</p>
    </div>

    <div v-else class="grid gap-3 sm:grid-cols-2">
      <div class="flex min-w-0 flex-col gap-1">
        <label :for="field('start')" class="text-2">
          {{ form.mode === "vod" ? "Date" : "Start" }} <span class="text-red-500" aria-hidden="true">*</span>
        </label>
        <input :id="field('start')" v-model="form.start" type="datetime-local" required class="tum-live-input w-full" />
      </div>
      <div v-if="form.mode === 'scheduled'" class="flex min-w-0 flex-col gap-1">
        <label :for="field('end')" class="text-2">
          End <span class="text-red-500" aria-hidden="true">*</span>
          <span v-if="duration" class="text-5 ml-1 font-normal">({{ duration }})</span>
        </label>
        <input :id="field('end')" v-model="form.end" type="datetime-local" required class="tum-live-input w-full" />
      </div>
    </div>

    <div v-if="form.mode !== 'vod' && canChooseHall" class="flex flex-col gap-1">
      <label :for="field('hall')" class="text-2">Lecture hall</label>
      <select :id="field('hall')" v-model.number="form.lectureHallId" class="tum-live-input">
        <option :value="SELF_STREAMED">None (self-streamed)</option>
        <option v-for="hall in halls" :key="hall.id" :value="hall.id">{{ hall.name }}</option>
      </select>
    </div>
    <p v-else-if="form.mode !== 'vod'" class="text-5 text-xs">
      Self-streamed: the stream key is on the lecture's card once it is created. To stream from a lecture hall,
      please contact the RBG.
    </p>

    <label v-if="courseChatEnabled" class="text-3 flex items-center gap-2">
      <input v-model="form.chatEnabled" type="checkbox" />
      Chat enabled
    </label>

    <template v-if="form.mode === 'scheduled'">
      <label class="text-3 flex items-center gap-2">
        <input v-model="form.recurring" type="checkbox" />
        Recurring: create a series of lectures
      </label>
      <div v-if="form.recurring" class="flex flex-col gap-3 sm:border-l-2 sm:pl-3 dark:border-gray-700">
        <div class="grid gap-3 sm:grid-cols-2">
          <div class="flex min-w-0 flex-col gap-1">
            <label :for="field('interval')" class="text-2">Repeat</label>
            <select :id="field('interval')" v-model="form.interval" class="tum-live-input w-full">
              <option value="daily">Daily</option>
              <option value="weekly">Weekly</option>
              <option value="monthly">Monthly</option>
            </select>
          </div>
          <div class="flex min-w-0 flex-col gap-1">
            <label :for="field('count')" class="text-2">Number of lectures</label>
            <input
              :id="field('count')"
              v-model.number="form.count"
              type="number"
              min="2"
              :max="MAX_LECTURES"
              class="tum-live-input w-full"
            />
          </div>
        </div>
        <CreateLectureSeries :form="form" />
      </div>
    </template>
  </div>
</template>
