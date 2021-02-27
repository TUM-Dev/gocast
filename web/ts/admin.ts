function createUser() {
    let userName: string = (document.getElementById("name") as HTMLInputElement).value
    let email: string = (document.getElementById("email") as HTMLInputElement).value
    postData("api/createUser", {"name": userName, "email": email, "password": null})
        .then(data => {
            console.log(data)
        }).catch(error => {
        console.log(error)
    })

}