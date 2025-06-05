---
title: "Organizations & Maintainers 🆕"
sidebar_position: 3
description: "Basic terminology."
---

# Maintainers and Organizations

_The idea is the following_: To avoid one entity having to manage and process all streaming data for the entire university (or multiple universities), GoCast is distributed to multiple entities. Each entity (aka GoCast 'organization') has so-called maintainers (users with the `maintainer` user role) that are allowed to manage the organization's resources such as Workers/Runners, VoD Services, etc.

Maintainers also have some basic administrative functionality which is limited to their organizations' scope (e.g., create, update and delete courses and streams only for those organizations which are administered by that maintainer). For an overview of your administered organizations, go to the ["organizations"-tab](https://live.rbg.tum.de/admins/organizations) in the admin dashboard.

:::info
One maintainer can maintain multiple organizations.
:::

### The following organization-related actions are allowed by a maintainer of a organization:

- Create, update or delete organization

- Create new tokens for that organization (required to add new resources)

- Manage organization's resources

- Manage organization's maintainers

### TUMOnline organization vs. GoCast organization

TUMOnline has a strict hierarchical structure for its organizations (one organization has multiple departments; one department has multiple chairs; one chair has multiple courses ...).

:::note
On a side node, TUMOnline has 7 schools, 29 departments and 487 chairs.
:::

While GoCast is mainly used by the TUM, in principle it doesn't need to differentiate between organizational types that strictly. Organizations are only relevant when it comes to distributing the livestreams and recordings of a certain entity to that entity's resources (e.g., Workers/Runners and VoD Services).
Hence, the introduction of GoCast's "organizations" which **represent an entity responsible for processing data**. In practice, this is most of the time a TUMOnline organization, however, in theory one could also create a GoCast "organization" for a department, chair or smaller organization, depending on the specific situation.

#### Here's an example to illustrate this in a more detailled way:

> _The TUMOnline "organization of Management" (SOM) wants to start using GoCast. Hence, the SOM's IT team contacts the admins of GoCast who then create a new GoCast "organization" of type `TUM organization` and assign the SOM IT team as maintainers._
>
> _The subordinated "Chair of Financial Management and Capital Markets" (FA), however, has its own data center and wants to host its lectures with its own resources. In this case, either one of the SOM maintainers or the RBG can create a new GoCast "organization" of type `Lehrstuhl` and accordingly assign new maintainers from the FA-team. Now, the FA-team can connect their own resources from their data center with GoCast, independently of the SOM._
