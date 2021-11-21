import { postData } from './global'

class OnboardingFormData {
    public name: string;
    public email: string;
    public password: string;

    constructor(name: string, email: string, password: string) {
        this.name = name;
        this.email = email;
        this.password = password;
    }
}

let nameBox: HTMLInputElement = document.querySelector('#name');
nameBox.onfocus = function () {
    document.querySelector("#nameError").innerHTML = "";
}
let emailBox: HTMLInputElement = document.querySelector('#email');
emailBox.onfocus = function () {
    document.querySelector("#emailError").innerHTML = "";
}
let passwordBox: HTMLInputElement = document.querySelector('#password');
passwordBox.onfocus = function () {
    document.querySelector("#passwordError").innerHTML = "";
}

const form: HTMLFormElement = document.querySelector('#onboardingForm');
form.onsubmit = (e: Event) => {
    e.preventDefault()
    const formData = new FormData(form);
    let success: boolean = true;
    const data = new OnboardingFormData(formData.get("name") as string, formData.get("email") as string, formData.get("password") as string);
    if (data.name.length < 5 || !data.name.match("^.* .*")) {
        document.querySelector("#nameError").innerHTML = "<p>Provided name is invalid.</p>";
        success = false;
    }
    if (!data.email.match("^[A-Za-z0-9\\.-]{3,64}[a-zA-Z0-9]@([A-Za-z0-9\\.-]*\\.)?tum\\.(de|edu)")) {
        document.querySelector("#emailError").innerHTML = "<p>Provided email is not a valid TUM email address.</p>";
        success = false;
    }
    if (data.password.length < 8) {
        document.querySelector("#passwordError").innerHTML = "<p>The password is insufficiently secure.</p>";
        success = false;
    }
    if (!success) {
        return false;
    }
    console.log(JSON.stringify(data));
    postData("api/createUser", data)
        .then(data => {
            location.reload();
        }).catch(error => {
            console.log(error)
        })
    return false; // prevent reload
}
