export default {
  plugins: {
    // v4's PostCSS integration lives in its own package. autoprefixer is gone with
    // it: Tailwind runs the output through Lightning CSS, which prefixes for us.
    "@tailwindcss/postcss": {},
  },
};
