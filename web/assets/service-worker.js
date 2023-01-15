/*global console self caches fetch setTimeout clearTimeout*/
/*
   Network first cache strategy
 */
const CACHE_NAME = "v1";
const FALLBACK_TO_CACHE_TIMEOUT = 30000;
const PREFETCH_CACHE_FILES = [
    "/",
    "/login",
    "/about",
    "/static/node_modules/@fortawesome/fontawesome-free/css/all.min.css",
    "/static/assets/css-dist/main.css",
    "/static/assets/css/icons.css",
    "/static/assets/ts-dist/global.bundle.js",
    "/static/assets/ts-dist/admin.bundle.js",
    "/static/node_modules/@alpinejs/persist/dist/cdn.min.js",
    "/static/node_modules/alpinejs/dist/cdn.js",
];
const CACHE_REQUEST_METHOD_ALLOWLIST = ["GET"];
const CACHE_REQUEST_HOST_ALLOWLIST = [self.location.host];
const DONT_CACHE_URLS = ["/api/editor/waveform"];

const shouldCacheReq = (req) => {
    // eslint-disable-next-line no-undef
    const urlHost = new URL(req.url).host;
    return CACHE_REQUEST_METHOD_ALLOWLIST.includes(req.method) && CACHE_REQUEST_HOST_ALLOWLIST.includes(urlHost);
};

self.addEventListener("install", function (e) {
    //console.log("[ServiceWorker] Installed");
    e.waitUntil(
        caches.open(CACHE_NAME).then(function (cache) {
            //console.log("[ServiceWorker] Caching cacheFiles");
            return cache.addAll(PREFETCH_CACHE_FILES);
        }),
    );
});

self.addEventListener("activate", function (e) {
    //console.log("[ServiceWorker] Activated");
    e.waitUntil(
        caches.keys().then((cacheNames) => {
            return Promise.all(
                cacheNames.map((cacheName) => {
                    if (cacheName !== CACHE_NAME) {
                        return caches.delete(cacheName);
                    }
                }),
            );
        }),
    );
});

self.addEventListener("fetch", (e) => {
    if (!shouldCacheReq(e.request)) {
        //console.debug("Cache exception");
        return;
    }

    const fromNetwork = (request, timeout) =>
        new Promise((fulfill, reject) => {
            let curTimeout = timeout;
            // set timout to infinity for DON'T CACHE URLS
            if (DONT_CACHE_URLS.includes(request.url)) {
                curTimeout = Infinity;
            }
            let timeoutId = setTimeout(reject, curTimeout);

            fetch(request).then((response) => {
                clearTimeout(timeoutId);
                fulfill(response.clone());
                caches.open(CACHE_NAME).then((cache) => cache.put(request, response));
            }, reject);
        });

    const fromCache = (request) =>
        caches.open(CACHE_NAME).then((cache) => cache.match(request).then((matching) => matching));

    e.respondWith(fromNetwork(e.request, FALLBACK_TO_CACHE_TIMEOUT).catch(() => fromCache(e.request)));
});
