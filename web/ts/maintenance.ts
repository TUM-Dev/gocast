interface maintenancePage {
    generateThumbnails(): Promise<boolean>
}

export function maintenancePage(): maintenancePage {
    return {
        generateThumbnails() {
            return fetch("/api/maintenance/generateThumbnails", {method: "POST"}).then(r => {
                return true
            });
        }
    }
}
