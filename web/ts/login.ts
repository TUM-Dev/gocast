class Login {
    private loginWithTUM: boolean = true;
    private usernameInput: HTMLInputElement;
    private passwordInput: HTMLInputElement;
    private studentLoginBtn: HTMLDivElement
    private adminLoginBtn: HTMLDivElement

    constructor() {
        this.usernameInput = document.getElementById("username") as HTMLInputElement
        this.passwordInput = document.getElementById("password") as HTMLInputElement
        this.studentLoginBtn = document.getElementById("studentLoginBtn") as HTMLDivElement
        this.studentLoginBtn.addEventListener("click", (e: Event) => this.onStudentClick());
        this.adminLoginBtn = document.getElementById("adminLoginBtn") as HTMLDivElement
        this.adminLoginBtn.addEventListener("click", (e: Event) => this.onAdminClick());

        (document.getElementById("loginForm") as HTMLFormElement).addEventListener("submit", (e: Event) => this.submitForm(e))
    }

    private onStudentClick(): void {
        this.loginWithTUM = true;
        this.adminLoginBtn.classList.add("text-gray-500")
        this.adminLoginBtn.classList.remove("text-white")
        this.studentLoginBtn.classList.remove("text-gray-500")
        this.studentLoginBtn.classList.add("text-white")
        this.usernameInput.placeholder = "ga12abc"
        this.usernameInput.type = "text"
    }

    private onAdminClick(): void {
        this.loginWithTUM = false;
        this.adminLoginBtn.classList.remove("text-gray-500")
        this.adminLoginBtn.classList.add("text-white")
        this.studentLoginBtn.classList.remove("text-white")
        this.studentLoginBtn.classList.add("text-gray-500")
        this.usernameInput.placeholder = "erika.mustermann@tum.de"
        this.usernameInput.type = "email"
    }

    private submitForm(e: Event): boolean {
        e.preventDefault()
        let success: boolean = true
        if (this.usernameInput.value === "") {
            success = false
            this.usernameInput.classList.add("border-2", "border-warn")
        }
        if (this.passwordInput.value === "") {
            success = false
            this.passwordInput.classList.add("border-2", "border-warn")
        }
        if (success) {
            postData("/api/login", {
                "loginWithTUM": this.loginWithTUM,
                "username": this.usernameInput.value,
                "password": this.passwordInput.value,
            }).then(res => {
                if (res.status !== 200) {
                    res.text().then(text => showMessage(text))
                } else {
                    window.location.href = "/"
                }
            })
        }
        return false
    }
}

new Login();
