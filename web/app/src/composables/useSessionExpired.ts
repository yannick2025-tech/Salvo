import { ref } from 'vue'

/** Global reactive flag — set when any API returns body code 401. */
export const sessionExpired = ref(false)
