export function pageData(): Home {
    return {
        showUserContext: false,
        toggleUserContext() {
            this.showUserContext = !this.showUserContext;
        },
    };
}

interface Home {
    showUserContext: boolean;
    toggleUserContext();
}
