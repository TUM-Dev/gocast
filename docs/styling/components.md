# Components

## Buttons

### Primary

<img alt = "Light primary button" width="365" src="./img/light_primary_button.jpg">
<img alt = "Dark primary button" width="365" src="./img/dark_primary_button.jpg">

```html

<button type=""
        class="block bg-blue-500 text-center px-3 py-1 mt-3 rounded w-full dark:bg-indigo-600">
    <span class="text-white uppercase text-sm font-semibold">Login</span>
</button>
```

### Secondary

<img alt = "Light secondary button" width="128" src="./img/light_secondary_buttons.jpg">
<img alt = "Dark secondary button" width="128" src="./img/dark_secondary_buttons.jpg">

```html
<!-- change background on :hover -->
<button @click=""
        title=""
        class="rounded-lg px-3 py-2 w-fit bg-gray-100 hover:bg-gray-200 dark:bg-secondary-light dark:hover:bg-gray-600">
    <i class="fa-solid fa-comments text-4"></i>
</button>

<!-- change text-color on :hover -->
<button @click=""
        title=""
        class="rounded-lg px-3 py-2 w-fit bg-gray-100 dark:bg-secondary-light hover:text-1">
    <i class="fa-solid fa-comments text-4"></i>
</button>
```

## Inputs

<img alt = "Light input" width="365" src="./img/light_input.jpg">
<img alt = "Dark input" width="365" src="./img/dark_input.jpg">

```html

<div class="text-sm">
    <label for="" class="block text-5">Password</label>
    <input type="" name="" id="" placeholder="" autofocus="" required
           class="rounded px-4 py-3 mt-3 focus:outline-none border-0 bg-gray-50 w-full dark:bg-gray-600"/>
</div>
```

## Forms

<img alt = "Light form" width="365" src="./img/light_form.jpg">
<img alt = "Dark form" width="365" src="./img/dark_form.jpg">

```html 
<div class="mx-auto w-4/5 my-10 shadow rounded-lg border bg-white dark:shadow-0 dark:border-gray-800 dark:bg-secondary-light">
    <div class="border-b py-2 px-5 dark:border-gray-800">
        <h6 class="text-3 font-bold">Title</h6>
    </div>
    <form class="grid gap-3 px-5 py-4">
          ...
    </form>
</div>
```

