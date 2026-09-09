<template>
  <AppLayout>
    <div class="mx-auto max-w-2xl space-y-6">
      <div v-if="loading" class="flex justify-center py-16">
        <Icon name="refresh" size="lg" class="animate-spin text-primary-500" />
      </div>

      <template v-else-if="status">
        <section class="overflow-hidden rounded-2xl border border-primary-200 bg-primary-50 dark:border-primary-900/50 dark:bg-primary-950/30">
          <div class="grid gap-6 p-6 sm:grid-cols-[1fr_auto] sm:items-center">
            <div>
              <p class="flex items-center gap-2 text-sm font-medium text-primary-700 dark:text-primary-300">
                <Icon name="calendar" size="sm" />
                {{ status.claimed_today ? t('dailyCheckIn.statusClaimed') : t('dailyCheckIn.statusAvailable') }}
              </p>
              <h2 class="mt-3 text-2xl font-semibold text-gray-900 dark:text-white">{{ t('dailyCheckIn.title') }}</h2>
              <p class="mt-2 text-sm text-gray-600 dark:text-dark-300">{{ t('dailyCheckIn.description') }}</p>
            </div>
            <div class="text-left sm:text-right">
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('dailyCheckIn.rewardLabel') }}</p>
              <p class="mt-1 text-3xl font-semibold text-emerald-600 dark:text-emerald-400">${{ status.reward.toFixed(2) }}</p>
            </div>
          </div>
        </section>

        <section class="card p-6">
          <div class="flex flex-col gap-5 sm:flex-row sm:items-center sm:justify-between">
            <div class="space-y-1">
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('dailyCheckIn.balanceLabel') }}</p>
              <p class="text-2xl font-semibold text-gray-900 dark:text-white">${{ status.balance.toFixed(2) }}</p>
              <p class="pt-2 text-sm text-gray-500 dark:text-dark-400">
                {{ t('dailyCheckIn.serverDate') }}: {{ status.checkin_date }}
              </p>
            </div>
            <button
              class="btn min-w-36"
              :class="status.claimed_today ? 'btn-secondary' : 'btn-primary'"
              :disabled="status.claimed_today || claiming"
              @click="claim"
            >
              <Icon v-if="claiming" name="refresh" size="sm" class="animate-spin" />
              <Icon v-else :name="status.claimed_today ? 'checkCircle' : 'calendar'" size="sm" />
              <span>{{ claiming ? t('dailyCheckIn.claiming') : status.claimed_today ? t('dailyCheckIn.statusClaimed') : t('dailyCheckIn.claim') }}</span>
            </button>
          </div>
          <p v-if="status.claimed_today" class="mt-5 border-t border-gray-100 pt-4 text-sm text-emerald-700 dark:border-dark-700 dark:text-emerald-400">
            {{ t('dailyCheckIn.alreadyClaimed') }}
          </p>
        </section>

        <section class="card p-6">
          <div class="flex items-start justify-between gap-4">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                {{ t('dailyCheckIn.calendarTitle') }}
              </h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ calendarMonthLabel }}</p>
            </div>
            <Icon name="calendar" size="sm" class="text-primary-500" />
          </div>

          <div class="mt-5 grid grid-cols-7 gap-1 text-center text-[11px] font-medium text-gray-400 dark:text-dark-400 sm:gap-2">
            <span v-for="weekday in weekdayLabels" :key="weekday" class="py-1">{{ weekday }}</span>
          </div>

          <div class="mt-1 grid grid-cols-7 gap-1 sm:gap-2">
            <span
              v-for="(day, index) in calendarDays"
              :key="day ? day.date : `empty-${index}`"
              class="relative flex min-h-10 aspect-square items-center justify-center rounded-lg border text-sm transition-colors sm:min-h-12"
              :class="day ? calendarDayClass(day) : 'border-transparent'"
            >
              <template v-if="day">
                <span>{{ day.day }}</span>
                <Icon v-if="day.isCheckedIn" name="check" size="xs" class="absolute bottom-1 right-1" :stroke-width="2" />
              </template>
            </span>
          </div>

          <div class="mt-5 flex flex-wrap items-center gap-x-5 gap-y-2 text-xs text-gray-500 dark:text-dark-400">
            <span class="inline-flex items-center gap-2">
              <span class="h-2.5 w-2.5 rounded-full bg-emerald-500" aria-hidden="true"></span>
              {{ t('dailyCheckIn.calendarCheckedIn') }}
            </span>
            <span class="inline-flex items-center gap-2">
              <span class="h-2.5 w-2.5 rounded-full border border-primary-500" aria-hidden="true"></span>
              {{ t('dailyCheckIn.calendarToday') }}
            </span>
          </div>
        </section>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import userAPI, { type DailyCheckInCalendar, type DailyCheckInStatus } from '@/api/user'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

