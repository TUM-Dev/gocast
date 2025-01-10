// @ts-check
// `@type` JSDoc annotations allow editor autocompletion and type checking
// (when paired with `@ts-check`).
// There are various equivalent ways to declare your Docusaurus config.
// See: https://docusaurus.io/docs/api/docusaurus-config

import { themes as prismThemes } from "prism-react-renderer";

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: "GoCast",
  tagline:
    "Livestreaming und VoD Service of the Technical University of Munich",
  url: "https://tum.live",
  baseUrl: "/",
  onBrokenLinks: "throw",
  onBrokenMarkdownLinks: "warn",
  favicon: "img/favicon.ico",

  // GitHub pages deployment config.
  // If you aren't using GitHub pages, you don't need these.
  organizationName: "TUM-Dev", // Usually your GitHub org/user name.
  projectName: "gocast-docs", // Usually your repo name.

  // Even if you don't use internalization, you can use this field to set useful
  // metadata like html lang. For example, if your site is Chinese, you may want
  // to replace "en" with "zh-Hans".
  i18n: {
    defaultLocale: "en",
    locales: ["en"],
  },

  presets: [
    [
      "classic",
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          sidebarPath: require.resolve("./sidebars.js"),
          editUrl: "https://github.com/tum-dev/gocast/edit/main",
          lastVersion: "current",
          versions: {
            current: {
              label: "Production",
              path: "/",
            },
            beta: {
              label: "Beta 🚧",
              path: "beta",
              banner: "unreleased",
            },
          },
        },
        blog: {
          showReadingTime: true,
          // Please change this to your repo.
          // Remove this to remove the "edit this page" links.
        },
        theme: {
          customCss: require.resolve("./src/css/custom.css"),
        },
      }),
    ],
  ],
  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      announcementBar: {
        id: "support_us",
        content:
          '<a target="_blank" rel="noopener noreferrer" href="https://github.com/TUM-Dev/gocast/issues/new/choose/">Help us improve! Did you know that GoCast is open source? If you have any features in mind please request them on GitHub</a>',
        backgroundColor: "#0063ba",
        textColor: "white",
        isCloseable: true,
      },
      navbar: {
        logo: {
          alt: "GoCast Logo",
          src: "icons/tum-live-logo.svg",
          srcDark: "icons/tum-live-logo.svg",
        },
        items: [
          {
            type: "doc",
            docId: "intro",
            position: "left",
            label: "Documentation",
          },
          //{ to: "/blog", label: "Changelogs", position: "left" },
          {
            href: "https://live.rbg.tum.de",
            label: "GoCast",
            position: "left",
          },
          {
            href: "https://meldeplattform.tum.de/",
            label: "Security",
            position: "left",
          },
          {
            href: "https://tum-live.betteruptime.com/",
            label: "System Status",
            position: "left",
          },
          {
            type: "docsVersionDropdown",
            position: "right",
            // dropdownItemsAfter: [{ to: "/versions", label: "All versions" }],
            dropdownActiveClassDisabled: true,
          },
          {
            href: "https://github.com/TUM-Dev/gocast",
            label: "GitHub",
            icon: "Github",
            position: "right",
          },
        ],
      },
      footer: {
        style: "dark",
        logo: {
          alt: "TumDev Logo",
          src: "/icons/tum-live-logo.svg",
          href: "https://github.com/TUM-Dev",
        },
        links: [
          {
            title: "Stream & Record",
            items: [
              {
                label: "Quickstart",
                to: "/docs/intro",
              },
              {
                label: "Tutorials",
                to: "/docs/usage/user-guide#create-a-course",
              },
              {
                label: "Guides",
                to: "/docs/usage/user-guide#create-a-course",
              },
              {
                label: "Troubleshooting",
                to: "/docs/usage/user-guide#create-a-course",
              },
            ],
          },
          {},
          {
            title: "Most Viewed Docs",
            items: [
              //{
              //  label: "Set up GoCast for your school",
              //  to: "/docs/beta/category/deployment",
              //},
              {
                label: "Start streaming lectures",
                to: "/docs/features/LectureHallStreams",
              },
              {
                label: "Import courses",
                to: "/docs/usage/user-guide#create-a-course",
              },
              {
                label: "Live Chat",
                to: "/docs/usage/chat",
              },
            ],
          },
          {},
          {
            title: "More",
            items: [
              {
                label: "About",
                href: "https://app.tum.de",
              },
              {
                label: "Privacy",
                href: "https://live.rbg.tum.de/privacy",
              },
              {
                label: "Imprint",
                href: "https://live.rbg.tum.de/imprint",
              },
              {
                label: "Changelogs",
                href: "/blog",
              },
            ],
          },
        ],
        copyright: `Copyright © ${new Date().getFullYear()} • Technische Universität München.`,
      },
      prism: {
        theme: prismThemes.github,
        darkTheme: prismThemes.dracula,
        additionalLanguages: ["ruby", "bash", "python", "java", "json", "php"],
      },
      colorMode: {
        defaultMode: "dark",
        disableSwitch: false,
        respectPrefersColorScheme: false,
      },
      algolia: {
        // The application ID provided by Algolia
        appId: "FLS9C8AV4W",
        // Public API key: it is safe to commit it
        apiKey: "639109f1526c3dbcf3d87dbdd35666f1",
        indexName: "tumlive-pages",
      },
    }),
  scripts: [
    {
      src: "https://fonts.googleapis.com/css2?family=roboto&display=swap",
      async: true,
    },
  ],
};

module.exports = config;
