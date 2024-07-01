import React from "react";
import clsx from "clsx";
import styles from "./styles.module.css";

const FeatureList = [
  {
    title: "Easy to Use",
    Svg: require("@site/static/icons/play.svg").default,
    description: (
      <>
        Designed to be easy to use, so that stutdents can focus on the content
        and not the technology when watching lectures.
      </>
    ),
  },
  {
    title: "Built by students for students.",
    Svg: require("@site/static/icons/film-camera.svg").default,
    description: (
      <>
        GoCast is built by students for students and handles thousands of hours
        of video every semester for more than 150 courses and 15.000 Students.
      </>
    ),
  },
  {
    title: "Privacy Friendly",
    Svg: require("@site/static/icons/server.svg").default,
    description: (
      <>
        Deliver live events and recordings like it's the 21st century - Privacy
        friendly, self-hosted and open-source.
      </>
    ),
  },
];

const ForLecturers = [
  {
    title: "1. Create a GoCast Account",
    description: (
      <>
        Contact your organization's GoCast maintainer to receive access to the
        GoCast platform.
      </>
    ),
  },
  {
    title: "2. Create a Course",
    description: (
      <>Automatically import a course from TUMOnline or create it manually.</>
    ),
  },
  {
    title: "3. Start Streaming",
    description: (
      <>
        Start streaming your lectures and events. GoCast will automatically take
        care of the rest.
      </>
    ),
  },
];

const ForOrganizations = [
  {
    title: "1. Apply for a GoCast Account",
    description: (
      <>Contact the RBG to receive maintainer-access to the GoCast platform.</>
    ),
  },
  {
    title: "2. Connect your Resources to the GoCast Network",
    description: (
      <>
        Integrate GoCast with your existing infrastructure and connect to our
        network. All streaming data is processed by and stored on your servers.
      </>
    ),
  },
  {
    title: "3. Start Streaming",
    description: (
      <>
        Start inviting lecturers to stream their lectures and events. GoCast
        will automatically take care of the rest.
      </>
    ),
  },
];

function Card({ Svg, title, description }) {
  return (
    <div className="col">
      <div className="padding-horiz--md">
        <h3>{title}</h3>
        <p>{description}</p>
      </div>
    </div>
  );
}

function Features({ Svg, title, description }) {
  return (
    <div className="col">
      <div className="text--center">
        <Svg className={styles.featureSvg} role="img" />
      </div>
      <div className="padding-horiz--md">
        <h3>{title}</h3>
        <p>{description}</p>
      </div>
    </div>
  );
}

export default function HomepageFeatures() {
  return (
    <>
      <section className={styles.features}>
        <div className="container">
          <h2 className="text--center">Features</h2>
          <div className="row">
            {FeatureList.map((props, idx) => (
              <Features key={idx} {...props} />
            ))}
          </div>
        </div>
      </section>
      <section>
        <div className={clsx(styles.getStarted, "container")}>
          <div className="row">
            <div className="col">
              <h2 className="text--center">For Lecturers</h2>
              {ForLecturers.map((props, idx) => (
                <Card key={idx} {...props} />
              ))}
            </div>
            <div className="col">
              <h2 className="text--center">For Organizations</h2>
              {ForOrganizations.map((props, idx) => (
                <Card key={idx} {...props} />
              ))}
            </div>
          </div>
        </div>
      </section>
    </>
  );
}
