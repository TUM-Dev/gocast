class LoginFormData {
    public email: string;
    public password: string;

    constructor(email: string, password: string) {
        this.email = email;
        this.password = password;
    }
}

const loginForm: HTMLFormElement = document.querySelector('#loginForm');
loginForm.onsubmit = (e: Event) => {
    e.preventDefault()
    const formData = new FormData(loginForm);
    const data = new LoginFormData(formData.get("email") as string, formData.get("password") as string);
    console.log(JSON.stringify(data));
    postData("api/login", data)
        .then(data => {
            location.reload();
        }).catch(error => {
            console.log(error)
        })
    return false; // prevent reload
}
