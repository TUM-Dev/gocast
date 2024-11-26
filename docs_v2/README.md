# GoCast Docs

Contains the new docs for GoCast.

A prototype can be found [here](https://tumlive-docs.pages.dev/).

## Getting started

To start the development server, run:

```bash
npm run dev
```

To build the project site **for production**, run:

```bash
npm run build
```

> The static files are generated in the `build` folder.

To start the production server, run:

```bash
npm run start
```

## How to add a new page

To add a new page, create a new markdown file in the `docs` directory. The file should have the following structure:

```markdown
---
title: Page Title
---

# Page Title

Page content goes here.
```

The `title` field in the front matter is used to generate the page title in the sidebar.

## How to add a new section

To add a new section, create a new directory in the `docs` directory. Inside the directory, create a markdown file for each page in the section. The directory should have an `index.md` file with the following structure:

```markdown
---
title: Section Title
---

# Section Title

Section description goes here.
```

The `title` field in the front matter is used to generate the section title in the sidebar.

## How to add a new sidebar item

To add a new sidebar item, edit the `sidebar.json` file in the `data` directory. The file should have the following structure:

```json
[
  {
    "title": "Section Title",
    "children": [
      {
        "title": "Page Title",
        "slug": "page-slug"
      }
    ]
  }
]
```

The `title` field is used to generate the section title in the sidebar. The `children` field is an array of sidebar items. Each sidebar item should have a `title` field and a `slug` field. The `title` field is used to generate the page title in the sidebar. The `slug` field is used to generate the page URL.

## More information

For more information, see the official Docusaurus docs [here](https://docusaurus.io/docs).



## Credit

https://docusaurus.io \
https://github.com/facebook/docusaurus
