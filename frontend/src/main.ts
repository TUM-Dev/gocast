import "./styles/fa-icons.css";
import { createPinia } from "pinia";
import { createApp } from "vue";

import App from "./App.vue";
import { registerServiceWorker, watchConnectivity, watchInstalled } from "./lib/pwa";
import { router } from "./router";
import { useOfflineStore } from "./stores/offline";
import "./styles/main.css";

const pinia = createPinia();
createApp(App).use(pinia).use(router).mount("#app");

watchConnectivity();
watchInstalled();
// Not awaited: the app works without a worker, it only loses offline support.
registerServiceWorker().catch(() => {});

// Renew the licences of lectures saved on this device whenever there is a network to
// do it with. A failure leaves them as they are until the next time.
const offline = useOfflineStore(pinia);
const syncOffline = () => offline.sync().catch(() => {});
syncOffline();
window.addEventListener("online", syncOffline);
