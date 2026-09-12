import js from "@eslint/js";
import prettierRecommended from "eslint-plugin-prettier/recommended";
import tseslint from "typescript-eslint";

// Flat config, required from ESLint 10 on. Mirrors the rule set of the .eslintrc it
// replaced; `eslint:recommended` and the two @typescript-eslint presets are the same
// lists, reached through the typescript-eslint meta package instead of the separate
// parser/plugin entries.
export default tseslint.config(
  {
    // Build output. `spa/assets` matters in particular: it holds the minified Vite
    // bundle, and running prettier/recommended over a 280 KB single-line file pegs a
    // core indefinitely.
    ignores: ["assets/ts-dist/**", "assets/css-dist/**", "spa/**", "node_modules/**"],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  prettierRecommended,
  {
    rules: {
      // `ban-ts-ignore` from the old config is gone: it was removed in
      // @typescript-eslint v5 and superseded by ban-ts-comment, which stays off.
      "@typescript-eslint/ban-ts-comment": "off",
      "@typescript-eslint/no-unused-vars": "off",
    },
  },
);
