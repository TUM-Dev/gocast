import effects from "chart.js/dist/helpers/helpers.easing";

let db: IDBDatabase;
let openOrCreateDB : IDBOpenDBRequest;

export function initIndexDB() {
    openOrCreateDB = window.indexedDB.open("offline-videos", 1);
    openOrCreateDB.addEventListener("error", () => console.error("Error opening IndexedDB database"));
    openOrCreateDB.addEventListener("success", () => {
        console.log("Successfully opened IndexedDB database");
        db = openOrCreateDB.result;
    });
    openOrCreateDB.addEventListener("upgradeneeded", init => {
        db = (init.target as any).result;

        db.onerror = () => {
            console.error("Error loading IndexedDB database");
        }

        db.createObjectStore("videos", { keyPath: "id"});
    });
}

export function storeVideoOffline(title: string, url: string, date: Date, duration: number, course: string, video: File) {
    const newVideo = {key: 10, title: title, url: url, date: date, duration: duration, course: course, video: video};
    const transaction = db.transaction(["videos"], "readwrite");
    const objectStore = transaction.objectStore("videos");
    const query = objectStore.add(newVideo, 10);
    query.addEventListener("success", () => {
        console.log("Video stored offline");
    });
    transaction.addEventListener("complete", () => {
        console.log("Transaction completed");
    });
    transaction.addEventListener("error", () => {
        console.error("Error storing video offline");
    });

}