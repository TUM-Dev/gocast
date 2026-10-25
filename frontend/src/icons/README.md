# Icons

Font Awesome Free 7.3.1 SVGs (CC BY 4.0; attribution is in the header of `src/styles/fa-icons.css`), one folder per style.
`icons.json` maps the class name templates use (`fa-sync`) to the SVG file (`arrows-rotate.svg`).
`pnpm icons` turns both into `src/styles/fa-icons.css`; `icons.test.ts` fails if a class is used but not defined, or if the CSS is stale.

To add an icon, copy its SVG from the Font Awesome package into the right folder, add it to `icons.json`, run `pnpm icons`.
