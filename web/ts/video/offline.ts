import {getPlayers} from "../TUMLiveVjs";

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

        db.createObjectStore("videos");
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

export function testStoreOffline() {
    let xhr = new XMLHttpRequest(), blob;

    xhr.open("GET", "http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4", true);
    xhr.responseType = "blob";

    xhr.addEventListener("load", () => {
        if (xhr.status === 200) {
            console.log("Video loaded");
            blob = xhr.response as Blob;
            console.log("Blob loaded" + blob);

            let transaction = db.transaction(["videos"], "readwrite");
            transaction.objectStore("videos").put(blob, "video");
            transaction.objectStore("videos").get("video").onsuccess = async (event) => {
                let vdFile = (event.target as any).result;
                console.log("Got video from db" + (vdFile as Blob));

                let URL = window.URL || window.webkitURL;
                let videoFileUrl = URL.createObjectURL(vdFile);

                document.getElementById("offlineVideo").setAttribute("src", videoFileUrl);

            }
        }
    });

    xhr.send();

}