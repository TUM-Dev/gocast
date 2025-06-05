---
title: "Troubleshooting"
sidebar_position: 9
description: "Troubleshooting."
---

:::note
For more technical problems, check out the [GitHub issues](https://github.com/TUM-Dev/gocast/issues).
:::

## Troubleshooting

<details>
<summary>I can't access the admin dashboard</summary>

Make sure you're logged in and are authorized (you must have at least the 'maintainer' role). If you're an admin or maintainer, you should see the "Admin" tab in the navigation bar. Click on it and you should be able to access the admin dashboard. If not, contact your school's IT team or the RBG.

</details>

<details>
<summary>I can't see my organization</summary>

If you're an admin or maintainer, you should see your organization(s) in the "organizations"-tab of the admin dashboard. If not, either create a new organization or contact your organization's IT team or the RBG.

</details>

<details>
<summary>I can't add resources to my organization</summary>

Make sure you have a valid token for your organization. If not, fetch a new one by clicking on the key-icon of the relevant organization. The token expires after 7 hours.

</details>

<details>
<summary>I can't add a worker</summary>

Make sure your worker is running and has the correct token. If not, check the worker's logs for errors. If you can't resolve the issue, contact your organization's IT team or the RBG.

</details>

<details>
<summary>I can't add a runner</summary>

Make sure your runner is running and has the correct token. If not, check the runner's logs for errors. If you can't resolve the issue, contact your organization's IT team or the RBG.

</details>

<details>
<summary>I can't add a VoD Service</summary>

Check the logs and make sure to have set all required environment variables.

</details>

<details>
<summary>My Edge server isn't reachable</summary>

Check the logs and make sure to have set all required environment variables.

</details>
