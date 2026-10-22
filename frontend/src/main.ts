import "@fortawesome/fontawesome-free/css/all.min.css";
import { createPinia } from "pinia";
import { createApp } from "vue";

import App from "./App.vue";
import { registerServiceWorker, watchConnectivity } from "./lib/pwa";
import { router } from "./router";
import "./styles/main.css";

createApp(App).use(createPinia()).use(router).mount("#app");

watchConnectivity();
// Not awaited: the app works without a worker, it only loses offline support.
registerServiceWorker().catch(() => {});
