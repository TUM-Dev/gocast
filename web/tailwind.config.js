module.exports = {
    purge: [
        './assets/css/*.css',
        './template/**/*.gohtml',
        './template/**/*.html',
        './ts/**/*.ts',
    ],
    darkMode: 'class', // or 'media' or 'false'
    theme: {
        extend: {
            colors: {
                'primary': '#090c10',
                'secondary': '#161b22',
                'secondary-lighter': '#0d1117',
                'success': '#2fe395',
                'info': '#2f56e3',
                'danger': '#e3342f',
                'warn': '#e3bc2f',
            },
            container: {
                center: true,
                padding: {
                    DEFAULT: '1rem',
                    sm: '2rem',
                    lg: '6rem',
                    xl: '8rem',
                    '2xl': '10rem',
                },
            }
        }
    },
    variants: {
        extend: {
            backgroundColor: ['odd'],
            display:['group-hover'],
            textColor: ['visited'],
        },
    },
    plugins: [],
}
