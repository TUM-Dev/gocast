function createUser() {
    let userName: string = (document.getElementById("name") as HTMLInputElement).value
    let email: string = (document.getElementById("email") as HTMLInputElement).value
    postData("api/createUser", {"name": userName, "email": email, "password": null})
        .then(data => {
            showMessage("User was created successfully.Reload to see them.")
        }).catch(error => {
        showMessage("There was an error creating the user: " + error)
    })

}