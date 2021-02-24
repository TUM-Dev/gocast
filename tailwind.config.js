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
                'info': '#2f56e3',
                'danger': '#e3342f',
            },
            container: {
                center: true,
                padding: {
                    DEFAULT: '1rem',
                    sm: '2rem',
                    lg: '4rem',
                    xl: '5rem',
                    '2xl': '6rem',
                },
            }
        }
    },
    variants: {
        extend: {},
    },
    plugins: [],
}
