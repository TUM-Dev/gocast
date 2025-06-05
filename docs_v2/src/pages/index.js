import React from "react";
import clsx from "clsx";
import Link from "@docusaurus/Link";
import useDocusaurusContext from "@docusaurus/useDocusaurusContext";
import Layout from "@theme/Layout";
import HomepageFeatures from "@site/src/components/HomepageFeatures";

import styles from "./index.module.css";

function HomepageHeader() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <header className={clsx("hero hero--primary", styles.heroBanner)}>
      <div className="container">
        <h1 className="hero__title">GoCast Docs</h1>
        <p className="hero__subtitle">{siteConfig.tagline}</p>
        <div className={styles.buttons}>
          <Link
            className={clsx(
              "button button--lg background:white",
              styles.heroButton
            )}
            to="/docs/beta/deployment/overview"
          >
            Deploy GoCast at your organization 🎥
          </Link>
          <Link
            className={clsx("button button--lg", styles.heroButton)}
            to="/docs/intro"
          >
            Get started and start streaming ⏱️
          </Link>
        </div>
      </div>
    </header>
  );
}

const StatsList = [
  {
    stat: "300+",
    headline: "Courses",
  },
  {
    stat: "15.000+",
    headline: "Students",
  },
  {
    stat: "150+",
    headline: "Lecturers",
  },
  {
    stat: "25.000+",
    headline: "Hours of Video",
  },
];

function Stat({ stat, headline }) {
  return (
    <div className={clsx(styles.statBox)}>
      <h1>{stat}</h1>
      <h3>{headline}</h3>
    </div>
  );
}

export default function Home() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <Layout
      title={`${siteConfig.title}`}
      description="Description will go into a meta tag in <head />"
    >
      <section className={styles.header}>
        <HomepageHeader />
        <section className={styles.stats}>
          <div>
            <h2 className="text--center">
              Already used at TUM's largest schools
            </h2>
            <div className={styles.statsContainer}>
              {StatsList.map((props, idx) => (
                <Stat key={idx} {...props} />
              ))}
            </div>
          </div>
        </section>
      </section>
      <main>
        <HomepageFeatures />
      </main>
    </Layout>
  );
}
