<template>
  <div
    class="glass-auth relative flex min-h-screen items-center justify-center overflow-x-hidden p-4 sm:p-6"
    @mousemove="updateTilt"
    @mouseleave="resetTilt"
  >
    <div class="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden="true">
      <span class="auth-orb orb-blue"></span>
      <span class="auth-orb orb-pink"></span>
      <span class="auth-orb orb-green"></span>
      <span class="auth-orb orb-amber"></span>
    </div>

    <div ref="stageRef" class="glass-auth-stage relative z-10 w-full max-w-md">
      <div class="mb-7 text-center">
        <template v-if="settingsLoaded">
          <div class="glass-avatar mx-auto mb-5 flex h-16 w-16 items-center justify-center overflow-hidden rounded-full">
            <img :src="siteLogo || '/logo.svg'" alt="" class="h-9 w-9 object-contain" />
          </div>
          <h1 class="text-[26px] font-medium leading-tight text-white">{{ siteName }}</h1>
          <p class="mt-1.5 text-sm font-light text-white/60">{{ siteSubtitle }}</p>
        </template>
      </div>

      <div class="glass-auth-card">
        <div ref="materialRef" class="glass-material" aria-hidden="true"></div>
        <div class="relative z-10 p-6 sm:p-8">
          <slot />
        </div>
      </div>

      <div class="mt-6 text-center text-sm text-white/70">
        <slot name="footer" />
      </div>

      <p class="mt-8 text-center text-xs font-light text-white/35">
        &copy; {{ currentYear }} {{ siteName }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'

const appStore = useAppStore()
const stageRef = ref<HTMLElement | null>(null)
const materialRef = ref<HTMLElement | null>(null)
let tiltAnimationFrame = 0

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() =>
  sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true })
)
const siteSubtitle = computed(
  () => appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform'
)
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const currentYear = computed(() => new Date().getFullYear())
const authPageClass = 'auth-glass-page'

function updateTilt(event: MouseEvent): void {
  const stage = stageRef.value
  if (!stage || !event.currentTarget) {
    return
  }

  const root = event.currentTarget as HTMLElement
  const rect = stage.getBoundingClientRect()
  const relativeX = (event.clientX - rect.left) / Math.max(rect.width, 1) - 0.5
  const relativeY = (event.clientY - rect.top) / Math.max(rect.height, 1) - 0.5

  materialRef.value?.style.setProperty('--mouse-x', `${event.clientX - rect.left}px`)
  materialRef.value?.style.setProperty('--mouse-y', `${event.clientY - rect.top}px`)

  if (root.matches(':focus-within') || window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    return
  }

  cancelAnimationFrame(tiltAnimationFrame)
  tiltAnimationFrame = requestAnimationFrame(() => {
    root.style.setProperty('--tilt-x', `${relativeY * -7}deg`)
    root.style.setProperty('--tilt-y', `${relativeX * 7}deg`)
  })
}

function resetTilt(event: MouseEvent): void {
  cancelAnimationFrame(tiltAnimationFrame)
  const root = event?.currentTarget as HTMLElement | null
  root?.style.setProperty('--tilt-x', '0deg')
  root?.style.setProperty('--tilt-y', '0deg')
}

onMounted(() => {
  document.documentElement.classList.add(authPageClass)
  document.body.classList.add(authPageClass)
  appStore.fetchPublicSettings()
})

onBeforeUnmount(() => {
  cancelAnimationFrame(tiltAnimationFrame)
  document.documentElement.classList.remove(authPageClass)
  document.body.classList.remove(authPageClass)
})
</script>

<style scoped>
.glass-auth {
  min-height: 100vh;
  min-height: 100dvh;
  background: #000;
  color: #fff;
}

.auth-orb {
  position: absolute;
  display: block;
  border-radius: 9999px;
  mix-blend-mode: screen;
  filter: blur(90px);
  animation: auth-orb-move linear infinite;
}

.orb-blue {
  width: clamp(240px, 42vw, 420px);
  height: clamp(240px, 42vw, 420px);
  top: 8%;
  left: 8%;
  background: rgba(0, 122, 255, 0.58);
  animation-duration: 25s;
}

.orb-pink {
  width: clamp(300px, 52vw, 520px);
  height: clamp(300px, 52vw, 520px);
  right: 10%;
  bottom: 4%;
  background: rgba(255, 45, 85, 0.52);
  animation-direction: reverse;
  animation-duration: 30s;
}

.orb-green {
  width: clamp(220px, 36vw, 360px);
  height: clamp(220px, 36vw, 360px);
  bottom: 26%;
  left: 24%;
  background: rgba(52, 199, 89, 0.48);
  animation-duration: 20s;
}

.orb-amber {
  width: clamp(190px, 30vw, 300px);
  height: clamp(190px, 30vw, 300px);
  top: 5%;
  right: 20%;
  background: rgba(255, 149, 0, 0.44);
  animation-direction: reverse;
  animation-duration: 27s;
}

.glass-auth-stage {
  perspective: 1200px;
  animation: glass-fade-up 0.75s cubic-bezier(0.23, 1, 0.32, 1) both;
}

.glass-auth-card {
  position: relative;
  transform-style: preserve-3d;
  border-radius: 28px;
  transition: transform 0.55s cubic-bezier(0.23, 1, 0.32, 1);
}

.glass-material {
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: rgba(255, 255, 255, 0.095);
  border: 1px solid rgba(255, 255, 255, 0.15);
  backdrop-filter: blur(38px) saturate(175%);
  -webkit-backdrop-filter: blur(38px) saturate(175%);
  box-shadow: 0 24px 64px rgba(0, 0, 0, 0.34);
}

.glass-material::before {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: radial-gradient(
    circle at var(--mouse-x, 50%) var(--mouse-y, 50%),
    rgba(255, 255, 255, 0.2),
    transparent 40%
  );
  opacity: 0;
  transition: opacity 0.3s ease-out;
  pointer-events: none;
}

.glass-auth-card:hover .glass-material::before {
  opacity: 1;
}

.glass-avatar {
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.13);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.12);
}

@media (hover: hover) and (pointer: fine) {
  .glass-auth:hover .glass-auth-card,
  .glass-auth:focus-within .glass-auth-card {
    transform: rotateX(var(--tilt-x, 0deg)) rotateY(var(--tilt-y, 0deg));
  }
}

@keyframes auth-orb-move {
  0% {
    transform: translate3d(0, 0, 0) scale(1);
  }
  25% {
    transform: translate3d(72px, -38px, 0) scale(1.08);
  }
  50% {
    transform: translate3d(-28px, 48px, 0) scale(0.96);
  }
  75% {
    transform: translate3d(42px, -62px, 0) scale(1.04);
  }
  100% {
    transform: translate3d(0, 0, 0) scale(1);
  }
}

@keyframes glass-fade-up {
  from {
    opacity: 0;
    transform: translateY(26px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
