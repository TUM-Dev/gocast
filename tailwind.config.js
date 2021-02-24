module.exports = {
    purge: [
        './web/**/*.html',
        './web/**/*.gohtml',
        './web/**/*.js'
    ],
    darkMode: 'class', // or 'media' or 'false'
    theme: {
        extend: {
            colors: {
                'primary': '#0d1117',
                'secondary': '#161b22',
                'secondary-lighter': '#161b22',
                'success': '#2fe395',
                'danger': '#e3342f',
            }
        }
    },
    variants: {
        extend: {},
    },
    plugins: [],
}