const { t, locale } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const loading = ref(true)
const claiming = ref(false)
const status = ref<DailyCheckInStatus | null>(null)
const calendar = ref<DailyCheckInCalendar | null>(null)

interface CalendarDay {
  date: string
  day: number
  isToday: boolean
  isCheckedIn: boolean
  isFuture: boolean
}

function parseDateParts(value?: string) {
  const parts = value?.split('-').map(Number)
  if (!parts || parts.length < 3 || parts.some((part) => !Number.isInteger(part))) {
    return null
  }
  return { year: parts[0], month: parts[1], day: parts[2] }
}

const calendarDate = computed(() => parseDateParts(calendar.value?.checkin_date || status.value?.checkin_date))

const checkedInDates = computed(() => {
  const dates = new Set(calendar.value?.checkin_dates || [])
  if (status.value?.claimed_today && status.value.checkin_date) {
    dates.add(status.value.checkin_date)
  }
  return dates
})

const calendarMonthLabel = computed(() => {
  const date = calendarDate.value
  if (!date) return ''
  return new Intl.DateTimeFormat(locale.value, { year: 'numeric', month: 'long' }).format(
    new Date(date.year, date.month - 1, 1)
  )
})

const weekdayLabels = computed(() => {
  const formatter = new Intl.DateTimeFormat(locale.value, { weekday: 'short' })
  return Array.from({ length: 7 }, (_, index) => formatter.format(new Date(2024, 0, index + 1)))
})

const calendarDays = computed<Array<CalendarDay | null>>(() => {
  const date = calendarDate.value
  const today = calendar.value?.checkin_date || status.value?.checkin_date
  if (!date || !today) return []

  const firstDay = new Date(date.year, date.month - 1, 1)
  const leadingEmptyDays = (firstDay.getDay() + 6) % 7
  const daysInMonth = new Date(date.year, date.month, 0).getDate()
  const days: Array<CalendarDay | null> = Array.from({ length: leadingEmptyDays }, () => null)

  for (let day = 1; day <= daysInMonth; day += 1) {
    const dayDate = `${date.year}-${String(date.month).padStart(2, '0')}-${String(day).padStart(2, '0')}`
    days.push({
      date: dayDate,
      day,
      isToday: dayDate === today,
      isCheckedIn: checkedInDates.value.has(dayDate),
      isFuture: dayDate > today,
    })
  }

  return days
})

function calendarDayClass(day: CalendarDay) {
  return [
    day.isCheckedIn
      ? 'border-emerald-500 bg-emerald-500 font-semibold text-white shadow-sm'
      : day.isFuture
        ? 'border-transparent text-gray-300 dark:text-dark-600'
        : 'border-gray-100 bg-gray-50 text-gray-700 dark:border-dark-700 dark:bg-dark-800/60 dark:text-dark-200',
    day.isToday ? 'ring-2 ring-primary-400 ring-offset-1 ring-offset-white dark:ring-offset-dark-900' : '',
  ]
}

async function loadStatus() {
  loading.value = true
  try {
    status.value = await userAPI.getDailyCheckIn()
    void loadCalendar()
  } catch {
    appStore.showError(t('dailyCheckIn.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function loadCalendar() {
  try {
    calendar.value = await userAPI.getDailyCheckInCalendar()
  } catch {
    // The existing status card remains usable if the optional history request fails.
    calendar.value = null
  }
}

async function claim() {
  if (claiming.value || status.value?.claimed_today) return
  claiming.value = true
  try {
    const result = await userAPI.claimDailyCheckIn()
    status.value = result
    void loadCalendar()
    void authStore.refreshUser().catch(() => undefined)
    appStore.showSuccess(t(result.newly_claimed ? 'dailyCheckIn.claimSuccess' : 'dailyCheckIn.alreadyClaimed'))
  } catch {
    appStore.showError(t('dailyCheckIn.claimFailed'))
  } finally {
    claiming.value = false
  }
}

onMounted(loadStatus)
</script>
