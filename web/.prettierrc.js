module.exports = {
    semi: true,
    trailingComma: "all",
    printWidth: 120,
    tabWidth: 4,
    bracketSpacing: true,
    endOfLine: "auto",
    plugins: [require("prettier-plugin-tailwindcss"), require("prettier-plugin-go-template")],
};
